//go:build integration

package invariants_test

import (
	"context"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
)

// T-B108, INV-07: every foreign key between two company tables (both have tenant_id) includes
// tenant_id on both sides, so the database itself refuses a reference across companies.
func TestCatalog_ForeignKeysBetweenCompanyTablesAreComposite(t *testing.T) {
	t.Parallel()
	bad := queryStrings(t, pgtest.OwnerPool(t), `
		WITH company AS (
		  SELECT c.oid FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		  WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p')
		    AND EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped)
		)
		SELECT con.conname FROM pg_constraint con
		WHERE con.contype = 'f' AND con.conrelid IN (SELECT oid FROM company) AND con.confrelid IN (SELECT oid FROM company)
		  AND NOT (
		    (SELECT array_agg(a.attname) FROM pg_attribute a WHERE a.attrelid = con.conrelid AND a.attnum = ANY(con.conkey)) @> ARRAY['tenant_id'::name]
		    AND
		    (SELECT array_agg(a.attname) FROM pg_attribute a WHERE a.attrelid = con.confrelid AND a.attnum = ANY(con.confkey)) @> ARRAY['tenant_id'::name]
		  )`)
	if len(bad) > 0 {
		t.Errorf("foreign keys between company tables without tenant_id on both sides: %v", bad)
	}
}

// T-B108: behaviour. As the table owner with FORCE ROW LEVEL SECURITY switched off - inside a
// transaction that always rolls back - a session of company A pointing at a user of company B is
// rejected by the foreign key (23503), for every table that references users.
func TestCatalog_ForeignKeyRefusesAReferenceAcrossCompanies(t *testing.T) {
	// Sequential: it takes ACCESS EXCLUSIVE locks on the tables it alters.
	ctx := context.Background()
	a := fixture.NewCompany(t, pgtest.AppPool(t))
	b := fixture.NewCompany(t, pgtest.AppPool(t))

	cases := []struct{ table, insert string }{
		{"sessions", `INSERT INTO app.sessions (tenant_id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, now() + interval '1 day')`},
		{"user_tokens", `INSERT INTO app.user_tokens (tenant_id, user_id, purpose, token_hash, expires_at) VALUES ($1, $2, 'password_reset', $3, now() + interval '1 hour')`},
		{"audit_log", `INSERT INTO app.audit_log (tenant_id, actor_user_id, action) VALUES ($1, $2, 'tenant.registered')`},
	}
	for _, c := range cases {
		t.Run(c.table, func(t *testing.T) {
			tx, err := pgtest.OwnerPool(t).Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, `ALTER TABLE app.`+c.table+` NO FORCE ROW LEVEL SECURITY`); err != nil {
				t.Fatal(err)
			}
			args := []any{a.ID, b.UserID}
			if c.table != "audit_log" {
				args = append(args, make([]byte, 32))
			}
			if _, err := tx.Exec(ctx, c.insert, args...); sqlState(err) != "23503" {
				t.Errorf("insert of A's row pointing at B's user: err = %v, want SQLSTATE 23503 (foreign_key_violation)", err)
			}
		})
	}
}
