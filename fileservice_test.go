package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
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

func TestMatchAsset(t *testing.T) {
	var assets []github.ReleaseAsset
	for _, n := range []string{
		"SHA256SUMS.txt",
		"Twina-1.2.0-linux-amd64.rpm", "Twina-1.2.0-linux-amd64.deb", "Twina-1.2.0-linux-arm64.deb",
		"Twina-1.2.0-macos-universal.dmg", "Twina-1.2.0-macos-universal.zip",
		"Twina-1.2.0-windows-amd64-setup.exe", "Twina-1.2.0-windows-amd64-portable.exe",
		"Twina-1.2.0-windows-arm64-portable.exe",
	} {
		assets = append(assets, github.ReleaseAsset{Name: n})
	}
	for req, want := range map[updater.CheckRequest]string{
		{Platform: "darwin", Arch: "arm64"}:  "Twina-1.2.0-macos-universal.zip",
		{Platform: "darwin", Arch: "amd64"}:  "Twina-1.2.0-macos-universal.zip",
		{Platform: "windows", Arch: "amd64"}: "Twina-1.2.0-windows-amd64-portable.exe",
		{Platform: "windows", Arch: "arm64"}: "Twina-1.2.0-windows-arm64-portable.exe",
		{Platform: "linux", Arch: "amd64"}:   "Twina-1.2.0-linux-amd64.deb",
		{Platform: "linux", Arch: "arm64"}:   "Twina-1.2.0-linux-arm64.deb",
	} {
		i := matchAsset(req, assets)
		if i < 0 || assets[i].Name != want {
			t.Errorf("%s/%s: got index %d, want %s", req.Platform, req.Arch, i, want)
		}
	}
	if i := matchAsset(updater.CheckRequest{Platform: "freebsd", Arch: "amd64"}, assets); i != -1 {
		t.Errorf("freebsd: got %d, want -1", i)
	}
}

func TestFitOnScreen(t *testing.T) {
	screens := []*application.Screen{
		{WorkArea: application.Rect{X: 0, Y: 25, Width: 1440, Height: 875}},
		{WorkArea: application.Rect{X: 1440, Y: 0, Width: 1920, Height: 1080}},
	}
	for _, c := range []struct {
		name string
		in   application.Rect
		want application.Rect
		ok   bool
	}{
		{"fits", application.Rect{X: 100, Y: 100, Width: 1000, Height: 600}, application.Rect{X: 100, Y: 100, Width: 1000, Height: 600}, true},
		{"second screen", application.Rect{X: 1600, Y: 50, Width: 1280, Height: 800}, application.Rect{X: 1600, Y: 50, Width: 1280, Height: 800}, true},
		{"too big, clamped", application.Rect{X: 0, Y: 25, Width: 2000, Height: 1200}, application.Rect{X: 0, Y: 25, Width: 1440, Height: 875}, true},
		{"hangs off right, pulled in", application.Rect{X: 3000, Y: 100, Width: 1000, Height: 600}, application.Rect{X: 2360, Y: 100, Width: 1000, Height: 600}, true},
		{"unplugged monitor", application.Rect{X: -2000, Y: 100, Width: 1000, Height: 600}, application.Rect{X: -2000, Y: 100, Width: 1000, Height: 600}, false},
		{"title bar above screen", application.Rect{X: 100, Y: -500, Width: 1000, Height: 600}, application.Rect{X: 100, Y: -500, Width: 1000, Height: 600}, false},
	} {
		got, ok := fitOnScreen(c.in, screens)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: got %+v %v, want %+v %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestWatch(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0o755)
	got := make(chan string, 16)
	s := &FileService{changed: func(d string) { got <- d }}
	s.Watch([]string{dir, dir}) // duplicates are fine
	time.Sleep(300 * time.Millisecond)

	expect := func(what string) {
		t.Helper()
		select {
		case d := <-got:
			if d != dir {
				t.Fatalf("%s: changed %q, want %q", what, d, dir)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s: no change reported", what)
		}
		time.Sleep(300 * time.Millisecond)
		for len(got) > 0 {
			<-got
		}
	}
	write(t, filepath.Join(dir, "new.txt"), "x")
	expect("create")
	os.Rename(filepath.Join(dir, "new.txt"), filepath.Join(dir, "renamed.txt"))
	expect("rename")
	os.Remove(filepath.Join(dir, "renamed.txt"))
	expect("delete")

	// Changes inside a subfolder aren't the watched folder's entries.
	write(t, filepath.Join(sub, "deep.txt"), "x")
	select {
	case d := <-got:
		t.Fatalf("change in subfolder reported as %q", d)
	case <-time.After(time.Second):
	}

	s.Watch(nil)
	time.Sleep(300 * time.Millisecond)
	write(t, filepath.Join(dir, "after.txt"), "x")
	select {
	case d := <-got:
		t.Fatalf("unwatched folder reported %q", d)
	case <-time.After(time.Second):
	}
}

func TestDebouncer(t *testing.T) {
	var mu sync.Mutex
	var fired []time.Duration
	start := time.Now()
	d := &debouncer{wait: 50 * time.Millisecond, maxWait: 200 * time.Millisecond, fn: func(string) {
		mu.Lock()
		fired = append(fired, time.Since(start))
		mu.Unlock()
	}}
	// A steady stream of changes for 500ms: without maxWait nothing would fire
	// until it stops.
	for time.Since(start) < 500*time.Millisecond {
		d.hit("/a")
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(fired) < 3 {
		t.Fatalf("fired %d times (%v), want a reload about every 200ms plus one at the end", len(fired), fired)
	}
	if fired[0] > 300*time.Millisecond {
		t.Errorf("first reload after %v, want within maxWait", fired[0])
	}
}
