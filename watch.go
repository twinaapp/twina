package main

import (
	"sync"
	"time"
)

// dirChangedEvent carries the path of an open folder whose contents changed
// outside the app (a terminal, another app), so its panels can reload.
const dirChangedEvent = "fs:changed"

// dirWatcher reports changes to the entries of a set of directories (not their
// subdirectories). Implementations are per platform.
type dirWatcher interface {
	set(dirs []string)
}

type noWatcher struct{}

func (noWatcher) set([]string) {}

// Watch replaces the set of watched folders with the ones open in any tab.
func (s *FileService) Watch(dirs []string) {
	s.watchOnce.Do(func() {
		d := &debouncer{wait: 150 * time.Millisecond, maxWait: time.Second, fn: func(dir string) {
			if s.changed != nil {
				s.changed(dir)
			}
		}}
		s.watcher = newDirWatcher(d.hit)
	})
	seen := map[string]bool{}
	var unique []string
	for _, d := range dirs {
		if d != "" && !seen[d] {
			seen[d] = true
			unique = append(unique, d)
		}
	}
	s.watcher.set(unique)
}

// debouncer calls fn once per directory after a burst of changes settles, so
// copying a thousand files doesn't reload the panel a thousand times. During a
// long burst it still fires every maxWait, so files show up as they arrive.
type debouncer struct {
	wait, maxWait time.Duration
	fn            func(dir string)
	mu            sync.Mutex
	pending       map[string]*pendingDir
}

type pendingDir struct {
	timer *time.Timer
	first time.Time
}

func (d *debouncer) hit(dir string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pending == nil {
		d.pending = map[string]*pendingDir{}
	}
	if p, ok := d.pending[dir]; ok {
		p.timer.Reset(min(d.wait, max(0, d.maxWait-time.Since(p.first))))
		return
	}
	d.pending[dir] = &pendingDir{first: time.Now(), timer: time.AfterFunc(d.wait, func() {
		d.mu.Lock()
		delete(d.pending, dir)
		d.mu.Unlock()
		d.fn(dir)
	})}
}
