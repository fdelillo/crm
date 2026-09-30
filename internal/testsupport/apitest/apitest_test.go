package apitest_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/apitest"
)

// The harness serves the root handler over HTTPS and its client keeps cookies, so a
// `__Host-` + Secure cookie set by one request is sent back on the next, as in production
// (ADR-012 note, H-10).
func TestNewServer_ClientReplaysSecureHostCookie(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /set", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name: "__Host-x", Value: "1", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /read", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("__Host-x")
		if err != nil {
			http.Error(w, "no cookie", http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, c.Value)
	})

	srv := apitest.NewServer(t, mux)
	if !strings.HasPrefix(srv.URL, "https://") {
		t.Fatalf("URL = %q, want https://", srv.URL)
	}

	// Before the cookie exists the second endpoint refuses.
	resp, err := srv.Client.Get(srv.URL + "/read")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /read without cookie: status = %d, want 401", resp.StatusCode)
	}

	resp, err = srv.Client.Get(srv.URL + "/set")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("GET /set: status = %d", resp.StatusCode)
	}

	resp, err = srv.Client.Get(srv.URL + "/read")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "1" {
		t.Errorf("GET /read after /set: status = %d body = %q, want 200 \"1\"", resp.StatusCode, body)
	}
}

func TestNewServer_TrustsItsOwnCertificate(t *testing.T) {
	srv := apitest.NewServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			http.Error(w, "plain", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	resp, err := srv.Client.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v (client must trust the test certificate)", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 over TLS", resp.StatusCode)
	}
}
