package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// safeBuffer is a bytes.Buffer safe for concurrent use: the server logs from its own goroutines.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func mapEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":  "postgres://crm_app:x@localhost:5432/crm",
		"AUTH_HMAC_KEY": "a2tra2tra2tra2tra2tra2tra2tra2tra2tra2tra2s=", // 32 bytes of 'k'
		"APP_BASE_URL":  "https://localhost:8443",
		"HTTP_ADDR":     "127.0.0.1:0",
	}
}

func TestRun_Usage(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no command", nil},
		{"unknown command", []string{"frobnicate"}},
		{"migrate without direction", []string{"migrate"}},
		{"migrate unknown direction", []string{"migrate", "sideways"}},
		{"tenants without subcommand", []string{"tenants"}},
		{"tenants unknown subcommand", []string{"tenants", "nope"}},
		{"serve with stray arguments", []string{"serve", "extra"}},
		{"serve with unknown flag", []string{"serve", "-nope"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			err := run(context.Background(), tt.args, mapEnv(validEnv()), io.Discard, &stderr)
			var ue *usageError
			if !errors.As(err, &ue) {
				t.Fatalf("run(%v) = %v, want a *usageError", tt.args, err)
			}
		})
	}
}

func TestRun_ServeRequiresConfiguration(t *testing.T) {
	env := validEnv()
	delete(env, "DATABASE_URL")
	err := run(context.Background(), []string{"serve"}, mapEnv(env), io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("run(serve) = %v, want an error naming DATABASE_URL", err)
	}
}

func TestRun_MigrateRequiresMigrationURL(t *testing.T) {
	err := run(context.Background(), []string{"migrate", "status"}, mapEnv(validEnv()), io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_MIGRATION_URL") {
		t.Fatalf("run(migrate status) = %v, want an error naming DATABASE_MIGRATION_URL", err)
	}
}

// reprovision-roles is only a placeholder until Phase 9 (T-B906).
func TestRun_ReprovisionRolesIsNotImplementedYet(t *testing.T) {
	err := run(context.Background(), []string{"tenants", "reprovision-roles"}, mapEnv(validEnv()), io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("run(tenants reprovision-roles) = %v, want a \"not implemented\" error", err)
	}
	var ue *usageError
	if errors.As(err, &ue) {
		t.Error("a not-implemented command is not a usage error")
	}
}

// `crm serve` starts, answers /healthz and shuts down cleanly when its context ends.
func TestRun_ServeAnswersHealthzAndStopsOnCancel(t *testing.T) {
	stdout := &safeBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, []string{"serve"}, mapEnv(validEnv()), stdout, io.Discard) }()

	addr := waitForListenAddr(t, stdout, done)
	// No keep-alive: an idle or half-opened pooled connection would delay the graceful shutdown
	// (net/http waits up to 5 s for a connection that never sent a request).
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != `{"status":"ok"}` {
		t.Errorf("/healthz: status %d body %q", resp.StatusCode, body)
	}

	resp, err = client.Get("http://" + addr + "/api/v1/no-existe")
	if err != nil {
		t.Fatalf("GET /api/v1/no-existe: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound || resp.Header.Get("Content-Type") != "application/problem+json" {
		t.Errorf("/api/v1/no-existe: status %d content-type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	resp, err = client.Get("http://" + addr + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("/login without a built SPA: status %d, want 503", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run(serve) = %v after cancel, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not stop after its context was cancelled")
	}
}

// waitForListenAddr reads the "server listening" log line to learn the ephemeral port.
func waitForListenAddr(t *testing.T, out *safeBuffer, done <-chan error) string {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		for _, l := range strings.Split(out.String(), "\n") {
			var m map[string]any
			if json.Unmarshal([]byte(l), &m) == nil && m["msg"] == "server listening" {
				if m["listen"] != "http" {
					t.Fatalf("listen = %v, want http (no TLS_* configured)", m["listen"])
				}
				return m["addr"].(string)
			}
		}
		select {
		case err := <-done:
			t.Fatalf("serve exited early: %v\nlogs: %s", err, out.String())
		case <-deadline:
			t.Fatalf("serve never logged its address:\n%s", out.String())
		case <-time.After(10 * time.Millisecond):
		}
	}
}
