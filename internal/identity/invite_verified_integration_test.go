//go:build integration

package identity

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

// unverifiedFixture is a company with its founder as the only user: administrator, active, with a
// real session and an email that was not verified (what a fresh signup leaves).
func unverifiedFixture(t *testing.T) (*Service, db.TxRunner, authz.Principal, *loginClock) {
	t.Helper()
	svc, runner, company, _, c, _ := loginFixture(t, password.NewHasher(2))
	return svc, runner, authz.Principal{TenantID: company.ID, UserID: company.UserID, SessionID: company.Rows["sessions"], Role: authz.RoleAdmin}, c
}

func tenantFootprint(t *testing.T, r db.TxRunner, tenantID uuid.UUID) [4]int {
	t.Helper()
	var n [4]int
	if err := r.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.users WHERE tenant_id=$1),
		  (SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1),
		  (SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1),
		  (SELECT count(*) FROM app.audit_log WHERE tenant_id=$1)`, tenantID).Scan(&n[0], &n[1], &n[2], &n[3])
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

func emailOf(t *testing.T, r db.TxRunner, p authz.Principal) string {
	t.Helper()
	var email string
	if err := r.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, p.TenantID, p.UserID).Scan(&email)
	}); err != nil {
		t.Fatal(err)
	}
	return email
}

func verifiedAt(t *testing.T, r db.TxRunner, p authz.Principal) *time.Time {
	t.Helper()
	var at *time.Time
	if err := r.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT email_verified_at FROM app.users WHERE tenant_id=$1 AND id=$2`, p.TenantID, p.UserID).Scan(&at)
	}); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestInviteRequiresVerifiedEmail(t *testing.T) {
	ctx := context.Background()
	meta := RequestMeta{}
	t.Run("V1 an unverified administrator cannot invite", func(t *testing.T) {
		s, r, p, _ := unverifiedFixture(t)
		before := tenantFootprint(t, r, p.TenantID)
		if _, _, err := s.Invite(ctx, p, "new-"+uuid.NewString()+"@example.com", authz.RoleOperator, meta); !errors.Is(err, ErrEmailNotVerified) {
			t.Fatalf("got %v, want ErrEmailNotVerified", err)
		}
		if after := tenantFootprint(t, r, p.TenantID); after != before {
			t.Fatalf("the refused invitation left rows: %v -> %v", before, after)
		}
	})
	t.Run("V2 the check comes before looking up the email", func(t *testing.T) {
		s, _, p, _ := unverifiedFixture(t)
		other := fixture.NewCompany(t, pgtest.AppPool(t))
		var email string
		if err := db.NewTxRunner(pgtest.AppPool(t)).InTenantTx(ctx, other.ID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, other.ID, other.UserID).Scan(&email)
		}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Invite(ctx, p, email, authz.RoleOperator, meta); !errors.Is(err, ErrEmailNotVerified) {
			t.Fatalf("got %v, want ErrEmailNotVerified and not ErrEmailTaken", err)
		}
	})
	t.Run("V3 after confirming the email it can invite", func(t *testing.T) {
		s, r, p, _ := unverifiedFixture(t)
		if err := s.ResendEmailVerification(ctx, p); err != nil {
			t.Fatal(err)
		}
		token := deliveredToken(t, r, p.TenantID, emailOf(t, r, p), "email_verification")
		if err := s.ConfirmEmailVerification(ctx, token, meta); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Invite(ctx, p, "new-"+uuid.NewString()+"@example.com", authz.RoleOperator, meta); err != nil {
			t.Fatalf("verified administrator: %v", err)
		}
	})
	t.Run("V4 an administrator that arrived by an accepted invitation invites with no further step", func(t *testing.T) {
		s, r, p, _ := usersFixture(t)
		invited := inviteForTest(t, s, p, authz.RoleAdmin)
		accepted, err := s.AcceptInvitation(ctx, deliveredToken(t, r, p.TenantID, invited.Email, "invitation"), "B", "valid-password", meta)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Invite(ctx, accepted.Principal, "new-"+uuid.NewString()+"@example.com", authz.RoleOperator, meta); err != nil {
			t.Fatalf("accepted administrator: %v", err)
		}
	})
	t.Run("V9 an unverified administrator has nobody to reactivate", func(t *testing.T) {
		s, _, p, _ := unverifiedFixture(t)
		users, err := s.ListUsers(ctx, p)
		if err != nil || len(users) != 1 || users[0].ID != p.UserID {
			t.Fatalf("users of a fresh company: %+v %v", users, err)
		}
		other := fixture.NewCompany(t, pgtest.AppPool(t))
		for _, id := range []uuid.UUID{uuid.New(), other.UserID} {
			if _, err := s.Reactivate(ctx, p, id, meta); !errors.Is(err, ErrUserNotFound) {
				t.Fatalf("reactivate %s: %v", id, err)
			}
		}
	})
}

func TestPasswordResetVerifiesTheEmail(t *testing.T) {
	ctx := context.Background()
	meta := RequestMeta{}
	setup := func(t *testing.T, verified *time.Time) (*Service, db.TxRunner, fixture.Company, string, *loginClock) {
		hasher := password.NewHasher(2)
		s, r, c, email, clock, _ := loginFixture(t, hasher)
		hash, err := hasher.Hash(ctx, "old-password")
		if err != nil {
			t.Fatal(err)
		}
		setLoginPassword(t, r, c, hash, "active")
		if err := r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE app.users SET email_verified_at=$3 WHERE tenant_id=$1 AND id=$2`, c.ID, c.UserID, verified)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.RequestPasswordReset(ctx, email, meta); err != nil {
			t.Fatal(err)
		}
		return s, r, c, email, clock
	}
	t.Run("V5 confirming a reset verifies an unverified email", func(t *testing.T) {
		s, r, c, email, clock := setup(t, nil)
		if err := s.ConfirmPasswordReset(ctx, deliveredToken(t, r, c.ID, email, "password_reset"), "new-password", meta); err != nil {
			t.Fatal(err)
		}
		p := authz.Principal{TenantID: c.ID, UserID: c.UserID}
		if at := verifiedAt(t, r, p); at == nil || !at.Equal(clock.now) {
			t.Fatalf("email_verified_at=%v, want the clock %v", at, clock.now)
		}
		session, err := s.Login(ctx, email, "new-password", meta)
		if err != nil {
			t.Fatal(err)
		}
		me, err := s.Me(ctx, session.Principal)
		if err != nil || !me.EmailVerified {
			t.Fatalf("/me: %+v %v", me, err)
		}
		// The audit entry keeps the keys it had (the fixture's session is the one it revokes).
		var raw []byte
		if err := r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT data FROM app.audit_log WHERE tenant_id=$1 AND action='auth.password_reset_completed' AND target_id=$2`, c.ID, c.UserID).Scan(&raw)
		}); err != nil {
			t.Fatal(err)
		}
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil || !reflect.DeepEqual(data, map[string]any{"sessions_revoked": float64(1)}) {
			t.Fatalf("audit data: %s (%v)", raw, err)
		}
	})
	t.Run("V6 confirming a reset keeps an existing verification date", func(t *testing.T) {
		before := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
		s, r, c, email, _ := setup(t, &before)
		if err := s.ConfirmPasswordReset(ctx, deliveredToken(t, r, c.ID, email, "password_reset"), "new-password", meta); err != nil {
			t.Fatal(err)
		}
		if at := verifiedAt(t, r, authz.Principal{TenantID: c.ID, UserID: c.UserID}); at == nil || !at.Equal(before) {
			t.Fatalf("email_verified_at=%v, want %v", at, before)
		}
	})
}
