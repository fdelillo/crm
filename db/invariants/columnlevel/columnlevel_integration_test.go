//go:build integration

// Package columnlevel_test proves the isolation sweep covers a table whose UPDATE is granted by column only,
// the shape of the immutable tables of the financial specs (data-model.md §6). It has its own database
// because it creates a scratch table that the catalog tests of ../ would (rightly) complain about.
package columnlevel_test

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/isolation"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func asRoleCommit(t *testing.T, role string, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pgtest.AppPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	fn(ctx, tx)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// The sweep must exercise a table whose UPDATE is granted by column only (the immutable tables of the
// financial specs): the update and move cases run, and a row of B is still out of reach.
func TestIsolation_SweepCoversColumnLevelUpdates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	app := pgtest.AppPool(t)
	owner := pgtest.OwnerPool(t)
	a := fixture.NewCompany(t, app)
	b := fixture.NewCompany(t, app)

	table := fixture.ProbeTable(t, owner) // "app.probe_<hex>"
	name := strings.TrimPrefix(table, "app.")
	if _, err := owner.Exec(ctx, `GRANT UPDATE (note) ON `+table+` TO crm_tenant`); err != nil {
		t.Fatal(err)
	}
	if isolation.HasPrivilege(t, a.Role, name, "UPDATE") {
		t.Fatal("test set-up: the probe grants UPDATE on the whole table, not on a column")
	}
	if !isolation.HasAnyUpdate(t, a.Role, name) {
		t.Fatal("test set-up: hasAnyUpdate does not see the column-level UPDATE")
	}

	insert := func(ctx context.Context, tx pgx.Tx, tenantID, _ uuid.UUID) error {
		_, err := tx.Exec(ctx, `INSERT INTO `+table+` (tenant_id, note) VALUES ($1, 'row')`, tenantID)
		return err
	}
	rowOf := func(c fixture.Company) uuid.UUID {
		var id uuid.UUID
		asRoleCommit(t, c.Role, func(ctx context.Context, tx pgx.Tx) {
			if err := tx.QueryRow(ctx, `INSERT INTO `+table+` (tenant_id, note) VALUES ($1, 'row') RETURNING id`, c.ID).Scan(&id); err != nil {
				t.Fatal(err)
			}
		})
		return id
	}
	ran := isolation.Sweep(t, a, b, name, rowOf(a), rowOf(b), insert)
	for _, want := range []string{"select", "update", "move own row to B", "insert for B"} {
		if !slices.Contains(ran, want) {
			t.Errorf("cases run = %v, missing %q (a column-level UPDATE must not be skipped)", ran, want)
		}
	}
}
