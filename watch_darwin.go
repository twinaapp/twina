//go:build darwin

package main

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsevents"
)

// fseWatcher uses FSEvents rather than fsnotify's kqueue backend, which opens
// a file descriptor for every file in a watched folder and runs out of them
// in large folders.
type fseWatcher struct {
	mu    sync.Mutex
	hit   func(dir string)
	es    *fsevents.EventStream
	stop  chan struct{}
	paths []string
	shown map[string]string // resolved path (as FSEvents reports it) → path the panel shows
}

func newDirWatcher(hit func(dir string)) dirWatcher { return &fseWatcher{hit: hit} }

func (f *fseWatcher) set(dirs []string) {
	shown := map[string]string{}
	var paths []string
	for _, d := range dirs {
		// FSEvents reports resolved paths (/private/tmp for /tmp).
		r, err := filepath.EvalSymlinks(d)
		if err != nil {
			r = d
		}
		shown[r] = d
		paths = append(paths, r)
	}
	slices.Sort(paths)

	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = shown
	if slices.Equal(paths, f.paths) {
		return
	}
	f.paths = paths
	if f.es != nil {
		f.es.Stop()
		close(f.stop)
		f.es = nil
	}
	if len(paths) == 0 {
		return
	}
	es := &fsevents.EventStream{
		Paths:   paths,
		Latency: 100 * time.Millisecond,
		Flags:   fsevents.FileEvents | fsevents.WatchRoot | fsevents.NoDefer,
	}
	if es.Start() != nil {
		f.paths = nil
		return
	}
	stop := make(chan struct{})
	f.es, f.stop = es, stop
	go func() {
		for {
			select {
			case <-stop:
				return
			case events := <-es.Events:
				for _, e := range events {
					f.event(e)
				}
			}
		}
	}()
}

func (f *fseWatcher) event(e fsevents.Event) {
	p := e.Path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	// Streams are recursive; only direct entries of a watched folder count,
	// plus the folder itself (renamed or deleted).
	if d, ok := f.shown[filepath.Dir(p)]; ok {
		f.hit(d)
	}
	if d, ok := f.shown[p]; ok && e.Flags&(fsevents.ItemRemoved|fsevents.ItemRenamed|fsevents.RootChanged) != 0 {
		f.hit(d)
	}
}
