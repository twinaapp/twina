package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// mediaPath is where the viewer loads image and video bytes from:
// /localfile?path=… The webview decodes and plays them itself, so no media
// libraries are needed.
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

// mediaType returns the Content-Type for a viewable image or video, or "".
func mediaType(path string) string {
	return mediaTypes[strings.ToLower(filepath.Ext(path))]
}

// withMedia serves image and video files from disk at mediaPath and passes
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
