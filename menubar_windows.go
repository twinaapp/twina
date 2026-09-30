package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// redrawMenuBar repaints the menu bar once the window is shown. In dark mode
// Wails paints the bar's background on every move/resize but draws the items
// separately, so after placing a hidden window the labels stay blank until the
// mouse hovers over them.
func redrawMenuBar(w *application.WebviewWindow) {
	hwnd := w32.HWND(uintptr(w.NativeWindow()))
	application.InvokeAsync(func() {
		w32.InvalidateRect(hwnd, nil, true)
		w32.DrawMenuBar(hwnd)
	})
}
