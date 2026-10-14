//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/config"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type sampleRunner struct {
	db.TxRunner
	calls int
	fail  bool
}

func (r *sampleRunner) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	r.calls++
	if role != db.RoleWorker {
		return errors.New("sample must use crm_worker")
	}
	if r.fail {
		return db.ErrUnavailable
	}
	return r.TxRunner.InSystemTx(ctx, role, fn)
}

type sampleClock struct{ now time.Time }

func (c *sampleClock) Now() time.Time { return c.now }

func TestPhase9SampledGauges(t *testing.T) {
	ctx := context.Background()
	f := e2e.New(t, pgtest.AppPool(t))
	runner := &sampleRunner{TxRunner: f.Runner}
	clock := &sampleClock{time.Now().UTC()}
	stats := outbox.NewStats(runner, clock)
	roles := tenant.NewRoleInventory(runner)
	if stats.Every() != 60*time.Second || roles.Every() != 60*time.Second {
		t.Fatal("sampling period")
	}
	scrape := func() map[string]float64 {
		t.Helper()
		before := runner.calls
		started := time.Now()
		rec := httptest.NewRecorder()
		app.NewMetricsServer(config.Config{}).Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/debug/vars", nil))
		if time.Since(started) >= time.Second || rec.Code != 200 {
			t.Error("slow or failed scrape")
		}
		if runner.calls != before {
			t.Error("scrape queried database")
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		values := map[string]float64{}
		for _, name := range strings.Fields("outbox_pending outbox_oldest_pending_seconds tenants_total tenant_roles_total tenant_roles_missing") {
			var value float64
			if err := json.Unmarshal(raw[name], &value); err != nil {
				t.Fatal(name, err)
			}
			values[name] = value
		}
		return values
	}
	for name, value := range scrape() {
		if value != -1 {
			t.Errorf("before sample %s=%v", name, value)
		}
	}
	super := pgtest.SuperuserPool(t)
	// No pending messages from earlier tests: this package's phase tests are sequential.
	if _, err := super.Exec(ctx, "UPDATE app.outbox_messages SET next_attempt_at=now()+interval '1 day' WHERE status='pending'"); err != nil {
		t.Fatal(err)
	}
	oldest := clock.now.Add(-2 * time.Hour)
	if _, err := super.Exec(ctx, "UPDATE app.outbox_messages SET created_at=$2 WHERE tenant_id=$1 AND status='pending'", f.A.ID, oldest); err != nil {
		t.Fatal(err)
	}
	var pending int64
	var first *time.Time
	if err := super.QueryRow(ctx, "SELECT count(*),min(created_at) FROM app.outbox_messages WHERE status='pending'").Scan(&pending, &first); err != nil {
		t.Fatal(err)
	}
	if err := stats.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if err := roles.Run(ctx); err != nil {
		t.Fatal(err)
	}
	before := scrape()
	if before["outbox_pending"] != float64(pending) || before["outbox_oldest_pending_seconds"] != clock.now.Sub(*first).Seconds() {
		t.Errorf("pending stats=%v count=%d first=%s", before, pending, first)
	}
	clock.now = clock.now.Add(time.Minute)
	after := scrape()
	if after["outbox_oldest_pending_seconds"] != before["outbox_oldest_pending_seconds"]+60 {
		t.Error("pending age does not advance at scrape")
	}
	var tenants, total, missing int64
	if err := super.QueryRow(ctx, "SELECT count(*) FROM app.tenants").Scan(&tenants); err != nil {
		t.Fatal(err)
	}
	if before["tenants_total"] != float64(tenants) || before["tenant_roles_missing"] != 0 {
		t.Errorf("inventory=%v tenants=%d", before, tenants)
	}
	total = int64(before["tenant_roles_total"])
	missing = int64(before["tenant_roles_missing"])
	role := db.TenantRoleName(f.A.ID)
	if _, err := super.Exec(ctx, "DROP ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	restore := func() {
		t.Helper()
		if err := f.Runner.InSystemTx(ctx, db.RoleSignup, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, "SELECT provisioning.provision_tenant_role($1)", f.A.ID)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(restore)
	if err := roles.Run(ctx); err != nil {
		t.Fatal(err)
	}
	after = scrape()
	if after["tenant_roles_total"] != float64(total-1) || after["tenant_roles_missing"] != float64(missing+1) {
		t.Errorf("missing role inventory=%v before=%v", after, before)
	}
	restore()
	if err := roles.Run(ctx); err != nil {
		t.Fatal(err)
	}
	after = scrape()
	if after["tenant_roles_total"] != float64(total) || after["tenant_roles_missing"] != float64(missing) {
		t.Errorf("restored=%v", after)
	}
	if _, err := super.Exec(ctx, "UPDATE app.outbox_messages SET status='sent',payload=NULL,sent_at=now() WHERE status='pending'"); err != nil {
		t.Fatal(err)
	}
	if err := stats.Run(ctx); err != nil {
		t.Fatal(err)
	}
	after = scrape()
	if after["outbox_pending"] != 0 || after["outbox_oldest_pending_seconds"] != 0 {
		t.Errorf("empty=%v", after)
	}
	runner.fail = true
	if stats.Run(ctx) == nil || roles.Run(ctx) == nil {
		t.Fatal("failed sample accepted")
	}
	for name, value := range scrape() {
		if value != -1 {
			t.Errorf("failed sample %s=%v", name, value)
		}
	}
}
