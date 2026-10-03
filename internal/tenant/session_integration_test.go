//go:build integration

package tenant_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

type movableClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *movableClock) Now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *movableClock) Set(v time.Time) { c.mu.Lock(); c.now = v; c.mu.Unlock() }

func TestSessionExpirationAndCurrentUserState(t *testing.T) {
	ctx := context.Background()
	start := time.Now().UTC().Truncate(time.Second)
	c := &movableClock{now: start}
	runner := db.NewTxRunner(pgtest.AppPool(t))
	ident := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour)
	svc := tenant.NewService(runner, ident, industrytemplate.NoopSeeder{}, password.NewHasher(2),
		audit.NewRecorder(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	reg, err := svc.Register(ctx, signupWithEmail(uuid.NewString()+"@example.com"), identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if !reg.Session.ExpiresAt.Equal(start.Add(7 * 24 * time.Hour)) {
		t.Fatalf("session expiry=%s", reg.Session.ExpiresAt)
	}
	raw := reg.Session.RawToken
	if _, err := ident.ResolveSession(ctx, raw+"bogus"); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("unknown token: %v", err)
	}
	readSeen := func() time.Time {
		t.Helper()
		var seen time.Time
		err := runner.InTenantTx(ctx, reg.Tenant.ID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT last_seen_at FROM app.sessions WHERE tenant_id=$1 AND id=$2`,
				reg.Tenant.ID, reg.Session.Principal.SessionID).Scan(&seen)
		})
		if err != nil {
			t.Fatal(err)
		}
		return seen
	}
	resolve := func(wantRole string) {
		t.Helper()
		principal, err := ident.ResolveSession(ctx, raw)
		if err != nil || string(principal.Role) != wantRole || principal.TenantID != reg.Tenant.ID ||
			principal.UserID != reg.User.ID || principal.SessionID != reg.Session.Principal.SessionID {
			t.Fatalf("session principal=%+v err=%v", principal, err)
		}
	}
	c.Set(start.Add(4 * time.Minute))
	resolve("admin")
	if !readSeen().Equal(start) {
		t.Fatal("last_seen_at changed within five minutes")
	}
	c.Set(start.Add(6 * time.Minute))
	resolve("admin")
	if !readSeen().Equal(c.Now()) {
		t.Fatal("last_seen_at was not refreshed after five minutes")
	}
	// A database role change is reflected by the very next authenticated request.
	err = runner.InTenantTx(ctx, reg.Tenant.ID, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.users SET role='operator', email_verified_at=now()
			WHERE tenant_id=$1 AND id=$2`, reg.Tenant.ID, reg.User.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	resolve("operator")
	user, err := ident.Me(ctx, reg.Session.Principal)
	if err != nil || !user.EmailVerified || string(user.Role) != "operator" {
		t.Fatalf("current user=%+v err=%v", user, err)
	}
	c.Set(start.Add(23*time.Hour + 59*time.Minute))
	resolve("operator")
	for i := 2; i < 14; i++ {
		c.Set(start.Add(time.Duration(i) * 12 * time.Hour))
		resolve("operator")
	}
	c.Set(start.Add(7*24*time.Hour + time.Second))
	if _, err := ident.ResolveSession(ctx, raw); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("absolute expiry: %v", err)
	}
}

func TestSessionIdleRevokedAndDisabled(t *testing.T) {
	ctx := context.Background()
	start := time.Now().UTC().Truncate(time.Second)
	c := &movableClock{now: start}
	runner := db.NewTxRunner(pgtest.AppPool(t))
	ident := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour)
	svc := tenant.NewService(runner, ident, industrytemplate.NoopSeeder{}, password.NewHasher(2),
		audit.NewRecorder(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	reg, err := svc.Register(ctx, signupWithEmail(uuid.NewString()+"@example.com"), identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	c.Set(start.Add(24*time.Hour + time.Second))
	if _, err := ident.ResolveSession(ctx, reg.Session.RawToken); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("idle expiry: %v", err)
	}
	c.Set(start.Add(time.Hour))
	update := func(query string) {
		t.Helper()
		err := runner.InTenantTx(ctx, reg.Tenant.ID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, query, reg.Tenant.ID, reg.User.ID)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	update(`UPDATE app.users SET status='disabled' WHERE tenant_id=$1 AND id=$2`)
	if _, err := ident.ResolveSession(ctx, reg.Session.RawToken); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("disabled user: %v", err)
	}
	update(`UPDATE app.users SET status='active' WHERE tenant_id=$1 AND id=$2`)
	update(`UPDATE app.sessions SET revoked_at=now(), revoked_reason='user_disabled' WHERE tenant_id=$1 AND user_id=$2`)
	if _, err := ident.ResolveSession(ctx, reg.Session.RawToken); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("revoked session: %v", err)
	}
}
