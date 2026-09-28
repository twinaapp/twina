package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCopyMoveRename(t *testing.T) {
	ctx := context.Background()
	s := &FileService{}
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	write(t, filepath.Join(src, "a.txt"), "hello")
	write(t, filepath.Join(src, "dir", "b.txt"), "nested")
	os.Mkdir(dst, 0o755)

	if err := s.Copy(ctx, "t", []string{filepath.Join(src, "a.txt"), filepath.Join(src, "dir")}, dst, false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "dir", "b.txt")); string(b) != "nested" {
		t.Fatalf("nested copy: got %q", b)
	}
	if err := s.Copy(ctx, "t", []string{filepath.Join(src, "a.txt")}, dst, false); err == nil {
		t.Fatal("expected error copying over existing file")
	}
	if err := s.Copy(ctx, "t", []string{src}, filepath.Join(src, "dir"), false); err == nil {
		t.Fatal("expected error copying directory into itself")
	}

	if err := s.Move(ctx, "t", []string{filepath.Join(src, "dir")}, filepath.Join(dst, "dir"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "dir")); !os.IsNotExist(err) {
		t.Fatal("source still exists after move")
	}

	if _, err := s.Rename(filepath.Join(dst, "a.txt"), "c.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rename(filepath.Join(dst, "c.txt"), "../x"); err == nil {
		t.Fatal("expected invalid name error")
	}

	l, err := s.List(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) != 2 || l.Entries[0].Name != "c.txt" || !l.Entries[1].IsDir {
		t.Fatalf("unexpected listing: %+v", l.Entries)
	}
	if l.Parent != root {
		t.Fatalf("parent = %q, want %q", l.Parent, root)
	}
}

func TestReadText(t *testing.T) {
	s := &FileService{}
	dir := t.TempDir()
	write(t, filepath.Join(dir, "t.txt"), "héllo")
	write(t, filepath.Join(dir, "b.bin"), "\x00\x01\x02")
	if ft, _ := s.ReadText(filepath.Join(dir, "t.txt")); ft.Binary || ft.Content != "héllo" {
		t.Fatalf("text: %+v", ft)
	}
	if ft, _ := s.ReadText(filepath.Join(dir, "b.bin")); !ft.Binary {
		t.Fatal("expected binary")
	}
}

func TestOverwriteMerges(t *testing.T) {
	ctx := context.Background()
	var last Progress
	s := &FileService{emit: func(p Progress) { last = p }}
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	write(t, filepath.Join(src, "d", "new.txt"), "new")
	write(t, filepath.Join(src, "d", "same.txt"), "from src")
	write(t, filepath.Join(dst, "d", "same.txt"), "old")
	write(t, filepath.Join(dst, "d", "keep.txt"), "keep")

	conflicts, err := s.Conflicts([]string{filepath.Join(src, "d")}, dst)
	if err != nil || len(conflicts) != 1 || conflicts[0] != "d" {
		t.Fatalf("conflicts = %v, %v", conflicts, err)
	}
	if err := s.Copy(ctx, "job1", []string{filepath.Join(src, "d")}, dst, true); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"new.txt": "new", "same.txt": "from src", "keep.txt": "keep"} {
		if b, _ := os.ReadFile(filepath.Join(dst, "d", name)); string(b) != want {
			t.Errorf("%s = %q, want %q", name, b, want)
		}
	}
	if last.Job != "job1" || last.Phase != "done" || last.Files != 2 || last.Done != last.Total {
		t.Fatalf("progress = %+v", last)
	}

	// Moving a merged folder removes the source.
	if err := s.Move(ctx, "job2", []string{filepath.Join(src, "d")}, dst, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "d")); !os.IsNotExist(err) {
		t.Fatal("source still exists after merge move")
	}

	write(t, filepath.Join(src, "d"), "a file now")
	if err := s.Copy(ctx, "job3", []string{filepath.Join(src, "d")}, dst, true); err == nil {
		t.Fatal("expected error replacing a folder with a file")
	}
}

func TestCancelRemovesPartialCopy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &FileService{}
	root := t.TempDir()
	write(t, filepath.Join(root, "src", "big", "a.txt"), "data")
	os.Mkdir(filepath.Join(root, "dst"), 0o755)
	err := s.Copy(ctx, "j", []string{filepath.Join(root, "src", "big")}, filepath.Join(root, "dst"), false)
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(filepath.Join(root, "dst", "big")); !os.IsNotExist(err) {
		t.Fatal("partial copy left behind")
	}
}

// Touches the real Trash, so it only runs when asked: TWINA_TRASH_TEST=1.
func TestTrash(t *testing.T) {
	if os.Getenv("TWINA_TRASH_TEST") == "" {
		t.Skip("set TWINA_TRASH_TEST=1 to run")
	}
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, "twina-trash-test.txt")
	write(t, p, "trash me")
	res, err := (&FileService{}).Trash([]string{p})
	if err != nil || len(res.Failed) > 0 {
		t.Fatalf("trash: %+v %v", res, err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("file still exists")
	}
}

func TestExtOf(t *testing.T) {
	for name, want := range map[string]string{
		".gitignore":     "",
		".env":           "",
		".env.local":     "local",
		"archive.tar.GZ": "gz",
		"README":         "",
		"notes.":         "",
		"photo.JPG":      "jpg",
	} {
		if got := extOf(name); got != want {
			t.Errorf("extOf(%q) = %q, want %q", name, got, want)
		}
	}
}
