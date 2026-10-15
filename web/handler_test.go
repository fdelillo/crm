package web_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fdelillo/crm/web"
)

const documentCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; font-src 'self'; connect-src 'self'; manifest-src 'self'; worker-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

func builtFiles() fstest.MapFS {
	return fstest.MapFS{
		"index.html":              {Data: []byte("<!doctype html><html>" + strings.Repeat("application ", 150) + "</html>")},
		"assets/index-abc123.js":  {Data: []byte(strings.Repeat("const app = 'crm';\n", 100))},
		"assets/index-abc123.css": {Data: []byte(strings.Repeat("body { color: black; }\n", 100))},
		"assets/small-abc123.js":  {Data: []byte("export default 1;")},
		"sw.js":                   {Data: []byte(strings.Repeat("// service worker\n", 100))},
		"manifest.webmanifest":    {Data: []byte(`{"name":"` + strings.Repeat("CRM ", 300) + `"}`)},
		"offline.html":            {Data: []byte("<!doctype html><html>Sin conexión</html>")},
		"icons/icon-192.png":      {Data: append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 2048)...)},
		"favicon.ico":             {Data: bytes.Repeat([]byte{0}, 2048)},
	}
}

func serve(handler http.Handler, method, target, encoding string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if encoding != "" {
		req.Header.Set("Accept-Encoding", encoding)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// T-F007: resources and route fallbacks follow ADR-019 and plan §10.7.
func TestHandlerResources(t *testing.T) {
	files := builtFiles()
	handler := web.NewHandler(files)
	for _, tt := range []struct {
		path, file, mediaType, cache string
		html                         bool
	}{
		{"/", "index.html", "text/html", "no-cache", true},
		{"/index.html", "index.html", "text/html", "no-cache", true},
		{"/settings/users", "index.html", "text/html", "no-cache", true},
		{"/reset-password", "index.html", "text/html", "no-cache", true},
		{"/assets/index-abc123.js", "assets/index-abc123.js", "text/javascript", "public, max-age=31536000, immutable", false},
		{"/assets/index-abc123.css", "assets/index-abc123.css", "text/css", "public, max-age=31536000, immutable", false},
		{"/sw.js", "sw.js", "text/javascript", "no-cache", false},
		{"/manifest.webmanifest", "manifest.webmanifest", "application/manifest+json", "no-cache", false},
		{"/offline.html", "offline.html", "text/html", "no-cache", true},
		{"/icons/icon-192.png", "icons/icon-192.png", "image/png", "public, max-age=86400", false},
		{"/favicon.ico", "favicon.ico", "image/vnd.microsoft.icon", "public, max-age=86400", false},
	} {
		t.Run(tt.path, func(t *testing.T) {
			rec := serve(handler, http.MethodGet, tt.path, "")
			if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), files[tt.file].Data) {
				t.Fatalf("status = %d body = %q, want file %s with 200", rec.Code, rec.Body.String(), tt.file)
			}
			mediaType, _, err := mime.ParseMediaType(rec.Header().Get("Content-Type"))
			if tt.file == "favicon.ico" && mediaType == "image/x-icon" {
				mediaType = "image/vnd.microsoft.icon"
			}
			if err != nil || mediaType != tt.mediaType {
				t.Errorf("Content-Type = %q, want %s", rec.Header().Get("Content-Type"), tt.mediaType)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.cache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cache)
			}
			wantCSP := ""
			if tt.html {
				wantCSP = documentCSP
			}
			if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
				t.Errorf("CSP = %q, want %q", got, wantCSP)
			}
		})
	}
}

