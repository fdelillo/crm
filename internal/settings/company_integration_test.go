//go:build integration

package settings_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// testClock is the injected clock: a test moves time by hand to see which rows an operation touches.
type testClock struct {
	mu sync.Mutex
	at time.Time
}

func newTestClock() *testClock {
	return &testClock{at: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

// company is a registered-like company: role, tenant, one admin user and the configuration of its template.
type company struct {
	ID       uuid.UUID
	Admin    authz.Principal
	Operator authz.Principal
}

func testRunner(t *testing.T) db.TxRunner {
	t.Helper()
	return db.NewTxRunner(pgtest.AppPool(t))
}

func newService(t *testing.T, clk *testClock) *settings.Service {
	t.Helper()
	return settings.NewService(testRunner(t), audit.NewRecorder(), clk, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// newCompany does what the registration does (role, tenant, first admin, seeding) with the real
// TemplateSeeder. It does not go through tenant.Register: settings does not import tenant, not even in tests.
func newCompany(t *testing.T, clk *testClock, templateCode string) company {
	t.Helper()
	tpl, ok := industrytemplate.Lookup(templateCode)
	if !ok {
		t.Fatalf("unknown template %q", templateCode)
	}
	c := company{ID: uuid.New()}
	adminID := uuid.New()
	err := testRunner(t).InSystemTx(context.Background(), db.RoleSignup, func(ctx context.Context, tx db.Tx) error {
		var role string
		if err := tx.QueryRow(ctx, `SELECT provisioning.provision_tenant_role($1)`, c.ID).Scan(&role); err != nil {
			return err
		}
		if err := tx.AsTenant(ctx, c.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.tenants (id, name, base_currency, industry_template_code, industry_template_version)
			VALUES ($1, 'Empresa', 'ARS', $2, $3)`, c.ID, tpl.Code, tpl.Version); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.users (id, tenant_id, email, name, password_hash, role, status)
			VALUES ($1, $2, $3, 'Ana', '$argon2id$fixture', 'admin', 'active')`,
			adminID, c.ID, adminID.String()+"@example.test"); err != nil {
			return err
		}
		return settings.NewTemplateSeeder(clk).Seed(ctx, tx, c.ID, tpl)
	})
	if err != nil {
		t.Fatalf("new company: %v", err)
	}
	c.Admin = authz.Principal{TenantID: c.ID, UserID: adminID, SessionID: uuid.New(), Role: authz.RoleAdmin}
	c.Operator = authz.Principal{TenantID: c.ID, UserID: uuid.New(), SessionID: uuid.New(), Role: authz.RoleOperator}
	return c
}

// auditRow is one audit_log row as the company sees it.
type auditRow struct {
	Action     string
	ActorID    *uuid.UUID
	TargetType string
	TargetID   *uuid.UUID
	Data       map[string]any
	IP         *netip.Addr
	UserAgent  string
}

func readAudit(t *testing.T, tenantID uuid.UUID, actionPrefix string) []auditRow {
	t.Helper()
	var out []auditRow
	err := testRunner(t).InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT action, actor_user_id, coalesce(target_type, ''), target_id, data, ip, coalesce(user_agent, '')
			FROM app.audit_log WHERE tenant_id = $1 AND action LIKE $2 ORDER BY occurred_at, id`, tenantID, actionPrefix+"%")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r auditRow
			var raw []byte
			if err := rows.Scan(&r.Action, &r.ActorID, &r.TargetType, &r.TargetID, &raw, &r.IP, &r.UserAgent); err != nil {
				return err
			}
			if err := json.Unmarshal(raw, &r.Data); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	return out
}

// storedStages reads every row of app.stages of the company, deleted ones too, with every column that
// an operation may touch: two equal snapshots mean "nothing changed".
func storedStages(t *testing.T, tenantID uuid.UUID) string {
	t.Helper()
	var lines []string
	err := testRunner(t).InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, name, type, color, position, deleted_at, created_at, updated_at
			FROM app.stages WHERE tenant_id = $1 ORDER BY created_at, id`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			var name, typ, color string
			var position int
			var deleted *time.Time
			var created, updated time.Time
			if err := rows.Scan(&id, &name, &typ, &color, &position, &deleted, &created, &updated); err != nil {
				return err
			}
			lines = append(lines, fmt.Sprintf("%s|%s|%s|%s|%d|%v|%s|%s", id, name, typ, color, position, deleted, created.UTC(), updated.UTC()))
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("read stages: %v", err)
	}
	return strings.Join(lines, "\n")
}
