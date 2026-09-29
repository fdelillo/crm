package app_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/config"
)

// writeCertPair generates a self-signed certificate for 127.0.0.1 and writes the PEM files to
// dir. Nothing is versioned: every test creates its own pair.
func writeCertPair(t *testing.T, dir, name string) (certFile, keyFile string, pool *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "crm test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certFile = filepath.Join(dir, name+".pem")
	keyFile = filepath.Join(dir, name+"-key.pem")
	writePEM(t, certFile, "CERTIFICATE", der)
	writePEM(t, keyFile, "EC PRIVATE KEY", keyDER)
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool = x509.NewCertPool()
	pool.AddCert(leaf)
	return certFile, keyFile, pool
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}

func localConfig(t *testing.T, tlsFiles *config.TLSFiles) config.Config {
	t.Helper()
	u, err := url.Parse("https://localhost:8443")
	if err != nil {
		t.Fatal(err)
	}
	return config.Config{AppBaseURL: u, HTTPAddr: "127.0.0.1:0", TLS: tlsFiles}
}

func healthRoot() http.Handler {
	return app.NewRootHandler(app.RootDeps{
		API:       app.NewAPIRouter(),
		Liveness:  app.LivenessHandler(),
		Readiness: app.ReadinessPlaceholder(),
		SPA:       app.SPAUnavailableHandler(),
	}, app.NewCommonMiddleware(slog.New(slog.NewJSONHandler(io.Discard, nil)), true))
}

// startServer serves srv on an ephemeral port and returns its address and the log buffer. The
// server is shut down (and Serve's result checked) when the test ends.
func startServer(t *testing.T, srv *http.Server) (addr string, logs *syncBuffer) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	logs = &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Serve(ctx, srv, ln, logger) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Serve returned %v after shutdown, want nil", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Serve did not return after the context was cancelled")
		}
	})
	return ln.Addr().String(), logs
}

func logHas(t *testing.T, logs *syncBuffer, key, value string) bool {
	t.Helper()
	for _, l := range logs.lines(t) {
		if l[key] == value {
			return true
		}
	}
	return false
}

func waitForLog(t *testing.T, logs *syncBuffer, key, value string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if logHas(t, logs, key, value) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("log never had %s=%s:\n%s", key, value, logs.String())
}

func TestNewServer_LocalHTTPS(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile, pool := writeCertPair(t, dir, "localhost")
	cfg := localConfig(t, &config.TLSFiles{CertFile: certFile, KeyFile: keyFile})
	srv, err := app.NewServer(cfg, healthRoot())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	addr, logs := startServer(t, srv)
	waitForLog(t, logs, "listen", "https_local")

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		DisableKeepAlives: true, // a lingering pooled connection would delay the graceful shutdown
	}}
	resp, err := client.Get("https://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET over HTTPS: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != `{"status":"ok"}` {
		t.Errorf("status = %d body = %q", resp.StatusCode, body)
	}
	if resp.TLS == nil {
		t.Error("response did not come over TLS")
	}
	// Local mode: no HSTS (DD-24).
	if v := resp.Header.Get("Strict-Transport-Security"); v != "" {
		t.Errorf("HSTS = %q in local mode", v)
	}
}

