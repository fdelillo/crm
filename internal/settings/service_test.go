package settings_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/google/uuid"
)

type stubRunner struct{}

func (stubRunner) InTenantTx(context.Context, uuid.UUID, func(context.Context, db.Tx) error) error {
	return nil
}
func (stubRunner) InSystemTx(context.Context, db.SystemRole, func(context.Context, db.Tx) error) error {
	return nil
}

// T-002-B205 (DD-1): a missing dependency fails at construction, not on the first request.
func TestNewServicePanicsOnNilDependency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if settings.NewService(stubRunner{}, audit.NewRecorder(), clock.Real{}, logger) == nil {
		t.Fatal("NewService returned nil with valid dependencies")
	}
	for _, tc := range []struct {
		name  string
		build func()
	}{
		{"runner", func() { settings.NewService(nil, audit.NewRecorder(), clock.Real{}, logger) }},
		{"recorder", func() { settings.NewService(stubRunner{}, nil, clock.Real{}, logger) }},
		{"clock", func() { settings.NewService(stubRunner{}, audit.NewRecorder(), nil, logger) }},
		{"logger", func() { settings.NewService(stubRunner{}, audit.NewRecorder(), clock.Real{}, nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("NewService did not panic")
				}
			}()
			tc.build()
		})
	}
}
