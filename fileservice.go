package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"
)

// FileService exposes filesystem operations to the frontend panels.
type FileService struct {
	emit func(Progress) // reports copy/move progress; nil in tests
}

type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	IsLink  bool   `json:"isLink"`
	Hidden  bool   `json:"hidden"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"` // unix milliseconds
	Mode    string `json:"mode"`
	Ext     string `json:"ext"`
}

type Listing struct {
	Path    string  `json:"path"`
	Parent  string  `json:"parent"` // empty at filesystem root
	Entries []Entry `json:"entries"`
}

type FileText struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Binary    bool   `json:"binary"`
	Size      int64  `json:"size"`
}

const maxViewBytes = 2 << 20

func (s *FileService) Home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return string(filepath.Separator)
}

// List returns the entries of a directory. Paths starting with "~" are
// expanded to the user's home directory.
func (s *FileService) List(path string) (*Listing, error) {
	path, err := s.resolve(path)
	if err != nil {
		return nil, err
	}
	des, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(des))
	for _, de := range des {
		full := filepath.Join(path, de.Name())
		info, err := de.Info()
		if err != nil {
			continue
		}
		e := Entry{
			Name:    de.Name(),
			Path:    full,
			IsDir:   info.IsDir(),
			IsLink:  info.Mode()&os.ModeSymlink != 0,
			Hidden:  strings.HasPrefix(de.Name(), "."),
			Size:    info.Size(),
			ModTime: info.ModTime().UnixMilli(),
			Mode:    info.Mode().String(),
		}
		if e.IsLink {
			// Follow the link so symlinked directories are navigable.
			if target, err := os.Stat(full); err == nil {
				e.IsDir = target.IsDir()
				if !e.IsDir {
					e.Size = target.Size()
				}
			}
		}
		if !e.IsDir {
			e.Ext = extOf(e.Name)
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parent := filepath.Dir(path)
	if parent == path {
		parent = ""
	}
	return &Listing{Path: path, Parent: parent, Entries: entries}, nil
}

type TrashResult struct {
	Failed  []string `json:"failed"`  // paths that could not be moved to the Trash
	Message string   `json:"message"` // first failure reason
}

// Trash moves the given paths to the system Trash. Items that cannot be
// trashed are reported back so the user can decide to delete them for good.
func (s *FileService) Trash(paths []string) (*TrashResult, error) {
	res := &TrashResult{}
	failed := trashPaths(paths)
	for _, p := range paths {
		if err, ok := failed[p]; ok {
			res.Failed = append(res.Failed, p)
			if res.Message == "" {
				res.Message = err.Error()
			}
		}
	}
	return res, nil
}

// DeletePermanently removes the given paths without using the Trash.
func (s *FileService) DeletePermanently(ctx context.Context, paths []string) error {
	var errs []error
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(p), err))
		}
	}
	return errors.Join(errs...)
}

// extOf returns the lower-cased extension without the dot. A leading dot
// marks a hidden file, not an extension: ".gitignore" has none, while
// ".env.local" has "local".
func extOf(name string) string {
	ext := filepath.Ext(name)
	if len(ext) == len(name) {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

func (s *FileService) MakeDir(parent, name string) (string, error) {
	if err := validName(name); err != nil {
		return "", err
	}
	p := filepath.Join(parent, name)
	return p, os.Mkdir(p, 0o755)
}

func (s *FileService) Rename(path, newName string) (string, error) {
	if err := validName(newName); err != nil {
		return "", err
	}
	dst := filepath.Join(filepath.Dir(path), newName)
	if dst == path {
		return path, nil
	}
	if _, err := os.Lstat(dst); err == nil {
		return "", fmt.Errorf("%s already exists", newName)
	}
	return dst, os.Rename(path, dst)
}

func validName(name string) error {
	// "/" separates paths on every platform (Windows accepts it too), "\" only on Windows.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/"+string(filepath.Separator)) {
		return fmt.Errorf("invalid name %q", name)
	}
	return nil
}

// Open launches the file with the system's default application.
func (s *FileService) Open(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// ReadText returns up to maxViewBytes of a file for the built-in viewer.
func (s *FileService) ReadText(path string) (*FileText, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(io.LimitReader(f, maxViewBytes))
	if err != nil {
		return nil, err
	}
	ft := &FileText{Path: path, Size: info.Size(), Truncated: info.Size() > int64(len(buf))}
	probe := buf[:min(len(buf), 8192)]
	if strings.ContainsRune(string(probe), 0) || !utf8.Valid(trimPartialRune(probe)) {
		ft.Binary = true
		return ft, nil
	}
	ft.Content = string(buf)
	return ft, nil
}

// trimPartialRune drops a UTF-8 sequence cut off at the end of the buffer.
func trimPartialRune(b []byte) []byte {
	for i := 0; i < 3 && len(b) > 0; i++ {
		if r, _ := utf8.DecodeLastRune(b); r != utf8.RuneError {
			break
		}
		b = b[:len(b)-1]
	}
	return b
}

func (s *FileService) resolve(path string) (string, error) {
	if path == "" || path == "~" {
		return s.Home(), nil
	}
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(s.Home(), path[2:])
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}
	return abs, nil
}
