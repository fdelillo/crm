package httpx

import "net/http"

// internal/platform/httpx is the one place that reads the proxy headers.
func forwardedFor(r *http.Request) string { return r.Header.Get("X-Forwarded-For") }
