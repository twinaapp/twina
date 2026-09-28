//go:build linux

package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// trashPaths implements the freedesktop.org home trash.
func trashPaths(paths []string) map[string]error {
	failed := map[string]error{}
	for _, p := range paths {
		if err := trashOne(p); err != nil {
			failed[p] = err
		}
	}
	return failed
}

func trashOne(p string) error {
	root := os.Getenv("XDG_DATA_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		root = filepath.Join(home, ".local", "share")
	}
	filesDir := filepath.Join(root, "Trash", "files")
	infoDir := filepath.Join(root, "Trash", "info")
	if err := os.MkdirAll(filesDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(infoDir, 0o700); err != nil {
		return err
	}

	base := filepath.Base(p)
	for i := 1; i < 1000; i++ {
		name := base
		if i > 1 {
			name = base + "." + strconv.Itoa(i)
		}
		info, err := os.OpenFile(filepath.Join(infoDir, name+".trashinfo"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(info, "[Trash Info]\nPath=%s\nDeletionDate=%s\n",
			(&url.URL{Path: p}).EscapedPath(), time.Now().Format("2006-01-02T15:04:05"))
		info.Close()
		if err := os.Rename(p, filepath.Join(filesDir, name)); err != nil {
			os.Remove(info.Name())
			return fmt.Errorf("cannot move to Trash: %w", err)
		}
		return nil
	}
	return fmt.Errorf("cannot find a free name in the Trash")
}