// T-F007: a stale asset is never served or cached as the SPA document.
func TestHandlerMissingFiles(t *testing.T) {
	for _, target := range []string{"/assets/viejo-def456.js", "/missing.css", "/icons/missing.png"} {
		t.Run(target, func(t *testing.T) {
			rec := serve(web.NewHandler(builtFiles()), http.MethodGet, target, "")
			if rec.Code != http.StatusNotFound || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") || strings.Contains(rec.Body.String(), "<html") {
				t.Fatalf("status = %d type = %q body = %q, want plain-text 404", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

// T-F007: the Go backend works from a checkout with only the dist marker.
func TestHandlerWithoutBuild(t *testing.T) {
	handler := web.NewHandler(fstest.MapFS{".gitkeep": {}})
	for _, target := range []string{"/", "/login", "/assets/index-abc123.js"} {
		t.Run(target, func(t *testing.T) {
			rec := serve(handler, http.MethodGet, target, "gzip")
			if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != "La interfaz no está compilada (correr `make web-build`)" {
				t.Fatalf("status = %d body = %q, want build instructions with 503", rec.Code, rec.Body.String())
			}
			if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") || rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("headers = %v, want plain text and no-store", rec.Header())
			}
		})
	}
}

// T-F007: HEAD has no body; other methods advertise GET and HEAD in Allow.
func TestHandlerMethods(t *testing.T) {
	handler := web.NewHandler(builtFiles())
	for _, method := range []string{http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			target := "/login"
			if method == http.MethodHead {
				target = "/"
			}
			rec := serve(handler, method, target, "")
			if method == http.MethodHead {
				if rec.Code != http.StatusOK || rec.Body.Len() != 0 || rec.Header().Get("Content-Security-Policy") != documentCSP {
					t.Fatalf("HEAD: status = %d headers = %v body = %q", rec.Code, rec.Header(), rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("status = %d Allow = %q, want 405 and GET, HEAD", rec.Code, rec.Header().Get("Allow"))
			}
		})
	}
}

// T-F007: gzip applies to public text resources above gzhttp's default 1 KiB threshold.
func TestHandlerCompression(t *testing.T) {
	files := builtFiles()
	handler := web.NewHandler(files)
	for _, tt := range []struct {
		path, file, encoding string
		compressed           bool
	}{
		{"/", "index.html", "gzip", true},
		{"/settings/users", "index.html", "gzip", true},
		{"/assets/index-abc123.js", "assets/index-abc123.js", "gzip", true},
		{"/assets/index-abc123.css", "assets/index-abc123.css", "gzip", true},
		{"/manifest.webmanifest", "manifest.webmanifest", "gzip", true},
		{"/sw.js", "sw.js", "gzip", true},
		{"/assets/small-abc123.js", "assets/small-abc123.js", "gzip", false},
		{"/icons/icon-192.png", "icons/icon-192.png", "gzip", false},
		{"/favicon.ico", "favicon.ico", "gzip", false},
		{"/", "index.html", "gzip;q=0", false},
		{"/", "index.html", "zstd", false},
	} {
		t.Run(tt.path+"/"+tt.encoding, func(t *testing.T) {
			rec := serve(handler, http.MethodGet, tt.path, tt.encoding)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			body := rec.Body.Bytes()
			if tt.compressed {
				if rec.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
					t.Fatalf("headers = %v, want gzip and Vary: Accept-Encoding", rec.Header())
				}
				reader, err := gzip.NewReader(rec.Body)
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				body, err = io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
			} else if got := rec.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("Content-Encoding = %q, want absent", got)
			}
			if !bytes.Equal(body, files[tt.file].Data) {
				t.Error("response did not preserve the file contents")
			}
		})
	}
}

// T-F008: all:dist embeds hidden markers and exposes the contents at the FS root.
func TestDistFSRoot(t *testing.T) {
	dist := web.DistFS()
	if _, err := fs.ReadFile(dist, ".gitkeep"); err != nil {
		t.Fatalf("embedded dist marker: %v", err)
	}
	if _, err := fs.Stat(dist, "dist"); err == nil {
		t.Fatal("DistFS includes an extra dist directory instead of its contents")
	}
}