// net/http reports its own problems (here a client speaking plain HTTP to the TLS listener) through
// the structured logger, not as loose text on stderr.
func TestServe_HTTPServerErrorsGoThroughSlog(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile, _ := writeCertPair(t, dir, "localhost")
	srv, err := app.NewServer(localConfig(t, &config.TLSFiles{CertFile: certFile, KeyFile: keyFile}), healthRoot())
	if err != nil {
		t.Fatal(err)
	}
	addr, logs := startServer(t, srv)
	waitForLog(t, logs, "listen", "https_local")

	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	if resp, err := client.Get("http://" + addr + "/healthz"); err == nil { // plain HTTP against TLS
		resp.Body.Close()
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, l := range logs.lines(t) { // lines() fails the test if any line is not JSON
			if msg, _ := l["msg"].(string); strings.Contains(msg, "TLS handshake error") && l["level"] == "WARN" {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("no structured WARN about the TLS handshake error in:\n%s", logs.String())
}

func TestNewServer_PlainHTTP(t *testing.T) {
	srv, err := app.NewServer(localConfig(t, nil), healthRoot())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	addr, logs := startServer(t, srv)
	waitForLog(t, logs, "listen", "http")

	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET over HTTP: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.TLS != nil {
		t.Errorf("status = %d tls = %v, want 200 over plain HTTP", resp.StatusCode, resp.TLS != nil)
	}
	if logHas(t, logs, "listen", "https_local") {
		t.Error("logged listen=https_local for a plain HTTP server")
	}
}

func TestNewServer_TLSErrorsBeforeListening(t *testing.T) {
	dir := t.TempDir()
	certA, keyA, _ := writeCertPair(t, dir, "a")
	_, keyB, _ := writeCertPair(t, dir, "b")
	const secretMarker = "TOP-SECRET-KEY-CONTENT"
	garbage := filepath.Join(dir, "garbage.pem")
	if err := os.WriteFile(garbage, []byte(secretMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.pem")

	tests := []struct {
		name      string
		files     config.TLSFiles
		wantNames []string
	}{
		{"cert file missing", config.TLSFiles{CertFile: missing, KeyFile: keyA}, []string{"TLS_CERT_FILE"}},
		{"key file missing", config.TLSFiles{CertFile: certA, KeyFile: missing}, []string{"TLS_KEY_FILE"}},
		{"pair does not match", config.TLSFiles{CertFile: certA, KeyFile: keyB}, []string{"TLS_CERT_FILE", "TLS_KEY_FILE"}},
		{"key is not PEM", config.TLSFiles{CertFile: certA, KeyFile: garbage}, []string{"TLS_KEY_FILE"}},
		{"cert is not PEM", config.TLSFiles{CertFile: garbage, KeyFile: keyA}, []string{"TLS_CERT_FILE"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := app.NewServer(localConfig(t, &tt.files), healthRoot())
			if err == nil {
				t.Fatalf("NewServer returned no error (server %v)", srv)
			}
			if srv != nil {
				t.Error("NewServer returned a server together with an error")
			}
			for _, n := range tt.wantNames {
				if !strings.Contains(err.Error(), n) {
					t.Errorf("error %q does not name %s", err, n)
				}
			}
			if strings.Contains(err.Error(), secretMarker) {
				t.Errorf("error %q leaks file content", err)
			}
		})
	}
}

func TestNewServer_Timeouts(t *testing.T) {
	srv, err := app.NewServer(localConfig(t, nil), healthRoot())
	if err != nil {
		t.Fatal(err)
	}
	if srv.ReadHeaderTimeout != 5*time.Second || srv.ReadTimeout != 15*time.Second || srv.WriteTimeout != 30*time.Second {
		t.Errorf("timeouts = header %v read %v write %v, want 5s/15s/30s (plan §10.3)",
			srv.ReadHeaderTimeout, srv.ReadTimeout, srv.WriteTimeout)
	}
	if srv.IdleTimeout <= 0 {
		t.Error("IdleTimeout must be set")
	}
	if srv.Handler == nil {
		t.Error("Handler must be the root handler")
	}
}

// Cancelling the context (SIGTERM in production) lets the request in flight finish (T-B004 Green).
func TestServe_GracefulShutdown(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	slow := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "done")
	})
	srv, err := app.NewServer(localConfig(t, nil), slow)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- app.Serve(ctx, srv, ln, slog.New(slog.NewJSONHandler(io.Discard, nil))) }()

	type result struct {
		body string
		err  error
	}
	got := make(chan result, 1)
	go func() {
		client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
		resp, err := client.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			got <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		got <- result{body: string(b)}
	}()

	<-started
	cancel()
	select {
	case err := <-served:
		t.Fatalf("Serve returned (%v) while a request was still in flight", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)

	if r := <-got; r.err != nil || r.body != "done" {
		t.Errorf("in-flight request: body %q err %v, want it to complete", r.body, r.err)
	}
	if err := <-served; err != nil {
		t.Errorf("Serve = %v, want nil after a graceful shutdown", err)
	}
}

func TestSPAUnavailableHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	app.SPAUnavailableHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if !strings.Contains(rec.Body.String(), "La interfaz no está compilada (correr `make web-build`)") {
		t.Errorf("body = %q", rec.Body.String())
	}
}
