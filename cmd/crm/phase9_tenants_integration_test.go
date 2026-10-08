//go:build integration

package main

import (
	"bytes"
	"context"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5"
	"io"
	"strings"
	"testing"
)

func TestPhase9ReprovisionCommand(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.AppPool(t)
	company := fixture.NewCompany(t, pool)
	probe, err := pgx.ConnectConfig(ctx, pgtest.SuperuserPool(t).Config().ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close(ctx)
	if _, err = probe.Exec(ctx, "DROP ROLE "+pgx.Identifier{company.Role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	cfg := validEnv()
	cfg["DATABASE_URL"] = pgtest.AppURL(t)
	var output bytes.Buffer
	if err := run(ctx, []string{"tenants", "reprovision-roles"}, mapEnv(cfg), &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := probe.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", company.Role).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists || !strings.Contains(output.String(), `"failed":[]`) {
		t.Errorf("role=%v report=%s", exists, &output)
	}
	locker := db.NewTxRunner(pool).(db.SessionLocker)
	_, err = locker.WithSessionLock(ctx, 0x43524d525052, func(ctx context.Context) error {
		output.Reset()
		runErr := run(ctx, []string{"tenants", "reprovision-roles"}, mapEnv(cfg), &output, io.Discard)
		if runErr == nil || !strings.Contains(runErr.Error(), "otra reprovisión en curso") {
			t.Errorf("locked command err=%v", runErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
