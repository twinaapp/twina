package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// windowState is what Twina remembers between runs. It lives in the platform's
// state directory (Application Support, %LocalAppData%, ~/.local/state), not
// with preferences, because window positions are specific to this machine.
type windowState struct {
	Bounds    application.Rect `json:"bounds"`
	Maximised bool             `json:"maximised"`
}

func statePath() string { return filepath.Join(xdg.StateHome, "Twina", "state.json") }

func loadWindowState() (windowState, bool) {
	var s windowState
	b, err := os.ReadFile(statePath())
	if err != nil || json.Unmarshal(b, &s) != nil || s.Bounds.Width <= 0 || s.Bounds.Height <= 0 {
		return windowState{}, false
	}
	return s, true
}

func saveWindowState(s windowState) {
	b, _ := json.MarshalIndent(s, "", "  ")
	p := statePath()
	if os.MkdirAll(filepath.Dir(p), 0o755) != nil {
		return
	}
	tmp := p + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, p)
	}
}

// screens waits briefly for the screen list: on macOS it's filled in by a
// startup hook that races with ApplicationStarted.
func screens(app *application.App) []*application.Screen {
	for range 50 {
		if s := app.Screen.GetAll(); len(s) > 0 {
			return s
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

// fitOnScreen returns r if at least a grabbable strip of it (the title bar
// area) is on one of the screens, clamped to that screen's work area. ok is
// false when r is off every screen, e.g. after unplugging a monitor. With no
// screen list at all, r is trusted as is.
func fitOnScreen(r application.Rect, screens []*application.Screen) (application.Rect, bool) {
	if len(screens) == 0 {
		return r, true
	}
	const grab = 80
	for _, s := range screens {
		wa := s.WorkArea
		if r.X+r.Width-grab < wa.X || r.X+grab > wa.X+wa.Width || r.Y < wa.Y || r.Y+grab > wa.Y+wa.Height {
			continue
		}
		r.Width, r.Height = min(r.Width, wa.Width), min(r.Height, wa.Height)
		r.X = max(wa.X, min(r.X, wa.X+wa.Width-r.Width))
		r.Y = max(wa.Y, min(r.Y, wa.Y+wa.Height-r.Height))
		return r, true
	}
	return r, false
}

// rememberWindow restores w's last size and position (w must be created
// hidden) and saves them as the user moves, resizes or closes it.
func rememberWindow(app *application.App, w *application.WebviewWindow) {
	state, saved := loadWindowState()

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if saved {
			if r, ok := fitOnScreen(state.Bounds, screens(app)); ok {
				w.SetBounds(r)
			} else {
				w.SetSize(state.Bounds.Width, state.Bounds.Height)
				w.Center()
			}
			if state.Maximised {
				w.Maximise()
			}
		}
		w.Show()
	})

	var mu sync.Mutex
	var timer *time.Timer
	save := func() {
		mu.Lock()
		defer mu.Unlock()
		// Keep the normal-size bounds while maximised or fullscreen, so
		// un-maximising after a restart returns to the size the user chose.
		if state.Maximised = w.IsMaximised(); !state.Maximised && !w.IsFullscreen() {
			state.Bounds = w.Bounds()
		}
		saveWindowState(state)
	}
	later := func(*application.WindowEvent) {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(500*time.Millisecond, save)
	}
	w.OnWindowEvent(events.Common.WindowDidMove, later)
	w.OnWindowEvent(events.Common.WindowDidResize, later)
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { save() })
}
