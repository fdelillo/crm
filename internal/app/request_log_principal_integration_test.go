//go:build integration

package app_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
)

// T-B912 (DD-12, INV-38): the request line carries tenant_id and user_id only for the requests that
// identity.Authenticate resolved. The expected ids come from the fixture, never from a response.
func TestRequestLogPrincipal(t *testing.T) {
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	e := newAPIEnvWithLogger(t, logger)
	// The root of newAPIEnv answers the SPA with the unavailable placeholder; this one serves a
	// plain page so the SPA row is an ordinary 200.
	e.root = app.NewRootHandler(app.RootDeps{
		API: app.BuildAPIRouter(e.users, e.companies, e.clock, logger), Liveness: app.LivenessHandler(),
		Readiness: app.ReadinessPlaceholder(),
		SPA:       http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("spa")) }),
	}, app.NewCommonMiddleware(logger, true, nil))

	operatorEmail := newEmail()
	operator := e.member(e.a, operatorEmail, authz.RoleOperator)
	// Every cookie value, session id and email that must never reach a line.
	secrets := []string{e.aRaw, e.bRaw, operator.RawToken, e.a.SessionID.String(), e.b.SessionID.String(),
		operator.Principal.SessionID.String(), operatorEmail}

	var recs []*httptest.ResponseRecorder
	record := func(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		recs = append(recs, rec)
		return requestLineFor(t, logs, rec)
	}
	plain := func(t *testing.T, raw, method, path string, want int) *httptest.ResponseRecorder {
		t.Helper()
		rec, req := e.request(raw, method, path, nil)
		e.root.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: status=%d want %d", method, path, rec.Code, want)
		}
		return rec
	}
	requireIDs := func(t *testing.T, name string, line map[string]any, tenantID, userID string) {
		t.Helper()
		if line["tenant_id"] != tenantID || line["user_id"] != userID {
			t.Errorf("%s: tenant_id=%v user_id=%v, want %s %s", name, line["tenant_id"], line["user_id"], tenantID, userID)
		}
	}
	requireNoIDs := func(t *testing.T, name string, line map[string]any) {
		t.Helper()
		for _, key := range []string{"tenant_id", "user_id"} {
			if v, present := line[key]; present {
				t.Errorf("%s: %s=%v, want the key absent", name, key, v)
			}
		}
	}

	t.Run("authenticated rows carry the ids of the session", func(t *testing.T) {
		e.t = t
		line := record(t, e.do(e.aRaw, "GET", "/api/v1/me", "", 200))
		requireIDs(t, "GET /me", line, e.a.TenantID.String(), e.a.UserID.String())

		line = record(t, e.do(operator.RawToken, "POST", "/api/v1/users/invitations", `{"email":"`+newEmail()+`","role":"operator"}`, 403))
		requireIDs(t, "403 of RequirePermission", line, operator.Principal.TenantID.String(), operator.Principal.UserID.String())
	})

	t.Run("429 of the invitation quota names the company", func(t *testing.T) {
		e.t = t
		ips := [2]string{ipOne, ipTwo}
		for i := 0; i < 10; i++ {
			e.doFrom(e.aRaw, ips[i%2], "POST", "/api/v1/users/invitations", `{"email":"`+newEmail()+`","role":"operator"}`, 201)
		}
		rec := e.doFrom(e.aRaw, ipOne, "POST", "/api/v1/users/invitations", `{"email":"`+newEmail()+`","role":"operator"}`, 429)
		line := record(t, rec)
		if line["route"] != "/api/v1/users/invitations" {
			t.Errorf("route=%v", line["route"])
		}
		requireIDs(t, "429", line, e.a.TenantID.String(), e.a.UserID.String())
	})

	t.Run("requests without a valid session have no ids", func(t *testing.T) {
		e.t = t
		revoked := e.bRaw
		record(t, e.do(revoked, "POST", "/api/v1/auth/logout", "", 204)) // revokes B's session
		for name, raw := range map[string]string{"no cookie": "", "unknown cookie": "not-a-session", "revoked session": revoked} {
			requireNoIDs(t, name, record(t, e.do(raw, "GET", "/api/v1/me", "", 401)))
		}
	})

	t.Run("public routes with a valid cookie have no ids", func(t *testing.T) {
		e.t = t
		loginRec := e.do(e.aRaw, "POST", "/api/v1/auth/login", `{"email":"`+operatorEmail+`","password":"`+e2e.Password+`"}`, 200)
		for _, c := range loginRec.Result().Cookies() {
			if c.Name == identity.SessionCookieName {
				secrets = append(secrets, c.Value) // the session the login created
			}
		}
		line := record(t, loginRec)
		requireNoIDs(t, "login", line)
		line = record(t, e.do(operator.RawToken, "POST", "/api/v1/auth/password-reset", `{"email":"`+operatorEmail+`"}`, 202))
		requireNoIDs(t, "password-reset", line)
		requireNoIDs(t, "logout", record(t, e.do(operator.RawToken, "POST", "/api/v1/auth/logout", "", 204)))
		requireNoIDs(t, "healthz", record(t, plain(t, e.aRaw, "GET", "/healthz", 200)))
		requireNoIDs(t, "spa", record(t, plain(t, e.aRaw, "GET", "/", 200)))
	})

	t.Run("no line leaks the cookie, the session id or the email", func(t *testing.T) {
		e.t = t
		for _, rec := range recs {
			line := requestLineFor(t, logs, rec)
			raw, err := jsonLine(line)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range secrets {
				if strings.Contains(raw, secret) {
					t.Errorf("request line contains %q: %s", secret, raw)
				}
			}
		}
	})
}

func jsonLine(line map[string]any) (string, error) {
	raw, err := json.Marshal(line)
	return string(raw), err
}
