//go:build integration

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/testsupport/pgtest"
)

type failingAccept struct {
	net.Listener
	fault   error
	entered chan struct{}
}

func (ln *failingAccept) Accept() (net.Conn, error) { close(ln.entered); return nil, ln.fault }

func TestPhase9MetricsBindNamesAddressDespiteAPIFailure(t *testing.T) {
	values := validEnv()
	values["DATABASE_URL"] = pgtest.AppURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	acceptErr, bindErr := errors.New("API Accept failed"), errors.New("metrics bind failed")
	entered := make(chan struct{})
	calls := 0
	e := env{getenv: mapEnv(values), stdout: io.Discard, stderr: io.Discard, listen: func(ctx context.Context, network, address string) (net.Listener, error) {
		calls++
		if calls == 1 {
			ln, err := (&net.ListenConfig{}).Listen(ctx, network, "127.0.0.1:0")
			if err != nil {
				return nil, err
			}
			return &failingAccept{Listener: ln, fault: acceptErr, entered: entered}, nil
		}
		select {
		case <-entered:
			return nil, bindErr
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	err := runServe(ctx, nil, e)
	if err == nil || !strings.Contains(err.Error(), "METRICS_ADDR") || !errors.Is(err, bindErr) || !errors.Is(err, acceptErr) {
		t.Fatalf("metrics bind context lost: %v", err)
	}
}
