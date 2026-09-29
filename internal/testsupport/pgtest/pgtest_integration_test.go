//go:build integration

package pgtest_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func TestAppPool_ConnectsAsCrmApp(t *testing.T) {
	t.Parallel()
	pgtest.Start(t)
	var user, session string
	err := pgtest.AppPool(t).QueryRow(context.Background(), `SELECT current_user, session_user`).Scan(&user, &session)
	if err != nil {
		t.Fatal(err)
	}
	if user != "crm_app" || session != "crm_app" {
		t.Errorf("current_user = %q, session_user = %q, want crm_app for both (INV-18)", user, session)
	}
}

// INV-18: the pool that tests use is never a superuser and never bypasses RLS.
func TestAppPool_RoleAttributes(t *testing.T) {
	t.Parallel()
	var super, bypass bool
	err := pgtest.AppPool(t).QueryRow(context.Background(),
		`SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&super, &bypass)
	if err != nil {
		t.Fatal(err)
	}
	if super || bypass {
		t.Errorf("rolsuper = %v, rolbypassrls = %v, want both false", super, bypass)
	}
}

// INV-02: a query outside a role switch fails closed. There are no company tables until Phase 1,
// so the test creates its own as crm_owner (which gets the default privileges of migration 00001)
// and checks crm_app cannot touch it.
func TestAppPool_NoDirectAccessToAppTables(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	table := "app.pgtest_probe_" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")
	owner := pgtest.OwnerPool(t)
	if _, err := owner.Exec(ctx, `CREATE TABLE `+table+` (id int)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DROP TABLE IF EXISTS `+table) })

	_, err := pgtest.AppPool(t).Exec(ctx, `SELECT 1 FROM `+table)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("SELECT as crm_app on %s: err = %v, want SQLSTATE 42501", table, err)
	}
}

func TestOwnerPool_ConnectsAsCrmOwner(t *testing.T) {
	t.Parallel()
	var user string
	if err := pgtest.OwnerPool(t).QueryRow(context.Background(), `SELECT current_user`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	if user != "crm_owner" {
		t.Errorf("current_user = %q, want crm_owner", user)
	}
}

// The harness runs the migrations as crm_owner: the goose table exists in public and schemas exist.
func TestStart_RanMigrationsAsOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	owner := pgtest.OwnerPool(t)
	var version int64
	if err := owner.QueryRow(ctx, `SELECT max(version_id) FROM public.goose_db_version`).Scan(&version); err != nil {
		t.Fatalf("goose version table: %v", err)
	}
	if version < 1 {
		t.Errorf("migration version = %d, want at least 1", version)
	}
	var n int
	if err := owner.QueryRow(ctx,
		`SELECT count(*) FROM pg_namespace WHERE nspname IN ('app', 'provisioning')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("schemas app/provisioning present = %d, want 2", n)
	}
}

// The superuser pool exists only to prepare fixtures that need privileges; it is not crm_app.
func TestSuperuserPool_IsSuperuser(t *testing.T) {
	t.Parallel()
	var super bool
	if err := pgtest.SuperuserPool(t).QueryRow(context.Background(),
		`SELECT rolsuper FROM pg_roles WHERE rolname = current_user`).Scan(&super); err != nil {
		t.Fatal(err)
	}
	if !super {
		t.Error("SuperuserPool is not a superuser")
	}
}
