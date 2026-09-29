package apitest

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
)

// Server is a test HTTPS server over the real root handler.
type Server struct {
	URL    string       // https://127.0.0.1:<port>
	Client *http.Client // trusts the test certificate and keeps cookies
}

// NewServer serves root over HTTPS (httptest.NewTLSServer) and returns a client that trusts the
// test certificate and has a cookie jar. Session cookies are always Secure (INV-23), so tests that
// chain requests with the cookie (signup → /me, login → logout) must go through HTTPS as in
// production; no test needs mkcert. Note the jar does not enforce the __Host- prefix rules, so
// tests assert the cookie attributes on Set-Cookie explicitly. The server closes with the test.
func NewServer(t testing.TB, root http.Handler) *Server {
	t.Helper()
	srv := httptest.NewTLSServer(root)
	t.Cleanup(srv.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("apitest: creating cookie jar: %v", err)
	}
	client := srv.Client() // already trusts srv's certificate
	client.Jar = jar
	return &Server{URL: srv.URL, Client: client}
}
