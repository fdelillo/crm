//go:build integration

package app_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

// T-B803 uses real system-role lookups, then checks effects through each company's role.
func TestIsolationUnknownCompanyFlows(t *testing.T) {
	for _, flow := range []string{"login", "reset", "invited_reset_request", "resolve_session"} {
		t.Run(flow, func(t *testing.T) {
			f := e2e.New(t, pgtest.AppPool(t))
			ctx := context.Background()
			before := f.Snapshot(t, f.A.ID)
			bBefore := f.Snapshot(t, f.B.ID)
			api := app.BuildAPIRouter(f.Users, f.Companies, f.Clock, f.Logger)
			h := isolationRoot(t, f, api)
			call := func(path string, body []byte, want int) {
				isolationRequest(t, h, ctx, f.A.Users["admin"].Cookie, "POST", path, "application/json", body, "", want)
			}
			switch flow {
			case "login":
				rec := isolationRequest(t, h, ctx, nil, "POST", "/api/v1/auth/login", "application/json", jsonBody(map[string]string{"email": f.B.Users["operator"].Email, "password": e2e.Password}), "", 200)
				assertSessionCompany(t, f, rec, f.B.ID)
				var cookie *http.Cookie
				for _, v := range rec.Result().Cookies() {
					if v.Name == identity.SessionCookieName {
						cookie = v
					}
				}
				me := isolationRequest(t, h, ctx, cookie, "GET", "/api/v1/me", "", nil, "", 200)
				if !bytes.Contains(me.Body.Bytes(), []byte(f.B.ID.String())) {
					t.Fatal("login /me did not resolve B")
				}
				assertNoCompanyData(t, me.Body.Bytes(), f.A)
				// Confirm persisted tenant_id of the newly created session, read as B.
				principal, err := f.Users.ResolveSession(ctx, cookie.Value)
				if err != nil {
					t.Fatal(err)
				}
				err = f.Runner.InTenantTx(ctx, f.B.ID, func(ctx context.Context, tx db.Tx) error {
					var exists bool
					var role, setting string
					var bound uuid.UUID
					if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.sessions WHERE tenant_id=$1 AND id=$2 AND user_id=$3 AND revoked_at IS NULL), current_user, current_setting('role'), app.current_tenant_id()`, f.B.ID, principal.SessionID, f.B.Users["operator"].ID).Scan(&exists, &role, &setting, &bound); err != nil {
						return err
					}
					if !exists || role != db.TenantRoleName(f.B.ID) || setting != role || bound != f.B.ID {
						t.Errorf("new session visible as B=%t role=%s setting=%s tenant=%s", exists, role, setting, bound)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			case "reset":
				call("/api/v1/auth/password-reset/confirm", jsonBody(map[string]string{"token": f.B.Reset, "password": e2e.NewPassword}), 204)
				after := f.Snapshot(t, f.B.ID)
				assertOtherUsersUnchanged(t, bBefore["users"], after["users"], f.B.Users["operator"].ID)
				if _, err := f.Users.ResolveSession(ctx, f.B.Users["operator"].Cookie.Value); err == nil {
					t.Fatal("B reset did not revoke B sessions")
				}
				if _, err := f.Users.Login(ctx, f.B.Users["operator"].Email, e2e.NewPassword, identity.RequestMeta{}); err != nil {
					t.Fatal("B password unchanged", err)
				}
				for _, key := range []string{"admin", "operator"} {
					p, err := f.Users.ResolveSession(ctx, f.A.Users[key].Cookie.Value)
					if err != nil || p.TenantID != f.A.ID {
						t.Fatalf("A session affected: %s %v", key, err)
					}
				}
			case "invited_reset_request":
				call("/api/v1/auth/password-reset", jsonBody(map[string]string{"email": f.B.Users["invited"].Email}), 202)
				raw := f.Token(t, f.B.ID, f.B.Users["invited"].Email, "invitation")
				if raw == f.B.Invitation {
					t.Fatal("invitation not reissued")
				}
				if _, err := f.Users.PreviewInvitation(ctx, f.B.Invitation); err == nil {
					t.Fatal("old invitation still valid")
				}
				preview, err := f.Users.PreviewInvitation(ctx, raw)
				if err != nil || preview.Email != f.B.Users["invited"].Email {
					t.Fatal("new invitation not bound to B", err)
				}
				err = f.Runner.InTenantTx(ctx, f.B.ID, func(ctx context.Context, tx db.Tx) error {
					var n int
					err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND action='user.invitation_reissued' AND target_id=$2 AND data->>'trigger'='password_reset_request'`, f.B.ID, f.B.Users["invited"].ID).Scan(&n)
					if err == nil && n != 1 {
						t.Errorf("invitation audit in B: got %d want 1", n)
					}
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
				after := f.Snapshot(t, f.B.ID)
				if after["users"] != bBefore["users"] || after["sessions"] != bBefore["sessions"] {
					t.Fatal("invited reset altered users/sessions")
				}
				if !strings.Contains(after["audit_log"], "user.invitation_reissued") {
					t.Fatal("missing B audit")
				}
				assertChanged(t, f, f.B.ID, bBefore, "user_tokens", "outbox_messages", "audit_log")
			case "resolve_session":
				p, err := f.Users.ResolveSession(ctx, f.B.Users["admin"].Cookie.Value)
				if err != nil || p.TenantID != f.B.ID || p.UserID != f.B.Users["admin"].ID || p.SessionID != f.B.Users["admin"].Principal.SessionID {
					t.Fatalf("ResolveSession B: %+v %v", p, err)
				}
			}
			assertUnchanged(t, f, f.A.ID, before)
		})
	}
}
