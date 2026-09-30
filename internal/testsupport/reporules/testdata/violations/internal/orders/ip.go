package orders

import "net/http"

// Only httpx.ClientIP may read the proxy headers (INV-25, DD-32).
func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return v
	}
	if v := r.Header.Get("x-real-ip"); v != "" {
		return v
	}
	return r.Header.Get("Forwarded")
}
