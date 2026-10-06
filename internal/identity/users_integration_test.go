//go:build integration

package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func usersFixture(t *testing.T) (*Service, db.TxRunner, authz.Principal, *loginClock) {
	t.Helper()
	svc, runner, company, _, c, _ := loginFixture(t, password.NewHasher(2))
	return svc, runner, authz.Principal{TenantID: company.ID, UserID: company.UserID, Role: authz.RoleAdmin}, c
}
func inviteForTest(t *testing.T, s *Service, p authz.Principal, role authz.Role) User {
	t.Helper()
	u, reissued, err := s.Invite(context.Background(), p, uuid.NewString()+"@example.com", role, RequestMeta{})
	if err != nil || reissued {
		t.Fatalf("invite: %v reissued=%v", err, reissued)
	}
	return u
}
func requireUserAudit(t *testing.T, runner db.TxRunner, p authz.Principal, userID uuid.UUID, action string, actor uuid.UUID, expected map[string]any) {
	t.Helper()
	var data []byte
	var actorID uuid.UUID
	var target string
	err := runner.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT data, actor_user_id, target_type FROM app.audit_log WHERE tenant_id=$1 AND target_id=$2 AND action=$3 ORDER BY occurred_at DESC, id DESC LIMIT 1`, p.TenantID, userID, action).Scan(&data, &actorID, &target)
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) || actorID != actor || target != "user" {
		t.Fatalf("%s audit: data=%v actor=%s target=%s", action, got, actorID, target)
	}
}
func userAuditCount(t *testing.T, runner db.TxRunner, p authz.Principal) int {
	t.Helper()
	var n int
	if err := runner.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND action LIKE 'user.%'`, p.TenantID).Scan(&n)
	}); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestInviteAndReinvite(t *testing.T) {
	s, runner, p, c := usersFixture(t)
	ctx := context.Background()
	u := inviteForTest(t, s, p, authz.RoleOperator)
	if u.Status != "invited" || u.Name != nil || u.EmailVerified || u.InvitationExpiresAt == nil || !u.InvitationExpiresAt.Equal(c.now.Add(7*24*time.Hour)) {
		t.Fatalf("user: %+v", u)
	}
	first := deliveredToken(t, runner, p.TenantID, u.Email, "invitation")
	requireUserAudit(t, runner, p, u.ID, "user.invited", p.UserID, map[string]any{"role": "operator"})
	for _, role := range []authz.Role{authz.RoleOperator, authz.RoleAdmin, authz.RoleOperator} {
		before := userAuditCount(t, runner, p)
		oldRole := u.Role
		got, reissued, err := s.Invite(ctx, p, "  "+strings.ToUpper(u.Email)+"  ", role, RequestMeta{})
		if err != nil || !reissued || got.ID != u.ID || got.Role != role {
			t.Fatalf("reinvite: %+v %v %v", got, reissued, err)
		}
		requireUserAudit(t, runner, p, u.ID, "user.invitation_reissued", p.UserID, map[string]any{"role": string(role), "trigger": "admin"})
		delta := 1
		if oldRole != role {
			delta++
			requireUserAudit(t, runner, p, u.ID, "user.role_changed", p.UserID, map[string]any{"from": string(oldRole), "to": string(role), "status": "invited"})
		}
		if userAuditCount(t, runner, p) != before+delta {
			t.Fatal("unexpected audit rows")
		}
		u = got
	}
	if _, err := s.PreviewInvitation(ctx, first); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("superseded: %v", err)
	}
	var users, open, messages int
	var creator uuid.UUID
	var hasPassword bool
	if err := runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.users WHERE tenant_id=$1 AND email=$2),
    (SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$3 AND purpose='invitation' AND used_at IS NULL AND revoked_at IS NULL),
    (SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template='invitation' AND status='pending'),
    (SELECT created_by_user_id FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$3 AND purpose='invitation' AND used_at IS NULL AND revoked_at IS NULL),
    (SELECT password_hash IS NOT NULL FROM app.users WHERE tenant_id=$1 AND id=$3)`, p.TenantID, u.Email, u.ID).Scan(&users, &open, &messages, &creator, &hasPassword)
	}); err != nil {
		t.Fatal(err)
	}
	if users != 1 || open != 1 || messages != 4 || creator != p.UserID || hasPassword {
		t.Fatalf("users=%d open=%d messages=%d creator=%s hash=%v", users, open, messages, creator, hasPassword)
	}
	_ = inviteForTest(t, s, p, authz.RoleAdmin)
	for _, status := range []string{"active", "invited", "disabled"} {
		other := fixture.NewCompany(t, pgtest.AppPool(t))
		var email string
		if err := runner.InTenantTx(ctx, other.ID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `UPDATE app.users SET status=$1 WHERE tenant_id=$2 AND id=$3 RETURNING email`, status, other.ID, other.UserID).Scan(&email)
		}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Invite(ctx, p, email, authz.RoleOperator, RequestMeta{}); !errors.Is(err, ErrEmailTaken) {
			t.Fatalf("other %s: %v", status, err)
		}
	}
	var ownEmail string
	if err := runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, p.TenantID, p.UserID).Scan(&ownEmail)
	}); err != nil {
		t.Fatal(err)
	}
	before := userAuditCount(t, runner, p)
	for _, email := range []string{ownEmail, "not an email", "x\x00@example.com"} {
		_, _, err := s.Invite(ctx, p, email, authz.RoleOperator, RequestMeta{})
		if err == nil {
			t.Fatalf("accepted %q", email)
		}
	}
	if _, err := s.Deactivate(ctx, p, u.ID, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Invite(ctx, p, u.Email, authz.RoleOperator, RequestMeta{}); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("disabled email: %v", err)
	}
	if userAuditCount(t, runner, p) != before+1 {
		t.Fatal("failed invitations audited")
	}
}
func TestInvitationPreviewAcceptanceAndInvalidTokens(t *testing.T) {
	for _, scenario := range []string{"valid", "expired", "used", "reset_reissued", "disabled", "wrong_purpose"} {
		t.Run(scenario, func(t *testing.T) {
			s, runner, p, c := usersFixture(t)
			ctx := context.Background()
			u := inviteForTest(t, s, p, authz.RoleOperator)
			raw := deliveredToken(t, runner, p.TenantID, u.Email, "invitation")
			if _, err := s.ChangeRole(ctx, p, u.ID, authz.RoleAdmin, RequestMeta{}); err != nil {
				t.Fatal(err)
			}
			before := userAuditCount(t, runner, p)
			switch scenario {
			case "expired":
				c.now = c.now.Add(7*24*time.Hour + time.Second)
			case "used":
				if _, err := s.AcceptInvitation(ctx, raw, "Ana", "valid-password", RequestMeta{}); err != nil {
					t.Fatal(err)
				}
			case "reset_reissued":
				if err := s.RequestPasswordReset(ctx, u.Email, RequestMeta{}); err != nil {
					t.Fatal(err)
				}
				newer := deliveredToken(t, runner, p.TenantID, u.Email, "invitation")
				if _, err := s.PreviewInvitation(ctx, newer); err != nil {
					t.Fatal(err)
				}
				if _, err := s.AcceptInvitation(ctx, newer, "Ana", "valid-password", RequestMeta{}); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				if _, err := s.Deactivate(ctx, p, u.ID, RequestMeta{}); err != nil {
					t.Fatal(err)
				}
			case "wrong_purpose":
				if err := s.ResendEmailVerification(ctx, p); err != nil {
					t.Fatal(err)
				}
				var email string
				if err := runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
					return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, p.TenantID, p.UserID).Scan(&email)
				}); err != nil {
					t.Fatal(err)
				}
				raw = deliveredToken(t, runner, p.TenantID, email, "email_verification")
			}
			preview, err := s.PreviewInvitation(ctx, raw)
			if scenario != "valid" {
				if !errors.Is(err, ErrTokenInvalid) {
					t.Fatalf("preview: %v", err)
				}
				before = userAuditCount(t, runner, p)
				if _, err := s.AcceptInvitation(ctx, raw, "Ana", "valid-password", RequestMeta{}); !errors.Is(err, ErrTokenInvalid) {
					t.Fatalf("accept: %v", err)
				}
				if userAuditCount(t, runner, p) != before {
					t.Fatal("failed accept audited")
				}
				return
			}
			if err != nil || preview.Role != authz.RoleAdmin || preview.Email != u.Email || preview.TenantName != "Fixture SA" || !preview.ExpiresAt.Equal(c.now.Add(7*24*time.Hour)) {
				t.Fatalf("preview: %+v %v", preview, err)
			}
			for _, input := range []struct{ name, plain string }{{"Ana", "short"}, {"", "valid-password"}, {"Ana\x00", "valid-password"}, {strings.Repeat("x", 121), "valid-password"}} {
				if _, err := s.AcceptInvitation(ctx, raw, input.name, input.plain, RequestMeta{}); err == nil {
					t.Fatal("invalid input accepted")
				}
				if _, err := s.PreviewInvitation(ctx, raw); err != nil {
					t.Fatalf("invalid input consumed token: %v", err)
				}
			}
			session, err := s.AcceptInvitation(ctx, raw, "  Ana  ", "valid-password", RequestMeta{})
			if err != nil || session.Principal.Role != authz.RoleAdmin || session.Principal.UserID != u.ID {
				t.Fatalf("session: %+v %v", session, err)
			}
			me, err := s.Me(ctx, session.Principal)
			if err != nil || me.Name != "Ana" || me.Status != "active" || !me.EmailVerified {
				t.Fatalf("me: %+v %v", me, err)
			}
			if _, err := s.Login(ctx, u.Email, "valid-password", RequestMeta{}); err != nil {
				t.Fatal(err)
			}
			requireUserAudit(t, runner, p, u.ID, "user.invitation_accepted", u.ID, map[string]any{})
			if userAuditCount(t, runner, p) != before+1 {
				t.Fatal("unexpected acceptance audits")
			}
			var used bool
			if err := runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
				return tx.QueryRow(ctx, `SELECT used_at IS NOT NULL FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='invitation'`, p.TenantID, u.ID).Scan(&used)
			}); err != nil || !used {
				t.Fatalf("used=%v err=%v", used, err)
			}
		})
	}
}
func TestRoleChangesAndLastAdmin(t *testing.T) {
	s, runner, p, _ := usersFixture(t)
	ctx := context.Background()
	u := inviteForTest(t, s, p, authz.RoleOperator)
	raw := deliveredToken(t, runner, p.TenantID, u.Email, "invitation")
	for _, role := range []authz.Role{authz.RoleAdmin, authz.RoleOperator} {
		from := u.Role
		var err error
		u, err = s.ChangeRole(ctx, p, u.ID, role, RequestMeta{})
		if err != nil || u.Status != "invited" || u.Role != role {
			t.Fatalf("role: %+v %v", u, err)
		}
		requireUserAudit(t, runner, p, u.ID, "user.role_changed", p.UserID, map[string]any{"from": string(from), "to": string(role), "status": "invited"})
		if got := deliveredToken(t, runner, p.TenantID, u.Email, "invitation"); got != raw {
			t.Fatal("role change replaced invitation")
		}
	}
	before := userAuditCount(t, runner, p)
	if _, err := s.ChangeRole(ctx, p, u.ID, u.Role, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if userAuditCount(t, runner, p) != before {
		t.Fatal("no-op audited")
	}
	if _, err := s.ChangeRole(ctx, p, p.UserID, authz.RoleOperator, RequestMeta{}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("last admin: %v", err)
	}
	if _, err := s.ChangeRole(ctx, p, u.ID, authz.RoleAdmin, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeRole(ctx, p, p.UserID, authz.RoleOperator, RequestMeta{}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("invited admin counted: %v", err)
	}
	if _, err := s.AcceptInvitation(ctx, raw, "B", "valid-password", RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeRole(ctx, p, p.UserID, authz.RoleOperator, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	requireUserAudit(t, runner, p, p.UserID, "user.role_changed", p.UserID, map[string]any{"from": "admin", "to": "operator", "status": "active"})
	if _, err := s.ChangeRole(ctx, p, u.ID, authz.RoleOperator, RequestMeta{}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("last active: %v", err)
	}
	if _, err := s.ChangeRole(ctx, p, p.UserID, authz.RoleAdmin, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Deactivate(ctx, p, u.ID, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	before = userAuditCount(t, runner, p)
	for _, role := range []authz.Role{authz.RoleAdmin, authz.RoleOperator} {
		if _, err := s.ChangeRole(ctx, p, u.ID, role, RequestMeta{}); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("disabled role: %v", err)
		}
	}
	if _, err := s.ChangeRole(ctx, p, p.UserID, authz.RoleOperator, RequestMeta{}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disabled admin counted: %v", err)
	}
	other := fixture.NewCompany(t, pgtest.AppPool(t))
	for _, id := range []uuid.UUID{uuid.New(), other.UserID} {
		if _, err := s.ChangeRole(ctx, p, id, authz.RoleOperator, RequestMeta{}); !errors.Is(err, ErrUserNotFound) {
			t.Fatal(err)
		}
		if _, err := s.Deactivate(ctx, p, id, RequestMeta{}); !errors.Is(err, ErrUserNotFound) {
			t.Fatal(err)
		}
		if _, err := s.Reactivate(ctx, p, id, RequestMeta{}); !errors.Is(err, ErrUserNotFound) {
			t.Fatal(err)
		}
	}
	if userAuditCount(t, runner, p) != before {
		t.Fatal("failed operations audited")
	}
}
func TestDeactivateReactivateAndConstraint(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(fmt.Sprint(accepted), func(t *testing.T) {
			s, runner, p, c := usersFixture(t)
			ctx := context.Background()
			u := inviteForTest(t, s, p, authz.RoleOperator)
			raw := deliveredToken(t, runner, p.TenantID, u.Email, "invitation")
			var sessions []SessionResult
			if accepted {
				session, err := s.AcceptInvitation(ctx, raw, "Ana", "valid-password", RequestMeta{})
				if err != nil {
					t.Fatal(err)
				}
				sessions = append(sessions, session)
				session, err = s.Login(ctx, u.Email, "valid-password", RequestMeta{})
				if err != nil {
					t.Fatal(err)
				}
				sessions = append(sessions, session)
				if err := s.RequestPasswordReset(ctx, u.Email, RequestMeta{}); err != nil {
					t.Fatal(err)
				}
			}
			disabled, err := s.Deactivate(ctx, p, u.ID, RequestMeta{})
			if err != nil || disabled.Status != "disabled" || disabled.InvitationExpiresAt != nil {
				t.Fatalf("disabled: %+v %v", disabled, err)
			}
			count := float64(0)
			if accepted {
				count = 2
			}
			requireUserAudit(t, runner, p, u.ID, "user.deactivated", p.UserID, map[string]any{"sessions_revoked": count})
			for _, session := range sessions {
				if _, err := s.ResolveSession(ctx, session.RawToken); !errors.Is(err, ErrUnauthenticated) {
					t.Fatal(err)
				}
			}
			if _, err := s.PreviewInvitation(ctx, raw); !errors.Is(err, ErrTokenInvalid) {
				t.Fatal(err)
			}
			before := userAuditCount(t, runner, p)
			if _, err := s.Deactivate(ctx, p, u.ID, RequestMeta{}); !errors.Is(err, ErrInvalidTransition) {
				t.Fatal(err)
			}
			if userAuditCount(t, runner, p) != before {
				t.Fatal("failed deactivation audited")
			}
			if !accepted {
				err := runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
					_, err := tx.Exec(ctx, `UPDATE app.users SET status='active' WHERE tenant_id=$1 AND id=$2`, p.TenantID, u.ID)
					return db.MapError(err)
				})
				var constraint *pgconn.PgError
				if !errors.As(err, &constraint) || constraint.Code != "23514" || constraint.ConstraintName != "users_active_complete_chk" {
					t.Fatalf("constraint: %v", err)
				}
			}
			got, err := s.Reactivate(ctx, p, u.ID, RequestMeta{})
			want := "invited"
			delta := 2
			if accepted {
				want = "active"
				delta = 1
			}
			if err != nil || got.Status != want {
				t.Fatalf("reactivate: %+v %v", got, err)
			}
			requireUserAudit(t, runner, p, u.ID, "user.reactivated", p.UserID, map[string]any{"to_status": want})
			if userAuditCount(t, runner, p) != before+delta {
				t.Fatal("reactivation audit count")
			}
			for _, session := range sessions {
				if _, err := s.ResolveSession(ctx, session.RawToken); !errors.Is(err, ErrUnauthenticated) {
					t.Fatal("reopened session")
				}
			}
			if accepted {
				if _, err := s.Login(ctx, u.Email, "valid-password", RequestMeta{}); err != nil {
					t.Fatal(err)
				}
				var revoked int
				if err := runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
					return tx.QueryRow(ctx, `SELECT count(*) FROM app.sessions WHERE tenant_id=$1 AND user_id=$2 AND revoked_reason='user_disabled'`, p.TenantID, u.ID).Scan(&revoked)
				}); err != nil || revoked != 2 {
					t.Fatalf("revoked=%d %v", revoked, err)
				}
			} else {
				requireUserAudit(t, runner, p, u.ID, "user.invitation_reissued", p.UserID, map[string]any{"role": "operator", "trigger": "reactivation"})
				if got.InvitationExpiresAt == nil || !got.InvitationExpiresAt.Equal(c.now.Add(7*24*time.Hour)) {
					t.Fatalf("expiry: %+v", got)
				}
				newer := deliveredToken(t, runner, p.TenantID, u.Email, "invitation")
				if newer == raw {
					t.Fatal("reused invitation")
				}
				if _, err := s.PreviewInvitation(ctx, newer); err != nil {
					t.Fatal(err)
				}
			}
			before = userAuditCount(t, runner, p)
			if _, err := s.Reactivate(ctx, p, u.ID, RequestMeta{}); !errors.Is(err, ErrInvalidTransition) {
				t.Fatal(err)
			}
			if userAuditCount(t, runner, p) != before {
				t.Fatal("failed reactivation audited")
			}
		})
	}
}
func concurrentUsers(t *testing.T, a, b func() error) []error {
	t.Helper()
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, fn := range []func() error{a, b} {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- fn() }()
	}
	close(start)
	wg.Wait()
	close(results)
	var out []error
	for err := range results {
		out = append(out, err)
	}
	return out
}
func TestConcurrentUserOperations(t *testing.T) {
	for _, operation := range []string{"reinvite", "accept", "demote", "deactivate", "reactivate_deactivate"} {
		t.Run(operation, func(t *testing.T) {
			for i := range 20 {
				t.Run(fmt.Sprint(i), func(t *testing.T) {
					s, runner, p, _ := usersFixture(t)
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					initialRole := authz.RoleAdmin
					if operation == "reinvite" {
						initialRole = authz.RoleOperator
					}
					u := inviteForTest(t, s, p, initialRole)
					raw := deliveredToken(t, runner, p.TenantID, u.Email, "invitation")
					if operation != "reinvite" && operation != "accept" {
						if _, err := s.AcceptInvitation(ctx, raw, "B", "valid-password", RequestMeta{}); err != nil {
							t.Fatal(err)
						}
					}
					if operation == "reactivate_deactivate" {
						if _, err := s.Deactivate(ctx, p, u.ID, RequestMeta{}); err != nil {
							t.Fatal(err)
						}
					}
					before := userAuditCount(t, runner, p)
					var a, b func() error
					other := authz.Principal{TenantID: p.TenantID, UserID: u.ID, Role: authz.RoleAdmin}
					switch operation {
					case "reinvite":
						// The ninth review starts with an operator and permits either serial order.
						a = func() error { _, _, err := s.Invite(ctx, p, u.Email, authz.RoleOperator, RequestMeta{}); return err }
						b = func() error { _, _, err := s.Invite(ctx, p, u.Email, authz.RoleAdmin, RequestMeta{}); return err }
					case "accept":
						a = func() error { _, err := s.AcceptInvitation(ctx, raw, "B", "valid-password", RequestMeta{}); return err }
						b = a
					case "demote":
						a = func() error { _, err := s.ChangeRole(ctx, p, u.ID, authz.RoleOperator, RequestMeta{}); return err }
						b = func() error {
							_, err := s.ChangeRole(ctx, other, p.UserID, authz.RoleOperator, RequestMeta{})
							return err
						}
					case "deactivate":
						a = func() error { _, err := s.Deactivate(ctx, p, u.ID, RequestMeta{}); return err }
						b = func() error { _, err := s.Deactivate(ctx, other, p.UserID, RequestMeta{}); return err }
					default:
						a = func() error { _, err := s.Reactivate(ctx, p, u.ID, RequestMeta{}); return err }
						b = func() error { _, err := s.Deactivate(ctx, p, u.ID, RequestMeta{}); return err }
					}
					results := concurrentUsers(t, a, b)
					success := 0
					for _, err := range results {
						if err == nil {
							success++
							continue
						}
						expected := ErrLastAdmin
						if operation == "accept" {
							expected = ErrTokenInvalid
						}
						if operation == "reactivate_deactivate" {
							expected = ErrInvalidTransition
						}
						if !errors.Is(err, expected) {
							t.Fatalf("outcomes: %v", results)
						}
					}
					if operation == "reinvite" {
						if success != 2 {
							t.Fatalf("outcomes: %v", results)
						}
						// Deduce the serial order from the final role, never transaction
						// timestamps: the transaction waiting on the tenant may have started first.
						var final string
						type event struct {
							action string
							data   map[string]any
							xid    string
						}
						var events []event
						if err := runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
							if err := tx.QueryRow(ctx, `SELECT role FROM app.users WHERE tenant_id=$1 AND id=$2`, p.TenantID, u.ID).Scan(&final); err != nil {
								return err
							}
							rows, err := tx.Query(ctx, `SELECT action,data,xmin::text FROM app.audit_log WHERE tenant_id=$1 AND target_id=$2 AND action IN ('user.invitation_reissued','user.role_changed')`, p.TenantID, u.ID)
							if err != nil {
								return err
							}
							defer rows.Close()
							for rows.Next() {
								var e event
								var raw []byte
								if err := rows.Scan(&e.action, &raw, &e.xid); err != nil {
									return err
								}
								if err := json.Unmarshal(raw, &e.data); err != nil {
									return err
								}
								events = append(events, e)
							}
							return rows.Err()
						}); err != nil {
							t.Fatal(err)
						}
						order := []string{"operator", "admin"}
						if final == "operator" {
							order = []string{"admin", "operator"}
						} else if final != "admin" {
							t.Fatal(final)
						}
						reissues := map[string]event{}
						changes := map[string][]event{}
						for _, e := range events {
							if e.action == "user.invitation_reissued" {
								role, ok := e.data["role"].(string)
								if !ok {
									t.Fatal(e)
								}
								if _, duplicate := reissues[role]; duplicate {
									t.Fatal("duplicate reissue", events)
								}
								reissues[role] = e
							} else {
								changes[e.xid] = append(changes[e.xid], e)
							}
						}
						if len(reissues) != 2 {
							t.Fatal(events)
						}
						previous := "operator"
						expectedChanges := 0
						for _, role := range order {
							e, ok := reissues[role]
							if !ok || !reflect.DeepEqual(e.data, map[string]any{"role": role, "trigger": "admin"}) {
								t.Fatal(events)
							}
							paired := changes[e.xid]
							if previous == role {
								if len(paired) != 0 {
									t.Fatal("no-op role change audited", events)
								}
							} else {
								expectedChanges++
								if len(paired) != 1 || !reflect.DeepEqual(paired[0].data, map[string]any{"from": previous, "to": role, "status": "invited"}) {
									t.Fatal("role chain or transaction differs", events)
								}
							}
							previous = role
						}
						if reissues["operator"].xid == reissues["admin"].xid || len(events) != 2+expectedChanges || userAuditCount(t, runner, p) != before+2+expectedChanges {
							t.Fatal("incoherent concurrent audits", events)
						}
						if err := runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
							var open, revoked int
							var xid string
							if err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE used_at IS NULL AND revoked_at IS NULL), count(*) FILTER(WHERE revoked_at IS NOT NULL) FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='invitation'`, p.TenantID, u.ID).Scan(&open, &revoked); err != nil {
								return err
							}
							if open != 1 || revoked != 2 {
								t.Errorf("open=%d revoked=%d", open, revoked)
							}
							if err := tx.QueryRow(ctx, `SELECT xmin::text FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='invitation' AND used_at IS NULL AND revoked_at IS NULL`, p.TenantID, u.ID).Scan(&xid); err != nil {
								return err
							}
							if xid != reissues[final].xid {
								t.Error("open invitation is not from last reissue")
							}
							return nil
						}); err != nil {
							t.Fatal(err)
						}
						return
					}
					if success != 1 && operation != "reactivate_deactivate" {
						t.Fatalf("outcomes: %v", results)
					}
					if userAuditCount(t, runner, p) != before+success {
						t.Fatal("successful operations must audit exactly once")
					}
					list, err := s.ListUsers(ctx, p)
					if err != nil {
						t.Fatal(err)
					}
					admins := 0
					for _, item := range list {
						if item.Status == "active" && item.Role == authz.RoleAdmin {
							admins++
						}
					}
					if admins < 1 {
						t.Fatal("no active admin")
					}
				})
			}
		})
	}
}

