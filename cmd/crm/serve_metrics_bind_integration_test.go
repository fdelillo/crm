//go:build integration

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/testsupport/pgtest"
)

// failingAccept fails Accept and reports when http.Server.Serve has closed it. Serve closes the
// listener only on its way out, after it has decided which error to return, so a closed listener
// means the Accept failure is already the API server's result.
type failingAccept struct {
	net.Listener
	fault  error
	once   sync.Once
	closed chan struct{}
}

func (ln *failingAccept) Accept() (net.Conn, error) { return nil, ln.fault }

func (ln *failingAccept) Close() error {
	defer ln.once.Do(func() { close(ln.closed) })
	return ln.Listener.Close()
}

// T-B903, T-B904: a failure to bind the metrics listener names METRICS_ADDR even when the API server also fails.
func TestRunServeMetricsBindErrorNamesAddressDespiteAPIFailure(t *testing.T) {
	values := validEnv()
	values["DATABASE_URL"] = pgtest.AppURL(t)
	testCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	acceptErr, bindErr := errors.New("API Accept failed"), errors.New("metrics bind failed")
	closed := make(chan struct{})
	calls := 0
	e := env{getenv: mapEnv(values), stdout: io.Discard, stderr: io.Discard, listen: func(_ context.Context, network, address string) (net.Listener, error) {
		calls++
		if calls == 1 {
			ln, err := (&net.ListenConfig{}).Listen(testCtx, network, "127.0.0.1:0")
			if err != nil {
				return nil, err
			}
			return &failingAccept{Listener: ln, fault: acceptErr, closed: closed}, nil
		}
		// Fail the bind only once Serve has finished with the API listener, and wait on the test's own
		// deadline rather than on the context runServe hands to listen. Failing earlier races: the
		// bind failure cancels the servers, and Serve then reports ErrServerClosed instead of the
		// Accept error, which drops it from the joined error. That context is also cancelled by the
		// API failure itself, so selecting on it could return "context canceled" instead of bindErr.
		select {
		case <-closed:
			return nil, bindErr
		case <-testCtx.Done():
			return nil, testCtx.Err()
		}
	}}
	err := runServe(testCtx, nil, e)
	if err == nil || !strings.Contains(err.Error(), "METRICS_ADDR") || !errors.Is(err, bindErr) || !errors.Is(err, acceptErr) {
		t.Fatalf("metrics bind context lost: %v", err)
	}
}
