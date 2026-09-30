// Package fixture creates the data that integration tests need. It talks to PostgreSQL with plain
// pgx (no platform/db), so it can also be used to test platform/db itself.
package fixture

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RoleName is the PostgreSQL role of a company: "crm_t_" + the 32 hex digits of its UUID (ADR-005).
func RoleName(id uuid.UUID) string { return "crm_t_" + hex.EncodeToString(id[:]) }

// ProvisionRole creates the role of company id exactly as registration does: as crm_app, switch to
// crm_signup and call provisioning.provision_tenant_role, then commit. It returns the role name.
func ProvisionRole(t testing.TB, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("fixture: begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE crm_signup`); err != nil {
		t.Fatalf("fixture: SET LOCAL ROLE crm_signup: %v", err)
	}
	var role string
	if err := tx.QueryRow(ctx, `SELECT provisioning.provision_tenant_role($1)`, id).Scan(&role); err != nil {
		t.Fatalf("fixture: provisioning company role: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("fixture: commit: %v", err)
	}
	return role
}

// ProbeTable creates a scratch company table in schema app, as a migration would (owned by
// crm_owner, RLS forced, policy tenant_isolation on app.current_tenant_id()), and drops it when the
// test ends. Columns: id uuid, tenant_id uuid, note text. Use it to prove behaviour that needs real
// rows without depending on the tables of a later phase. It returns the qualified table name.
func ProbeTable(t testing.TB, owner *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	table := "app.probe_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	for _, q := range []string{
		`CREATE TABLE ` + table + ` (id uuid NOT NULL DEFAULT uuidv7(), tenant_id uuid NOT NULL, note text NOT NULL)`,
		`ALTER TABLE ` + table + ` ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE ` + table + ` FORCE ROW LEVEL SECURITY`,
		`CREATE POLICY tenant_isolation ON ` + table + ` FOR ALL TO PUBLIC
		   USING (tenant_id = (SELECT app.current_tenant_id()))
		   WITH CHECK (tenant_id = (SELECT app.current_tenant_id()))`,
	} {
		if _, err := owner.Exec(ctx, q); err != nil {
			t.Fatalf("fixture: %s: %v", q, err)
		}
	}
	t.Cleanup(func() { _, _ = owner.Exec(context.Background(), `DROP TABLE IF EXISTS `+table) })
	return table
}
