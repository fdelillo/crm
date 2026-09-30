//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func migrateCmd(t *testing.T, url string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	getenv := func(k string) string {
		if k == "DATABASE_MIGRATION_URL" {
			return url
		}
		return ""
	}
	err := run(context.Background(), append([]string{"migrate"}, args...), getenv, &out, &out)
	return out.String(), err
}

// `crm migrate` goes down, reports pending, and up again, through the embedded migrations as crm_owner.
// Sequential on purpose: it changes the schema of the package's database.
func TestMigrate_UpDownStatus(t *testing.T) {
	url := pgtest.OwnerURL(t)

	out, err := migrateCmd(t, url, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "applied") || !strings.Contains(out, "00001_schemas.sql") {
		t.Errorf("status after the harness migrated:\n%s", out)
	}

	if out, err = migrateCmd(t, url, "down"); err != nil {
		t.Fatalf("down: %v\n%s", err, out)
	}
	if !strings.Contains(out, "rolled back") {
		t.Errorf("down output: %q", out)
	}
	if out, _ = migrateCmd(t, url, "status"); !strings.Contains(strings.ToLower(out), "pending") {
		t.Errorf("status after down should list a pending migration:\n%s", out)
	}

	if out, err = migrateCmd(t, url, "up"); err != nil {
		t.Fatalf("up: %v\n%s", err, out)
	}
	if !strings.Contains(out, "applied") {
		t.Errorf("up output: %q", out)
	}
	if out, err = migrateCmd(t, url, "up"); err != nil || !strings.Contains(out, "no migrations to apply") {
		t.Errorf("second up: out %q err %v, want a no-op", out, err)
	}
}

// A connection failure must not print the password embedded in the URL.
func TestMigrate_ConnectionErrorDoesNotLeakThePassword(t *testing.T) {
	const password = "leak-canary-password"
	_, err := migrateCmd(t, "postgres://crm_owner:"+password+"@127.0.0.1:1/crm?sslmode=disable&connect_timeout=2", "status")
	if err == nil {
		t.Fatal("status against a closed port succeeded")
	}
	if strings.Contains(err.Error(), password) {
		t.Errorf("error leaks the password: %v", err)
	}
}
