package main

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// mediaPath is where the viewer loads image, video and PDF bytes from:
// /localfile?path=… The webview decodes, plays and renders them itself, so no
// media libraries are needed.
const mediaPath = "/localfile"

// mediaTypes lists the extensions the viewer shows as pictures or plays as
// video. Formats a webview can't handle (HEIC and TIFF outside macOS, MKV and
// AVI almost everywhere) show a message instead, with F4 still opening them in
// the default app.
var mediaTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
	".heic": "image/heic",
	".tif":  "image/tiff",
	".tiff": "image/tiff",

	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".ogv":  "video/ogg",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
}

// WebKitGTK has no PDF viewer, so on Linux PDFs stay binary files.
func init() {
	if runtime.GOOS != "linux" {
		mediaTypes[".pdf"] = "application/pdf"
	}
}

// mediaType returns the Content-Type for a viewable image, video or PDF, or "".
func mediaType(path string) string {
	return mediaTypes[strings.ToLower(filepath.Ext(path))]
}

// withMedia serves image, video and PDF files from disk at mediaPath and passes
// every other request to next. Range requests, which video seeking relies on,
// are answered by http.ServeContent.
func withMedia(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != mediaPath {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		ctype := mediaType(path)
		if ctype == "" || !filepath.IsAbs(path) {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		// Always send the whole current file. The webview's scheme handler has
		// no HTTP cache to satisfy a 304 from, and the file may have changed.
		for _, h := range []string{"If-Modified-Since", "If-None-Match", "If-Range", "If-Unmodified-Since", "If-Match"} {
			r.Header.Del(h)
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	})
}

// viewerCloseEvent asks the viewer to close. It is emitted for Esc and F3
// while CatchViewerKeys is on.
const viewerCloseEvent = "viewer:close"

// viewerKeys are the accelerators, in Wails' normalised form, that close the
// viewer while it shows a PDF.
var viewerKeys = []string{"ESCAPE", "F3"}

// CatchViewerKeys turns native Esc and F3 handling on while the viewer shows
// a PDF and off again after. Once the user clicks into the PDF, the webview's
// PDF viewer takes the keyboard and the page never sees those keys.
func (s *FileService) CatchViewerKeys(on bool) {
	if s.viewerKeys != nil {
		s.viewerKeys(on)
	}
}

func catchViewerKeys(app *application.App, on bool) {
	for _, key := range viewerKeys {
		if !on {
			app.KeyBinding.Remove(key)
			continue
		}
		app.KeyBinding.Add(key, func(application.Window) {
			// Drop the bindings straight away, so a page reload while a PDF
			// was open can't leave Esc caught for the whole app.
			catchViewerKeys(app, false)
			app.Event.Emit(viewerCloseEvent)
		})
	}
}
