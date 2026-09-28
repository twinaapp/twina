package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const progressEvent = "fileop:progress"

// Progress is emitted as the progressEvent while a copy or move runs.
type Progress struct {
	Job        string `json:"job"`
	Phase      string `json:"phase"` // "scan", "copy" or "done"
	Done       int64  `json:"done"`
	Total      int64  `json:"total"`
	Files      int    `json:"files"`
	TotalFiles int    `json:"totalFiles"`
	Current    string `json:"current"`
}

type tracker struct {
	ctx  context.Context
	emit func(Progress)
	p    Progress
	last time.Time
}

func (t *tracker) send(force bool) {
	if t.emit == nil || !force && time.Since(t.last) < 100*time.Millisecond {
		return
	}
	t.last = time.Now()
	t.emit(t.p)
}

// Conflicts returns the names of sources that already exist in destDir.
func (s *FileService) Conflicts(sources []string, destDir string) ([]string, error) {
	destDir, err := s.resolve(destDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, src := range sources {
		dst := filepath.Join(destDir, filepath.Base(src))
		if _, err := os.Lstat(dst); err == nil && dst != src {
			names = append(names, filepath.Base(src))
		}
	}
	return names, nil
}

// Copy recursively copies each source into destDir. Existing items are
// replaced (folders are merged) only when overwrite is set. Progress is
// reported under the given job id; cancelling the call stops the copy and
// removes partially copied items.
func (s *FileService) Copy(ctx context.Context, job string, sources []string, destDir string, overwrite bool) error {
	return s.transfer(ctx, job, sources, destDir, false, overwrite)
}

// Move moves each source into destDir, copying across devices.
func (s *FileService) Move(ctx context.Context, job string, sources []string, destDir string, overwrite bool) error {
	return s.transfer(ctx, job, sources, destDir, true, overwrite)
}

type transferItem struct {
	src, dst string
	existed  bool
}

func (s *FileService) transfer(ctx context.Context, job string, sources []string, destDir string, move, overwrite bool) error {
	destDir, err := s.resolve(destDir)
	if err != nil {
		return err
	}
	t := &tracker{ctx: ctx, emit: s.emit, p: Progress{Job: job, Phase: "scan"}}
	t.send(true)
	// Always report completion, including after cancellation, so the UI knows
	// cleanup has finished.
	defer func() {
		t.p.Phase = "done"
		t.send(true)
	}()

	var items []transferItem
	var errs []error
	for _, src := range sources {
		dst := filepath.Join(destDir, filepath.Base(src))
		if err := checkTarget(src, dst, overwrite); err != nil {
			errs = append(errs, err)
			continue
		}
		_, statErr := os.Lstat(dst)
		existed := statErr == nil
		// A same-volume move is a rename, unless a folder has to be merged.
		if move && !(existed && isDir(src)) && os.Rename(src, dst) == nil {
			continue
		}
		items = append(items, transferItem{src, dst, existed})
	}

	for _, it := range items {
		size, count := treeSize(ctx, it.src)
		t.p.Total += size
		t.p.TotalFiles += count
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	t.p.Phase = "copy"
	t.send(true)

	for _, it := range items {
		if err := t.copyPath(it.src, it.dst, overwrite); err != nil {
			if !it.existed {
				os.RemoveAll(it.dst)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(it.src), err))
			continue
		}
		if move {
			if err := os.RemoveAll(it.src); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func checkTarget(src, dst string, overwrite bool) error {
	name := filepath.Base(src)
	if src == dst {
		return fmt.Errorf("%s: source and destination are the same", name)
	}
	if strings.HasPrefix(dst, src+string(filepath.Separator)) {
		return fmt.Errorf("%s: cannot copy a folder into itself", name)
	}
	if _, err := os.Lstat(dst); err == nil {
		if !overwrite {
			return fmt.Errorf("%s: already exists in destination", name)
		}
		if isDir(src) != isDir(dst) {
			return fmt.Errorf("%s: cannot replace a folder with a file or vice versa", name)
		}
	}
	return nil
}

// isDir reports whether p is a real directory (symlinks are not followed).
func isDir(p string) bool {
	info, err := os.Lstat(p)
	return err == nil && info.IsDir()
}

func treeSize(ctx context.Context, root string) (size int64, files int) {
	filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || d.IsDir() {
			return nil
		}
		files++
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			size += info.Size()
		}
		return nil
	})
	return size, files
}

func (t *tracker) copyPath(src, dst string, overwrite bool) error {
	if err := t.ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if overwrite {
		if err := checkTarget(src, dst, true); err != nil {
			return err
		}
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if overwrite {
			os.Remove(dst)
		}
		t.p.Files++
		return os.Symlink(target, dst)
	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()|0o700); err != nil {
			return err
		}
		des, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, de := range des {
			if err := t.copyPath(filepath.Join(src, de.Name()), filepath.Join(dst, de.Name()), overwrite); err != nil {
				return err
			}
		}
		return os.Chmod(dst, info.Mode().Perm())
	default:
		return t.copyFile(src, dst, info)
	}
}

// copyFile writes into a temporary file next to dst and renames it into
// place, so a failed or cancelled copy never leaves a half-written dst.
func (t *tracker) copyFile(src, dst string, info os.FileInfo) error {
	t.p.Current = filepath.Base(src)
	t.send(false)
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".*.part")
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	buf := make([]byte, 1<<20)
	for {
		if err := t.ctx.Err(); err != nil {
			return err
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, err := tmp.Write(buf[:n]); err != nil {
				return err
			}
			t.p.Done += int64(n)
			t.send(false)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Chtimes(tmp.Name(), time.Now(), info.ModTime()); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return err
	}
	ok = true
	t.p.Files++
	return nil
}
