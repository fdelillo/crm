//go:build integration

package sqlcspike_test

import (
	"context"
	"os"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// schema.sql is only useful as a spike if it is valid PostgreSQL: sqlc accepting it proves nothing
// about the syntax being real. Apply it as crm_owner inside a transaction that is rolled back and
// check the column-level GRANT and the forced RLS mean what the design says.
func TestSpikeSchemaIsValidPostgreSQL(t *testing.T) {
	ctx := context.Background()
	ddl, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pgtest.OwnerPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(ddl)); err != nil {
		t.Fatalf("schema.sql is not valid PostgreSQL: %v", err)
	}

	checks := []struct {
		query string
		args  []any
		want  bool
	}{
		{`SELECT has_column_privilege('crm_auth', 'app.spike_items', 'email', 'SELECT')`, nil, true},
		{`SELECT has_column_privilege('crm_auth', 'app.spike_items', 'secret_hash', 'SELECT')`, nil, false},
		{`SELECT has_table_privilege('crm_auth', 'app.spike_items', 'SELECT')`, nil, false},
		{`SELECT has_column_privilege('crm_tenant', 'app.spike_items', 'secret_hash', 'UPDATE')`, nil, false},
		{`SELECT has_column_privilege('crm_tenant', 'app.spike_items', 'email', 'UPDATE')`, nil, true},
		{`SELECT has_column_privilege('crm_tenant', 'app.spike_items', 'ip', 'UPDATE')`, nil, false},
		{`SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid = 'app.spike_items'::regclass`, nil, true},
		{`SELECT count(*) = 2 FROM pg_policies WHERE schemaname = 'app' AND tablename = 'spike_items'`, nil, true},
	}
	for _, c := range checks {
		var got bool
		if err := tx.QueryRow(ctx, c.query, c.args...).Scan(&got); err != nil {
			t.Fatalf("%s: %v", c.query, err)
		}
		if got != c.want {
			t.Errorf("%s = %v, want %v", c.query, got, c.want)
		}
	}
}
