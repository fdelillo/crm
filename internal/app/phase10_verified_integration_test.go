//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/google/uuid"
)

const resendPath = "/api/v1/auth/email-verification/resend"

// emailCount counts the verification messages queued for an address.
func (e *phase10Env) emailCount(tenantID uuid.UUID, email string) int {
	e.t.Helper()
	var n int
	if err := e.runner.TxRunner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template='email_verification'`, tenantID, email).Scan(&n)
	}); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// openTokens counts the verification links of a user that are neither used nor revoked, and all of them.
func (e *phase10Env) tokens(tenantID, userID uuid.UUID) (open, total int) {
	e.t.Helper()
	if err := e.runner.TxRunner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE used_at IS NULL AND revoked_at IS NULL), count(*) FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='email_verification'`, tenantID, userID).Scan(&open, &total)
	}); err != nil {
		e.t.Fatal(err)
	}
	return open, total
}

func (e *phase10Env) emailOf(p authz.Principal) string {
	e.t.Helper()
	var email string
	if err := e.runner.TxRunner.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, p.TenantID, p.UserID).Scan(&email)
	}); err != nil {
		e.t.Fatal(err)
	}
	return email
}

func (e *phase10Env) requireProblem(rec *httptest.ResponseRecorder, retryAfter string) {
	e.t.Helper()
	if got := rec.Header().Get("Retry-After"); got != retryAfter {
		e.t.Fatalf("Retry-After=%q, want %q", got, retryAfter)
	}
}

func (e *phase10Env) resend(raw string, ipIndex int, want int) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.doFrom(raw, signupIP(ipIndex), http.MethodPost, resendPath, "", want)
}

