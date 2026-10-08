package main

import (
	"context"
	"encoding/json"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/config"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"time"
)

func runTenants(ctx context.Context, args []string, e env) error {
	if len(args) == 0 {
		return usagef("tenants: missing subcommand (reprovision-roles)")
	}
	if args[0] != "reprovision-roles" {
		return usagef("tenants: unknown subcommand %q", args[0])
	}
	if len(args) != 1 {
		return usagef("tenants reprovision-roles: unexpected argument %q", args[1])
	}
	cfg, err := config.Load(e.getenv)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger := slog.New(slog.NewJSONHandler(e.stdout, nil))
	runner := db.NewTxRunner(pool, db.WithLogger(logger))
	c := clock.Real{}
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour)
	companies := tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, password.NewHasher(4), audit.NewRecorder(), logger)
	report, err := companies.Reprovision(ctx)
	return finishReprovision(e.stdout, report, err)
}
func finishReprovision(output io.Writer, report tenant.ReprovisionReport, err error) error {
	if encodeErr := json.NewEncoder(output).Encode(report); encodeErr != nil {
		return encodeErr
	}
	return err
}
