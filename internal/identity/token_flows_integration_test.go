//go:build integration

package identity

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/platform/securetoken"
	"github.com/google/uuid"
)

func deliveredToken(t *testing.T, runner db.TxRunner, tenantID uuid.UUID, recipient, template string) string {
	t.Helper()
	var raw []byte
	err := runner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT payload FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template=$3 ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, recipient, template).Scan(&raw)
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]string
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["token"] == "" {
		t.Fatal("missing token")
	}
	return payload["token"]
}

func TestPasswordResetLifecycle(t *testing.T) {
	ctx := context.Background()
	hasher := password.NewHasher(2)
	svc, runner, company, email, clock, _ := loginFixture(t, hasher)
	original, err := hasher.Hash(ctx, "old-password")
	if err != nil {
		t.Fatal(err)
	}
	setLoginPassword(t, runner, company, original, "active")
	session, err := svc.Login(ctx, email, "old-password", RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestPasswordReset(ctx, email, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	first := deliveredToken(t, runner, company.ID, email, "password_reset")
	if err := svc.RequestPasswordReset(ctx, email, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	second := deliveredToken(t, runner, company.ID, email, "password_reset")
	if first == second {
		t.Fatal("token was not renewed")
	}
	if err := svc.ConfirmPasswordReset(ctx, first, "replacement-password", RequestMeta{}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("old token: %v", err)
	}
	if err := svc.ConfirmPasswordReset(ctx, second, "short", RequestMeta{}); err == nil {
		t.Fatal("accepted short password")
	}
	if err := svc.ConfirmPasswordReset(ctx, second, "replacement-password", RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmPasswordReset(ctx, second, "another-password", RequestMeta{}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("reused token: %v", err)
	}
	if _, err := svc.ResolveSession(ctx, session.RawToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("old session: %v", err)
	}
	if _, err := svc.Login(ctx, email, "old-password", RequestMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password: %v", err)
	}
	if _, err := svc.Login(ctx, email, "replacement-password", RequestMeta{}); err != nil {
		t.Fatalf("new password: %v", err)
	}
	var used, revoked, audits int
	err = runner.InTenantTx(ctx, company.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE used_at IS NOT NULL), count(*) FILTER (WHERE revoked_at IS NOT NULL),
            (SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND action='auth.password_reset_completed' AND data->>'sessions_revoked'='2')
            FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='password_reset'`, company.ID, company.UserID).Scan(&used, &revoked, &audits)
	})
	if err != nil || used != 1 || revoked < 1 || audits != 1 {
		t.Fatalf("state: used=%d revoked=%d audits=%d err=%v", used, revoked, audits, err)
	}
	_ = clock
}

func TestPasswordResetInvitedAndUnknown(t *testing.T) {
	svc, runner, company, email, _, _ := loginFixture(t, password.NewHasher(2))
	setLoginPassword(t, runner, company, "", "invited")
	if err := svc.RequestPasswordReset(context.Background(), email, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	token := deliveredToken(t, runner, company.ID, email, "invitation")
	var purpose string
	err := runner.InSystemTx(context.Background(), db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT purpose FROM app.user_tokens WHERE token_hash=$1`, securetoken.Hash(token)).Scan(&purpose)
	})
	if err != nil || purpose != "invitation" {
		t.Fatalf("invitation: %s %v", purpose, err)
	}
	if err := svc.ConfirmPasswordReset(context.Background(), token, "new-password", RequestMeta{}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("wrong purpose: %v", err)
	}
	if err := svc.RequestPasswordReset(context.Background(), "missing@example.com", RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	setLoginPassword(t, runner, company, "", "disabled")
	if err := svc.RequestPasswordReset(context.Background(), email, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	var count int
	err = runner.InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND action='user.invitation_reissued'
            AND actor_user_id IS NULL AND data='{"role":"admin","trigger":"password_reset_request"}'::jsonb`, company.ID).Scan(&count)
	})
	if err != nil || count != 1 {
		t.Fatalf("invitation audit: %d %v", count, err)
	}
}

func TestEmailVerificationLifecycle(t *testing.T) {
	svc, runner, company, email, clock, _ := loginFixture(t, password.NewHasher(2))
	principal := authz.Principal{TenantID: company.ID, UserID: company.UserID, Role: authz.RoleAdmin}
	if err := svc.ResendEmailVerification(context.Background(), principal); err != nil {
		t.Fatal(err)
	}
	first := deliveredToken(t, runner, company.ID, email, "email_verification")
	if err := svc.ResendEmailVerification(context.Background(), principal); err != nil {
		t.Fatal(err)
	}
	second := deliveredToken(t, runner, company.ID, email, "email_verification")
	if first == second {
		t.Fatal("verification token not renewed")
	}
	if err := svc.ConfirmEmailVerification(context.Background(), first, RequestMeta{}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("old token: %v", err)
	}
	if err := svc.ConfirmEmailVerification(context.Background(), second, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmEmailVerification(context.Background(), second, RequestMeta{}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("reused token: %v", err)
	}
	if err := svc.ResendEmailVerification(context.Background(), principal); err != nil {
		t.Fatal(err)
	}
	var count int
	err := runner.InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template='email_verification'`, company.ID, email).Scan(&count)
	})
	if err != nil || count != 2 {
		t.Fatalf("verification messages: %d %v", count, err)
	}
	clock.now = clock.now.Add(49 * time.Hour)
	if err := svc.ConfirmEmailVerification(context.Background(), first, RequestMeta{}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expired token: %v", err)
	}
}

