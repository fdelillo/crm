package web

import (
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/klauspost/compress/gzhttp"
)

const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; font-src 'self'; connect-src 'self'; manifest-src 'self'; worker-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

type cacheRule struct {
	exact, prefix, extension string
	value                    string
	uncompressed             bool
}

// Ordered from specific document rules to broader public asset prefixes (plan §10.7).
var cacheRules = []cacheRule{
	{exact: "index.html", value: "no-cache"},
	{exact: "sw.js", value: "no-cache"},
	{exact: "manifest.webmanifest", value: "no-cache"},
	{exact: "offline.html", value: "no-cache"},
	{prefix: "assets/", value: "public, max-age=31536000, immutable"},
	{prefix: "icons/", value: "public, max-age=86400", uncompressed: true},
	{prefix: "favicon.", value: "public, max-age=86400", uncompressed: true},
	{extension: ".html", value: "no-cache"},
}

func cachePolicy(name string) cacheRule {
	for _, rule := range cacheRules {
		if rule.exact == name || rule.prefix != "" && strings.HasPrefix(name, rule.prefix) || rule.extension != "" && path.Ext(name) == rule.extension {
			return rule
		}
	}
	return cacheRule{value: "no-cache"}
}

// NewHandler serves public assets and SPA route fallbacks from dist (ADR-019).
// Without index.html it answers 503 with build instructions. Only GET and HEAD are allowed.
func NewHandler(dist fs.FS) http.Handler {
	index, indexErr := fs.ReadFile(dist, "index.html")
	// gzhttp also enables zstd by default in the pinned version; DD-29 specifies gzip only.
	compress, err := gzhttp.NewWrapper(gzhttp.EnableZstd(false))
	if err != nil {
		panic(err) // All options are constants supported by gzhttp.
	}
	return compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if indexErr != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusServiceUnavailable)
			if r.Method != http.MethodHead {
				_, _ = io.WriteString(w, "La interfaz no está compilada (correr `make web-build`)")
			}
			return
		}

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		data, readErr := fs.ReadFile(dist, name)
		if readErr != nil {
			if path.Ext(name) != "" {
				w.Header().Set("Cache-Control", "no-store")
				http.NotFound(w, r)
				return
			}
			name, data = "index.html", index
		}
		policy := cachePolicy(name)
		w.Header().Set("Cache-Control", policy.value)
		if policy.uncompressed {
			w.Header().Set(gzhttp.HeaderNoCompression, "true")
		}
		if path.Ext(name) == ".html" {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		}
		contentType := mime.TypeByExtension(path.Ext(name))
		switch path.Ext(name) {
		case ".js":
			contentType = "text/javascript; charset=utf-8"
		case ".webmanifest":
			contentType = "application/manifest+json"
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	}))
}
