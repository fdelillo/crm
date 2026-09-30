package orders

import "net/http"

// Other headers, and messages that only mention the word, are fine.
func other(r *http.Request) string {
	_ = "the request was forwarded by the load balancer"
	return r.Header.Get("X-Request-Id")
}
