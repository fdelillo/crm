//go:build integration

// Package invariants_test holds the tests that guard the database design of every spec: the
// functions of migration 00002, the catalog (RLS, ownership, privileges), composite foreign keys
// and tenant isolation. It has its own PostgreSQL so no other test's scratch tables interfere.
package invariants_test

import (
	"context"
	"errors"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"os"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// sqlState returns the SQLSTATE of a PostgreSQL error, or "".
func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// asRole runs fn in a transaction of the runtime pool (crm_app) after SET LOCAL ROLE role, and
// always rolls back: it is for reading and for provoking errors.
func asRole(t testing.TB, pool *pgxpool.Pool, role string, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := fixture.SetRole(ctx, tx, role); err != nil {
		t.Fatalf("SET LOCAL ROLE %s: %v", role, err)
	}
	fn(ctx, tx)
}
