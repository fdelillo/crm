//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/apitest"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

type usersHTTPClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *usersHTTPClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *usersHTTPClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func phase6Server(t *testing.T) (*apitest.Server, *usersHTTPClock, db.TxRunner) {
	t.Helper()
	c := &usersHTTPClock{now: time.Now().UTC()}
	runner := db.NewTxRunner(pgtest.AppPool(t))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := password.NewHasher(2)
	recorder := audit.NewRecorder()
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour, identity.WithAuthentication(h, recorder, []byte("0123456789abcdef0123456789abcdef"), logger))
	companies := tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, h, recorder, logger)
	r := app.NewAPIRouter()
	tenant.RegisterRoutes(r, companies, ratelimit.NewLimiter(rate.Every(12*time.Minute), 5, c, time.Hour), logger)
	app.RegisterAuthRoutes(r, users, companies, ratelimit.NewLimiter(rate.Every(3*time.Second), 20, c, time.Minute), logger)
	app.RegisterMeRoute(r, users, companies, logger)
	app.RegisterUserRoutes(r, users, companies, ratelimit.NewLimiter(rate.Every(3*time.Minute), 20, c, time.Hour), logger)
	root := app.NewRootHandler(app.RootDeps{API: r, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: app.SPAUnavailableHandler()}, app.NewCommonMiddleware(logger, true, nil))
	server := apitest.NewServer(t, root)
	contract.Default(t).Wrap(t, server.Client)
	return server, c, runner
}
func usersRequest(t *testing.T, s *apitest.Server, method, path, body string, want int) (map[string]any, *http.Response) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, s.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s: %d want %d: %v", method, path, resp.StatusCode, want, out)
	}
	return out, resp
}
func usersSignup(t *testing.T, s *apitest.Server) (uuid.UUID, uuid.UUID) {
	t.Helper()
	body, _ := usersRequest(t, s, "POST", "/api/v1/auth/signup", `{"name":"A","email":"`+uuid.NewString()+`@example.com","password":"valid-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`, 201)
	return uuid.MustParse(body["tenant"].(map[string]any)["id"].(string)), uuid.MustParse(body["user"].(map[string]any)["id"].(string))
}
func invitationHTTPToken(t *testing.T, r db.TxRunner, tenantID uuid.UUID, email string) string {
	t.Helper()
	var encoded []byte
	if err := r.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT payload FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template='invitation' ORDER BY created_at DESC,id DESC LIMIT 1`, tenantID, email).Scan(&encoded)
	}); err != nil {
		t.Fatal(err)
	}
	var payload map[string]string
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["token"] == "" {
		t.Fatal("missing invitation token")
	}
	return payload["token"]
}
func TestUsersHTTPInvitationLifecycleAndPermissions(t *testing.T) {
	s, _, runner := phase6Server(t)
	tenantID, adminID := usersSignup(t, s)
	adminCookies := s.Client.Jar.Cookies(mustURL(t, s.URL))
	email := uuid.NewString() + "@example.com"
	invited, _ := usersRequest(t, s, "POST", "/api/v1/users/invitations", `{"email":"`+email+`","role":"operator"}`, 201)
	first := invitationHTTPToken(t, runner, tenantID, email)
	if invited["name"] != nil || invited["status"] != "invited" || invited["invitation_expires_at"] == nil {
		t.Fatalf("invited: %v", invited)
	}
	usersRequest(t, s, "POST", "/api/v1/users/invitations", `{"email":"`+email+`","role":"operator"}`, 200)
	changed, _ := usersRequest(t, s, "POST", "/api/v1/users/invitations", `{"email":"`+email+`","role":"admin"}`, 200)
	if changed["role"] != "admin" {
		t.Fatal(changed)
	}
	usersRequest(t, s, "POST", "/api/v1/auth/invitations/preview", `{"token":"`+first+`"}`, 400)
	raw := invitationHTTPToken(t, runner, tenantID, email)
	preview, _ := usersRequest(t, s, "POST", "/api/v1/auth/invitations/preview", `{"token":"`+raw+`"}`, 200)
	if preview["role"] != "admin" || preview["tenant_name"] != "ACME" {
		t.Fatal(preview)
	}
	accepted, resp := usersRequest(t, s, "POST", "/api/v1/auth/invitations/accept", `{"token":"`+raw+`","name":"B","password":"valid-password"}`, 201)
	cookie := resp.Cookies()
	if len(cookie) != 1 || !cookie[0].Secure || !cookie[0].HttpOnly || cookie[0].SameSite != http.SameSiteLaxMode || cookie[0].Path != "/" || cookie[0].Domain != "" || cookie[0].Name != identity.SessionCookieName || cookie[0].MaxAge < 604790 {
		t.Fatalf("cookie: %v", cookie)
	}
	if accepted["user"].(map[string]any)["role"] != "admin" {
		t.Fatal(accepted)
	}
	me, _ := usersRequest(t, s, "GET", "/api/v1/me", "", 200)
	if me["user"].(map[string]any)["role"] != "admin" {
		t.Fatal(me)
	}
	usersRequest(t, s, "POST", "/api/v1/auth/invitations/accept", `{"token":"`+raw+`","name":"B","password":"valid-password"}`, 400)
	s.Client.Jar.SetCookies(mustURL(t, s.URL), adminCookies)
	taken, _ := usersRequest(t, s, "POST", "/api/v1/users/invitations", `{"email":"`+email+`","role":"operator"}`, 409)
	if taken["code"] != "email_taken" {
		t.Fatal(taken)
	}
	if _, exists := taken["suggested_action"]; exists {
		t.Fatal("email_taken suggested action")
	}
	email = uuid.NewString() + "@example.com"
	operator, _ := usersRequest(t, s, "POST", "/api/v1/users/invitations", `{"email":"`+email+`","role":"operator"}`, 201)
	id := operator["id"].(string)
	raw = invitationHTTPToken(t, runner, tenantID, email)
	usersRequest(t, s, "POST", "/api/v1/auth/invitations/accept", `{"token":"`+raw+`","name":"C","password":"valid-password"}`, 201)
	operatorCookies := s.Client.Jar.Cookies(mustURL(t, s.URL))
	var before, after int
	countState := func() int {
		t.Helper()
		var n int
		if err := runner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.audit_log WHERE tenant_id=$1)+(SELECT count(*) FROM app.users WHERE tenant_id=$1)+(SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1)+(SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1)`, tenantID).Scan(&n)
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before = countState()
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/v1/users", ""},
		{"POST", "/api/v1/users/invitations", `{"email":"new@example.com","role":"operator"}`},
		{"PUT", "/api/v1/users/not-a-uuid/role", `{"role":"admin"}`},
		{"POST", "/api/v1/users/" + uuid.NewString() + "/deactivate", ""},
		{"POST", "/api/v1/users/" + adminID.String() + "/reactivate", ""},
	} {
		problem, _ := usersRequest(t, s, tc.method, tc.path, tc.body, 403)
		if problem["code"] != "forbidden" {
			t.Fatal(problem)
		}
	}
	after = countState()
	if before != after {
		t.Fatal("operator requests mutated database")
	}
	s.Client.Jar.SetCookies(mustURL(t, s.URL), adminCookies)
	disabled, _ := usersRequest(t, s, "POST", "/api/v1/users/"+id+"/deactivate", "", 200)
	if disabled["status"] != "disabled" {
		t.Fatal(disabled)
	}
	s.Client.Jar.SetCookies(mustURL(t, s.URL), operatorCookies)
	usersRequest(t, s, "GET", "/api/v1/me", "", 401)
	s.Client.Jar.SetCookies(mustURL(t, s.URL), adminCookies)
	active, _ := usersRequest(t, s, "POST", "/api/v1/users/"+id+"/reactivate", "", 200)
	if active["status"] != "active" || active["invitation_expires_at"] != nil {
		t.Fatal(active)
	}
	usersRequest(t, s, "POST", "/api/v1/auth/login", `{"email":"`+email+`","password":"valid-password"}`, 200)
}
func TestUsersHTTPStatesAndNotFound(t *testing.T) {
	s, c, runner := phase6Server(t)
	tenantID, adminID := usersSignup(t, s)
	c.advance(time.Second) // Registration uses the database clock; later invitations use the fake clock.
	cookie := s.Client.Jar.Cookies(mustURL(t, s.URL))
	email := uuid.NewString() + "@example.com"
	invited, _ := usersRequest(t, s, "POST", "/api/v1/users/invitations", `{"email":"`+email+`","role":"operator"}`, 201)
	id := invited["id"].(string)
	expires := invited["invitation_expires_at"]
	changed, _ := usersRequest(t, s, "PUT", "/api/v1/users/"+id+"/role", `{"role":"admin"}`, 200)
	if changed["role"] != "admin" || changed["status"] != "invited" || changed["invitation_expires_at"] != expires {
		t.Fatal(changed)
	}
	for _, tc := range []struct{ method, path, body, code string }{
		{"PUT", "/api/v1/users/" + adminID.String() + "/role", `{"role":"operator"}`, "last_admin"},
		{"POST", "/api/v1/users/" + adminID.String() + "/deactivate", "", "last_admin"},
		{"POST", "/api/v1/users/" + adminID.String() + "/reactivate", "", "invalid_state"},
		{"POST", "/api/v1/users/" + id + "/reactivate", "", "invalid_state"},
	} {
		problem, _ := usersRequest(t, s, tc.method, tc.path, tc.body, 409)
		if problem["code"] != tc.code {
			t.Fatal(problem)
		}
	}
	usersRequest(t, s, "POST", "/api/v1/users/"+id+"/deactivate", "", 200)
	for _, tc := range []struct{ method, path, body string }{
		{"PUT", "/api/v1/users/" + id + "/role", `{"role":"operator"}`},
		{"POST", "/api/v1/users/" + id + "/deactivate", ""},
	} {
		problem, _ := usersRequest(t, s, tc.method, tc.path, tc.body, 409)
		if problem["code"] != "invalid_state" {
			t.Fatal(problem)
		}
	}
	reactivated, _ := usersRequest(t, s, "POST", "/api/v1/users/"+id+"/reactivate", "", 200)
	if reactivated["status"] != "invited" || reactivated["invitation_expires_at"] == nil {
		t.Fatal(reactivated)
	}
	// List covers all three states and preserves the expired invitation.
	c.advance(time.Second)
	disabledEmail := uuid.NewString() + "@example.com"
	disabledUser, _ := usersRequest(t, s, "POST", "/api/v1/users/invitations", `{"email":"`+disabledEmail+`","role":"operator"}`, 201)
	disabledID := disabledUser["id"].(string)
	usersRequest(t, s, "POST", "/api/v1/users/"+disabledID+"/deactivate", "", 200)
	// A real second tenant's ID, not a fabricated cross-tenant case.
	otherID, _ := usersSignup(t, s)
	var otherUser uuid.UUID
	if err := runner.InTenantTx(context.Background(), otherID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT id FROM app.users WHERE tenant_id=$1`, otherID).Scan(&otherUser)
	}); err != nil {
		t.Fatal(err)
	}
	s.Client.Jar.SetCookies(mustURL(t, s.URL), cookie)
	for _, operation := range []string{"role", "deactivate", "reactivate"} {
		var baseline map[string]any
		for _, target := range []string{otherUser.String(), uuid.NewString(), "not-a-uuid"} {
			method, body := "POST", ""
			if operation == "role" {
				method, body = "PUT", `{"role":"operator"}`
			}
			problem, _ := usersRequest(t, s, method, "/api/v1/users/"+target+"/"+operation, body, 404)
			delete(problem, "instance")
			if problem["code"] != "not_found" {
				t.Fatal(problem)
			}
			if baseline == nil {
				baseline = problem
			} else if !reflect.DeepEqual(baseline, problem) {
				t.Fatal("404 differs by resource existence or ID syntax")
			}
		}
	}
	// Expire the invitation by three days. Login renews the admin session after advancing ten days.
	c.advance(10 * 24 * time.Hour)
	var adminEmail string
	if err := runner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, tenantID, adminID).Scan(&adminEmail)
	}); err != nil {
		t.Fatal(err)
	}
	usersRequest(t, s, "POST", "/api/v1/auth/login", `{"email":"`+adminEmail+`","password":"valid-password"}`, 200)
	list, _ := usersRequest(t, s, "GET", "/api/v1/users", "", 200)
	items := list["items"].([]any)
	if len(items) != 3 || items[0].(map[string]any)["id"] != adminID.String() || items[1].(map[string]any)["id"] != id || items[2].(map[string]any)["id"] != disabledID {
		t.Fatal(list)
	}
	if items[1].(map[string]any)["invitation_expires_at"] != reactivated["invitation_expires_at"] {
		t.Fatal("expired invitation date lost")
	}
	if items[2].(map[string]any)["status"] != "disabled" || items[2].(map[string]any)["invitation_expires_at"] != nil {
		t.Fatal("disabled list entry", items[2])
	}
	if items[0].(map[string]any)["invitation_expires_at"] != nil {
		t.Fatal("active user invitation expiry")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestUsersHTTPValidationPreservesInvitation(t *testing.T) {
	s, c, r := phase6Server(t)
	tenantID, adminID := usersSignup(t, s)
	c.advance(time.Second)
	for _, tc := range []struct{ body, field, code string }{
		{`{"email":"not an email","role":"operator"}`, "email", "invalid_format"},
		{`{"email":"valid@example.com","role":"unknown"}`, "role", "invalid_value"},
	} {
		problem, _ := usersRequest(t, s, "POST", "/api/v1/users/invitations", tc.body, 422)
		validation := problem["errors"].([]any)[0].(map[string]any)
		if validation["field"] != tc.field || validation["code"] != tc.code {
			t.Fatal(problem)
		}
	}
	usersRequest(t, s, "PUT", "/api/v1/users/"+adminID.String()+"/role", `{"role":"unknown"}`, 422)
	email := uuid.NewString() + "@example.com"
	usersRequest(t, s, "POST", "/api/v1/users/invitations", `{"email":"`+email+`","role":"operator"}`, 201)
	raw := invitationHTTPToken(t, r, tenantID, email)
	for _, body := range []string{
		`{"token":"` + raw + `","name":"","password":"valid-password"}`,
		`{"token":"` + raw + `","name":"B","password":"short"}`,
	} {
		usersRequest(t, s, "POST", "/api/v1/auth/invitations/accept", body, 422)
		usersRequest(t, s, "POST", "/api/v1/auth/invitations/preview", `{"token":"`+raw+`"}`, 200)
	}
	usersRequest(t, s, "POST", "/api/v1/auth/invitations/accept", `{"token":"`+raw+`","name":"B","password":"valid-password"}`, 201)
}
