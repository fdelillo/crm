package app

import (
	"context"
	"errors"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const ReadinessTimeout = time.Second

// ReadinessHandler bounds both pool acquisition and the schema read. Versions stay in logs only.
func ReadinessHandler(runner db.TxRunner, expected int64, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), ReadinessTimeout)
		defer cancel()
		var version int64
		var known bool
		err := runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
			var err error
			version, known, err = db.SchemaVersion(ctx, tx)
			return err
		})
		if r.Context().Err() != nil {
			httpx.WriteDBError(w, r, db.ErrCanceled, logger)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		reason := ""
		level := slog.LevelWarn
		fields := []any{"event", "readiness_failed", "request_id", httpx.RequestIDFrom(r.Context())}
		switch {
		case errors.Is(err, db.ErrPrivilege):
			reason = "privilege"
			level = slog.LevelError
			fields = append(fields, "security_event", "rls_violation", "err", err)
		case ctx.Err() != nil:
			reason = "timeout"
		case err != nil:
			reason = "unavailable"
			fields = append(fields, "err", err)
		case !known:
			reason = "schema_unknown"
		case version != expected:
			reason = "schema_mismatch"
			fields = append(fields, "db_version", version, "expected_version", expected)
		}
		if reason != "" {
			httpx.SetLogLevel(r, level)
			logger.Log(r.Context(), level, "readiness failed", append(fields, "reason", reason)...)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"status":"unavailable"}`)
			return
		}
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
}

// LogServerVersion records the PostgreSQL version once at startup, with a bounded auth transaction.
func LogServerVersion(ctx context.Context, runner db.TxRunner, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(ctx, ReadinessTimeout)
	defer cancel()
	var version string
	err := runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, "SELECT current_setting('server_version')").Scan(&version)
	})
	if err == nil {
		logger.Info("database connected", "server_version", version)
	} else if ctx.Err() == nil {
		logger.Warn("database version unavailable", "err", err)
	}
}