func TestAdministratorSelfDeactivationAndRestoration(t *testing.T) {
	s, r, p, _ := usersFixture(t)
	ctx := context.Background()
	u := inviteForTest(t, s, p, authz.RoleAdmin)
	accepted, err := s.AcceptInvitation(ctx, deliveredToken(t, r, p.TenantID, u.Email, "invitation"), "Second admin", "valid-password", RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	var firstSession SessionResult
	if err := r.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		var err error
		firstSession, err = s.CreateSession(ctx, tx, p, RequestMeta{})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	disabled, err := s.Deactivate(ctx, p, p.UserID, RequestMeta{})
	if err != nil || disabled.Status != "disabled" {
		t.Fatalf("self deactivation: %+v %v", disabled, err)
	}
	if _, err := s.ResolveSession(ctx, firstSession.RawToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal(err)
	}
	other := accepted.Principal
	if _, err := s.ChangeRole(ctx, other, u.ID, authz.RoleOperator, RequestMeta{}); !errors.Is(err, ErrLastAdmin) {
		t.Fatal("disabled administrator counted", err)
	}
	active, err := s.Reactivate(ctx, other, p.UserID, RequestMeta{})
	if err != nil || active.Status != "active" {
		t.Fatalf("restore administrator: %+v %v", active, err)
	}
	requireUserAudit(t, r, p, p.UserID, "user.reactivated", other.UserID, map[string]any{"to_status": "active"})
	if _, err := s.ChangeRole(ctx, other, u.ID, authz.RoleOperator, RequestMeta{}); err != nil {
		t.Fatal("restored administrator must count", err)
	}
	if _, err := s.ChangeRole(ctx, p, p.UserID, authz.RoleOperator, RequestMeta{}); !errors.Is(err, ErrLastAdmin) {
		t.Fatal(err)
	}
	if _, err := s.ResolveSession(ctx, firstSession.RawToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("reactivation reopened old session", err)
	}
}
