package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"

	"github.com/fdelillo/crm/db/migrations"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver for goose
	"github.com/pressly/goose/v3"
)

// runMigrate applies the embedded migrations as crm_owner (DATABASE_MIGRATION_URL), never as the
// runtime role crm_app (ADR-004). It is a deployment step: `serve` does not migrate.
func runMigrate(ctx context.Context, args []string, e env) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	if err := fs.Parse(args); err != nil {
		return usagef("migrate: %v", err)
	}
	if fs.NArg() != 1 {
		return usagef("migrate: expected exactly one of up, down, status")
	}
	direction := fs.Arg(0)
	if direction != "up" && direction != "down" && direction != "status" {
		return usagef("migrate: unknown direction %q (want up, down or status)", direction)
	}

	// Read directly: only this command needs the owner URL, so it is not part of the serve
	// configuration. The value is a secret and is never printed.
	url := e.getenv("DATABASE_MIGRATION_URL")
	if url == "" {
		return errors.New("migrate: DATABASE_MIGRATION_URL is required")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return errors.New("migrate: DATABASE_MIGRATION_URL is not a valid connection string")
	}
	defer func() { _ = db.Close() }() // the work is done by now: a close error changes nothing

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("migrate: creating provider: %w", err)
	}
	switch direction {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			return fmt.Errorf("migrate up: %w", err)
		}
		if len(results) == 0 {
			fmt.Fprintln(e.stdout, "no migrations to apply")
		}
		for _, r := range results {
			fmt.Fprintf(e.stdout, "applied %s (%s)\n", r.Source.Path, r.Duration)
		}
	case "down":
		r, err := provider.Down(ctx)
		if err != nil {
			return fmt.Errorf("migrate down: %w", err)
		}
		fmt.Fprintf(e.stdout, "rolled back %s (%s)\n", r.Source.Path, r.Duration)
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return fmt.Errorf("migrate status: %w", err)
		}
		for _, s := range statuses {
			fmt.Fprintf(e.stdout, "%-10s %s\n", s.State, s.Source.Path)
		}
	}
	return nil
}
