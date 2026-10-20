package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMedia(t *testing.T) {
	root := t.TempDir()
	png := filepath.Join(root, "Pic.PNG")
	write(t, png, "\x89PNG fake")
	write(t, filepath.Join(root, "notes.txt"), "text")

	ft, err := (&FileService{}).ReadText(png)
	if err != nil || !ft.Image || ft.Content != "" || ft.Size != 9 {
		t.Fatalf("ReadText image: %+v, %v", ft, err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := withMedia(next)
	get := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, mediaPath+"?path="+url.QueryEscape(path), nil))
		return rec
	}

	if rec := get("GET", png); rec.Code != 200 || rec.Body.String() != "\x89PNG fake" || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("image: %d %q %q", rec.Code, rec.Body, rec.Header().Get("Content-Type"))
	}
	for _, p := range []string{filepath.Join(root, "notes.txt"), filepath.Join(root, "missing.png"), "Pic.PNG"} {
		if rec := get("GET", p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", p, rec.Code)
		}
	}
	if rec := get("POST", png); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d", rec.Code)
	}

	// Video seeking asks for byte ranges.
	mp4 := filepath.Join(root, "clip.mp4")
	write(t, mp4, "0123456789")
	if ft, _ := (&FileService{}).ReadText(mp4); !ft.Video || ft.Image {
		t.Fatalf("ReadText video: %+v", ft)
	}
	req := httptest.NewRequest("GET", mediaPath+"?path="+url.QueryEscape(mp4), nil)
	req.Header.Set("Range", "bytes=4-6")
	req.Header.Set("If-Modified-Since", "Mon, 01 Jan 2224 00:00:00 GMT")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "456" || rec.Header().Get("Content-Range") != "bytes 4-6/10" {
		t.Fatalf("range: %d %q %q", rec.Code, rec.Body, rec.Header().Get("Content-Range"))
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/index.html", nil))
	if rec.Code != http.StatusTeapot {
		t.Errorf("other paths should reach the asset handler, got %d", rec.Code)
	}

	// PDFs are rendered by the webview, except on Linux where it has no viewer.
	pdf := filepath.Join(root, "doc.pdf")
	write(t, pdf, "%PDF-1.7 fake")
	ft, err = (&FileService{}).ReadText(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "linux" {
		if ft.PDF || !ft.Binary {
			t.Fatalf("ReadText PDF on Linux: %+v", ft)
		}
		return
	}
	if !ft.PDF || ft.Image || ft.Video || ft.Content != "" {
		t.Fatalf("ReadText PDF: %+v", ft)
	}
	if rec := get("GET", pdf); rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("PDF: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}
