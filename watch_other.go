//go:build !darwin

package main

import (
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// fsnWatcher uses inotify on Linux and ReadDirectoryChangesW on Windows, both
// of which watch a folder's entries without per-file handles.
type fsnWatcher struct {
	mu   sync.Mutex
	w    *fsnotify.Watcher
	hit  func(dir string)
	dirs map[string]bool
}

func newDirWatcher(hit func(dir string)) dirWatcher {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return noWatcher{}
	}
	f := &fsnWatcher{w: w, hit: hit, dirs: map[string]bool{}}
	go func() {
		for {
			select {
			case e, ok := <-w.Events:
				if !ok {
					return
				}
				f.event(e.Name)
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
	return f
}

func (f *fsnWatcher) set(dirs []string) {
	want := map[string]bool{}
	for _, d := range dirs {
		want[d] = true
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for d := range f.dirs {
		if !want[d] {
			f.w.Remove(d)
			delete(f.dirs, d)
		}
	}
	for d := range want {
		if !f.dirs[d] && f.w.Add(d) == nil {
			f.dirs[d] = true
		}
	}
}

func (f *fsnWatcher) event(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if dir := filepath.Dir(name); f.dirs[dir] {
		f.hit(dir)
	}
	if f.dirs[name] { // the watched folder itself was renamed or deleted
		f.hit(name)
	}
}
