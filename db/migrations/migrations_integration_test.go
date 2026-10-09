//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/fdelillo/crm/db/migrations"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func queryBool(t *testing.T, pool *pgxpool.Pool, query string, args ...any) bool {
	t.Helper()
	var v bool
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return v
}

func schemaOwner(t *testing.T, pool *pgxpool.Pool, schema string) string {
	t.Helper()
	var owner string
	err := pool.QueryRow(context.Background(),
		`SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = $1`, schema).Scan(&owner)
	if err != nil {
		t.Fatalf("schema %s: %v", schema, err)
	}
	return owner
}

// Migration 00001 (data-model.md §3.4, §3.5, T-B007).
func TestMigration00001_Schemas(t *testing.T) {
	t.Parallel()
	pool := pgtest.OwnerPool(t)

	if got := schemaOwner(t, pool, "app"); got != "crm_owner" {
		t.Errorf("schema app owner = %q, want crm_owner", got)
	}
	if got := schemaOwner(t, pool, "provisioning"); got != "crm_provisioner" {
		t.Errorf("schema provisioning owner = %q, want crm_provisioner", got)
	}

	usage := []struct {
		role, schema string
		want         bool
	}{
		{"crm_tenant", "app", true},
		{"crm_auth", "app", true},
		{"crm_worker", "app", true},
		{"crm_signup", "provisioning", true},
		{"crm_signup", "app", false},
		{"crm_auth", "provisioning", false},
		{"crm_app", "app", false},          // INV-02: no privileges of its own
		{"crm_app", "provisioning", false}, // reaches it only through crm_signup
		{"crm_app", "public", false},
	}
	for _, u := range usage {
		got := queryBool(t, pool, `SELECT has_schema_privilege($1, $2, 'USAGE')`, u.role, u.schema)
		if got != u.want {
			t.Errorf("USAGE on %s for %s = %v, want %v", u.schema, u.role, got, u.want)
		}
	}

	var publicGrants int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM pg_namespace n, aclexplode(n.nspacl) a
		WHERE n.nspname = 'public' AND a.grantee = 0`).Scan(&publicGrants)
	if err != nil {
		t.Fatal(err)
	}
	if publicGrants != 0 {
		t.Errorf("PUBLIC still has %d privileges on schema public", publicGrants)
	}
}

// The default privileges of 00001: a new table in app is SELECT/INSERT for the company group and
// nothing else (UPDATE/DELETE are granted per table); a new function is not executable by PUBLIC.
func TestMigration00001_DefaultPrivileges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := pgtest.OwnerPool(t)

	if _, err := pool.Exec(ctx, `CREATE TABLE app.default_privs_probe (id int)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS app.default_privs_probe`) })
	if _, err := pool.Exec(ctx, `CREATE FUNCTION app.default_privs_probe() RETURNS int LANGUAGE sql AS 'SELECT 1'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DROP FUNCTION IF EXISTS app.default_privs_probe()`) })

	for priv, want := range map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": false, "DELETE": false, "TRUNCATE": false} {
		if got := queryBool(t, pool, `SELECT has_table_privilege('crm_tenant', 'app.default_privs_probe', $1)`, priv); got != want {
			t.Errorf("crm_tenant %s on a new table = %v, want %v", priv, got, want)
		}
	}
	for _, role := range []string{"crm_app", "crm_auth", "crm_worker", "crm_signup"} {
		if queryBool(t, pool, `SELECT has_table_privilege($1, 'app.default_privs_probe', 'SELECT')`, role) {
			t.Errorf("%s can SELECT a new table by default", role)
		}
	}
	var publicExecute int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_proc p, aclexplode(coalesce(p.proacl, acldefault('f', p.proowner))) a
		WHERE p.oid = 'app.default_privs_probe()'::regprocedure AND a.grantee = 0 AND a.privilege_type = 'EXECUTE'`).
		Scan(&publicExecute)
	if err != nil {
		t.Fatal(err)
	}
	if publicExecute != 0 {
		t.Error("PUBLIC can EXECUTE a new function by default")
	}
}

// Every migration has a working Down: going all the way down and up again gives the same result.
// It is sequential on purpose (parallel tests resume after the sequential ones finish).
func TestMigrations_DownAndUpRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("pgx", pgtest.OwnerURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	pool := pgtest.OwnerPool(t)

	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("down to 0: %v", err)
	}
	if queryBool(t, pool, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname IN ('app', 'provisioning'))`) {
		t.Error("schemas app/provisioning still exist after going down to version 0")
	}
	if !queryBool(t, pool, `SELECT has_schema_privilege('crm_owner', 'public', 'USAGE')`) {
		t.Error("crm_owner lost USAGE on public")
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if !queryBool(t, pool, `SELECT count(*) = 2 FROM pg_namespace WHERE nspname IN ('app', 'provisioning')`) {
		t.Error("schemas app/provisioning missing after going up again")
	}
	if got := schemaOwner(t, pool, "provisioning"); got != "crm_provisioner" {
		t.Errorf("provisioning owner after up again = %q", got)
	}
}

// The version table lives in public, owned by crm_owner, unreachable for the runtime roles.
func TestMigrations_VersionTableInPublic(t *testing.T) {
	t.Parallel()
	pool := pgtest.OwnerPool(t)
	var owner string
	err := pool.QueryRow(context.Background(),
		`SELECT tableowner FROM pg_tables WHERE schemaname = 'public' AND tablename = 'goose_db_version'`).Scan(&owner)
	if err != nil {
		t.Fatalf("goose_db_version not in public: %v", err)
	}
	if owner != "crm_owner" {
		t.Errorf("goose_db_version owner = %q, want crm_owner", owner)
	}
	if queryBool(t, pool, `SELECT has_table_privilege('crm_app', 'public.goose_db_version', 'SELECT')`) {
		t.Error("crm_app can read the goose version table")
	}
}

// The ninth migration is applied and reverted as the actual owner login, without a privileged helper.
func TestMigration00009OwnerAndDown(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.OwnerPool(t)
	var user string
	if err := pool.QueryRow(ctx, "SELECT current_user").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if user != "crm_owner" || schemaOwner(t, pool, "public") != "pg_database_owner" {
		t.Fatalf("owner=%s public=%s", user, schemaOwner(t, pool, "public"))
	}
	migrationDB, err := sql.Open("pgx", pgtest.OwnerURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = migrationDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, migrationDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	assert := func(want bool) {
		t.Helper()
		for _, query := range []string{"SELECT has_schema_privilege('crm_auth','public','USAGE')", "SELECT has_column_privilege('crm_auth','public.goose_db_version','version_id','SELECT')"} {
			if got := queryBool(t, pool, query); got != want {
				t.Errorf("%s=%v want=%v", query, got, want)
			}
		}
	}
	assert(true)
	if _, err = provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := provider.Up(context.Background()); err != nil {
			t.Error(err)
		}
	})
	assert(false)
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	assert(true)
}