func TestInviteEmailNotVerifiedHTTP(t *testing.T) {
	t.Run("V1 and V2 an unverified administrator gets 403 email_not_verified", func(t *testing.T) {
		e := newPhase10Env(t)
		c, _ := e.newUnverifiedCompany()
		other := e.newCompany()
		before := e.footprint(c.a.TenantID)
		rec := e.doFrom(c.aRaw, ipOne, "POST", "/api/v1/users/invitations", `{"email":"`+newEmail()+`","role":"operator"}`, 403)
		requireCode(t, rec.Body.Bytes(), "email_not_verified")
		// V2: the email of a user of another company still answers email_not_verified, not email_taken.
		rec = e.doFrom(c.aRaw, ipOne, "POST", "/api/v1/users/invitations", `{"email":"`+e.emailOf(other.a)+`","role":"operator"}`, 403)
		requireCode(t, rec.Body.Bytes(), "email_not_verified")
		if after := e.footprint(c.a.TenantID); after != before {
			t.Fatalf("refused invitations left rows: %v -> %v", before, after)
		}
	})
	t.Run("V7 the audit scenario: the owner recovers a company that someone else registered", func(t *testing.T) {
		e := newPhase10Env(t)
		ctx := context.Background()
		victim := "v-" + uuid.NewString() + "@example.test"
		signup := `{"name":"Attacker","email":"` + victim + `","password":"valid-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`
		created := e.signup(signupIP(40), signup, 201)
		attackerRaw := cookieOf(t, created)
		tenantID := tenantIDOf(t, created)
		settleOutbox(t, e.runner, tenantID)
		// The registrant cannot invite an administrator of their own.
		e.doFrom(attackerRaw, ipOne, "POST", "/api/v1/users/invitations", `{"email":"x-`+newEmail()+`","role":"admin"}`, 403)
		// The real owner of the address registers and is told the address is taken.
		e.signup(signupIP(41), signup, 409)
		// ...and follows the reset: it is a password reset request by email, then the confirmation.
		e.doFrom("", signupIP(42), "POST", "/api/v1/auth/password-reset", `{"email":"`+victim+`"}`, 202)
		token := e.token(tenantID, victim, "password_reset")
		e.doFrom("", signupIP(42), "POST", "/api/v1/auth/password-reset/confirm", `{"token":"`+token+`","password":"owner-new-password"}`, 204)
		// The registrant's session no longer resolves, and the company only has the owner.
		if _, err := e.users.ResolveSession(ctx, attackerRaw); err == nil {
			t.Fatal("the session of the registrant survived the reset")
		}
		login := e.doFrom("", signupIP(43), "POST", "/api/v1/auth/login", `{"email":"`+victim+`","password":"owner-new-password"}`, 200)
		owner := cookieOf(t, login)
		principal, err := e.users.ResolveSession(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		users, err := e.users.ListUsers(ctx, principal)
		if err != nil || len(users) != 1 || users[0].Email != victim || !users[0].EmailVerified {
			t.Fatalf("users: %+v %v", users, err)
		}
		// The reset verified the address, so the owner can invite.
		e.doFrom(owner, signupIP(44), "POST", "/api/v1/users/invitations", `{"email":"`+newEmail()+`","role":"operator"}`, 201)
	})
}

// TestResendEmailVerificationQuota covers T-B1009 V8a to V8i (DD-46 (3), amendment H-16).
func TestResendEmailVerificationQuota(t *testing.T) {
	t.Run("V8a to V8e the quota is per user, with the Retry-After of its next token", func(t *testing.T) {
		e := newPhase10Env(t)
		c := e.newCompany() // A (founder, U1), B (a verified member)
		u2 := e.member(c.a, "u2-"+newEmail(), authz.RoleOperator)
		e.unverify(c.a.TenantID, c.a.UserID, u2.Principal.UserID)
		u1Email, u2Email := e.emailOf(c.a), e.emailOf(u2.Principal)
		tenantID := c.a.TenantID
		mails := e.emailCount(tenantID, u1Email)
		// V8a: three requests from three addresses.
		for i := 0; i < 3; i++ {
			e.resend(c.aRaw, 1+i, 202)
		}
		if got := e.emailCount(tenantID, u1Email) - mails; got != 3 {
			t.Fatalf("V8a queued %d messages, want 3", got)
		}
		open, total := e.tokens(tenantID, c.a.UserID)
		// V8b: the fourth, from a new address.
		rec := e.resend(c.aRaw, 4, 429)
		requireCode(t, rec.Body.Bytes(), "rate_limited")
		e.requireProblem(rec, "1200")
		if o, n := e.tokens(tenantID, c.a.UserID); o != open || n != total || open != 1 {
			t.Fatalf("V8b: open=%d total=%d, want open=%d total=%d and one open link", o, n, open, total)
		}
		if got := e.emailCount(tenantID, u1Email) - mails; got != 3 {
			t.Fatalf("V8b queued a message: %d", got)
		}
		// V8c: five minutes later, another address.
		e.clock.advance(5 * time.Minute)
		rec = e.resend(c.aRaw, 5, 429)
		e.requireProblem(rec, "900")
		// V8d: another user of the same company, from the address of V8c, has its own quota.
		u2Mails := e.emailCount(tenantID, u2Email)
		e.resend(u2.RawToken, 5, 202)
		if got := e.emailCount(tenantID, u2Email) - u2Mails; got != 1 {
			t.Fatalf("V8d queued %d messages for U2, want 1", got)
		}
		// V8e: fifteen minutes more (20 since V8a): one token is back.
		e.clock.advance(15 * time.Minute)
		e.resend(c.aRaw, 6, 202)
		if got := e.emailCount(tenantID, u1Email) - mails; got != 4 {
			t.Fatalf("V8e: %d messages, want 4", got)
		}
		rec = e.resend(c.aRaw, 7, 429)
		e.requireProblem(rec, "1200")
		// V8i: no refusal revoked the link of the last 202.
		last := e.token(tenantID, u1Email, "email_verification")
		e.doFrom("", signupIP(8), "POST", "/api/v1/auth/email-verification/confirm", `{"token":"`+last+`"}`, 204)
	})
	t.Run("V8f and V8g the IP quota runs first and a refusal by IP does not spend the user quota", func(t *testing.T) {
		e := newPhase10Env(t)
		c, email := e.newUnverifiedCompany()
		x, y := signupIP(20), signupIP(21)
		for i := 0; i < 20; i++ {
			e.doFrom("", x, "POST", "/api/v1/auth/email-verification/confirm", `{"token":"not-a-token"}`, 400)
		}
		mails := e.emailCount(c.a.TenantID, email)
		rec := e.doFrom(c.aRaw, x, "POST", resendPath, "", 429)
		e.requireProblem(rec, "180")
		if e.emailCount(c.a.TenantID, email) != mails {
			t.Fatal("V8f: a message was queued")
		}
		for i := 0; i < 3; i++ {
			e.doFrom(c.aRaw, y, "POST", resendPath, "", 202)
		}
		if got := e.emailCount(c.a.TenantID, email) - mails; got != 3 {
			t.Fatalf("V8f: %d messages, want 3", got)
		}
		rec = e.doFrom(c.aRaw, y, "POST", resendPath, "", 429)
		e.requireProblem(rec, "1200")
		// V8g: both quotas are spent; the one of the IP answers, with its own Retry-After.
		rec = e.doFrom(c.aRaw, x, "POST", resendPath, "", 429)
		e.requireProblem(rec, "180")
		e.clock.advance(3 * time.Minute)
		rec = e.doFrom(c.aRaw, x, "POST", resendPath, "", 429)
		e.requireProblem(rec, "1020")
	})
	t.Run("V8h every authenticated request counts, even those that send nothing", func(t *testing.T) {
		e := newPhase10Env(t)
		u4 := e.member(e.a, newEmail(), authz.RoleOperator) // active, with a verified email
		before := e.emailCount(e.a.TenantID, e.emailOf(u4.Principal))
		for i := 0; i < 3; i++ {
			e.resend(u4.RawToken, 10+i, 202)
		}
		if e.emailCount(e.a.TenantID, e.emailOf(u4.Principal)) != before {
			t.Fatal("a verified user got a message")
		}
		rec := e.resend(u4.RawToken, 13, 429)
		requireCode(t, rec.Body.Bytes(), "rate_limited")
	})
}

func requireCode(t *testing.T, body []byte, want string) {
	t.Helper()
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &problem); err != nil || problem.Code != want {
		t.Fatalf("problem code: %s (%v), want %s", body, err, want)
	}
}

func cookieOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == identity.SessionCookieName && c.Value != "" {
			return c.Value
		}
	}
	t.Fatal("no session cookie in the response")
	return ""
}

func tenantIDOf(t *testing.T, rec *httptest.ResponseRecorder) uuid.UUID {
	t.Helper()
	var body struct {
		Tenant struct {
			ID uuid.UUID `json:"id"`
		} `json:"tenant"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Tenant.ID
}