func TestResetExpiryAndConcurrentConsumption(t *testing.T) {
	svc, runner, company, email, clock, _ := loginFixture(t, password.NewHasher(2))
	ctx := context.Background()
	if err := svc.RequestPasswordReset(ctx, email, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	expired := deliveredToken(t, runner, company.ID, email, "password_reset")
	clock.now = clock.now.Add(time.Hour + time.Second)
	if err := svc.ConfirmPasswordReset(ctx, expired, "valid-password", RequestMeta{}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expired reset: %v", err)
	}
	if err := svc.RequestPasswordReset(ctx, email, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	valid := deliveredToken(t, runner, company.ID, email, "password_reset")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- svc.ConfirmPasswordReset(ctx, valid, "valid-password", RequestMeta{})
		}()
	}
	wg.Wait()
	close(results)
	succeeded, invalid := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrTokenInvalid):
			invalid++
		default:
			t.Fatalf("concurrent confirm: %v", err)
		}
	}
	if succeeded != 1 || invalid != 1 {
		t.Fatalf("concurrent outcomes: success=%d invalid=%d", succeeded, invalid)
	}
}

func TestPasswordResetEmailRateLimit(t *testing.T) {
	svc, runner, company, email, _, _ := loginFixture(t, password.NewHasher(2))
	ctx := context.Background()
	var third string
	for i := range 4 {
		if err := svc.RequestPasswordReset(ctx, email, RequestMeta{}); err != nil {
			t.Fatal(err)
		}
		got := deliveredToken(t, runner, company.ID, email, "password_reset")
		if i == 2 {
			third = got
		}
		if i == 3 && got != third {
			t.Fatal("fourth request issued a token")
		}
	}
}

func TestEmailVerificationExpires(t *testing.T) {
	svc, runner, company, email, clock, _ := loginFixture(t, password.NewHasher(2))
	p := authz.Principal{TenantID: company.ID, UserID: company.UserID, Role: authz.RoleAdmin}
	if err := svc.ResendEmailVerification(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	token := deliveredToken(t, runner, company.ID, email, "email_verification")
	clock.now = clock.now.Add(48*time.Hour + time.Second)
	if err := svc.ConfirmEmailVerification(context.Background(), token, RequestMeta{}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expired verification: %v", err)
	}
}
