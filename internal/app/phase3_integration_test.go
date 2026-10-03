//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/apitest"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/time/rate"
)

func phase3Server(t *testing.T) *apitest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := db.NewTxRunner(pgtest.AppPool(t))
	c := clock.Real{}
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour)
	companies := tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, password.NewHasher(2), audit.NewRecorder(), logger)
	r := app.NewAPIRouter()
	industrytemplate.RegisterRoutes(r)
	tenant.RegisterRoutes(r, companies, ratelimit.NewLimiter(rate.Every(12*time.Minute), 5, c, time.Hour), logger)
	app.RegisterMeRoute(r, users, companies, logger)
	root := app.NewRootHandler(app.RootDeps{API: r, Liveness: app.LivenessHandler(),
		Readiness: app.ReadinessPlaceholder(), SPA: app.SPAUnavailableHandler()},
		app.NewCommonMiddleware(logger, true, nil))
	server := apitest.NewServer(t, root)
	contract.Default(t).Wrap(t, server.Client)
	return server
}

func TestPhase3SignupAndMe(t *testing.T) {
	server := phase3Server(t)
	email := "ana-" + uuid.NewString() + "@example.com"
	body := `{"name":"Ana","email":"` + email + `","password":"example-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`
	post := func(payload string) *http.Response {
		t.Helper()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/v1/auth/signup", strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := server.Client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	resp := post(body)
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		bytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("signup status=%d body=%s", resp.StatusCode, bytes)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache=%q", got)
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 || cookies[0].Name != identity.SessionCookieName || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].Path != "/" || cookies[0].Domain != "" || cookies[0].MaxAge != 604800 || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie attributes: %+v", cookies)
	}
	var result struct {
		User struct {
			ID            uuid.UUID `json:"id"`
			Email, Role   string
			EmailVerified bool `json:"email_verified"`
		} `json:"user"`
		Tenant struct {
			ID uuid.UUID `json:"id"`
		} `json:"tenant"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.User.Email != email || result.User.Role != "admin" || result.User.EmailVerified || len(result.Permissions) != 15 {
		t.Fatalf("signup response = %+v", result)
	}
	me, err := server.Client.Get(server.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	defer me.Body.Close()
	if me.StatusCode != 200 {
		bytes, _ := io.ReadAll(me.Body)
		t.Fatalf("me status=%d body=%s", me.StatusCode, bytes)
	}
	var meBody struct {
		User        struct{ Email, Role string } `json:"user"`
		Permissions []string                     `json:"permissions"`
	}
	if err := json.NewDecoder(me.Body).Decode(&meBody); err != nil {
		t.Fatal(err)
	}
	if meBody.User.Email != email || meBody.User.Role != "admin" || len(meBody.Permissions) != 15 {
		t.Fatalf("me response = %+v", meBody)
	}
	runner := db.NewTxRunner(pgtest.AppPool(t))
	err = runner.InTenantTx(context.Background(), result.Tenant.ID, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.users SET role='operator', email_verified_at=now()
			WHERE tenant_id=$1 AND id=$2`, result.Tenant.ID, result.User.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	me, err = server.Client.Get(server.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	var operator struct {
		User struct {
			Role          string `json:"role"`
			EmailVerified bool   `json:"email_verified"`
		} `json:"user"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(me.Body).Decode(&operator); err != nil {
		t.Fatal(err)
	}
	_ = me.Body.Close()
	if me.StatusCode != 200 || operator.User.Role != "operator" || !operator.User.EmailVerified || len(operator.Permissions) != 6 {
		t.Fatalf("operator me status=%d body=%+v", me.StatusCode, operator)
	}
	withoutCookie := &http.Client{Transport: server.Client.Transport}
	unauthenticated, err := withoutCookie.Get(server.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	_ = unauthenticated.Body.Close()
	if unauthenticated.StatusCode != 401 {
		t.Fatalf("me without cookie status=%d", unauthenticated.StatusCode)
	}
	duplicate := post(body)
	defer duplicate.Body.Close()
	if duplicate.StatusCode != 409 {
		bytes, _ := io.ReadAll(duplicate.Body)
		t.Fatalf("duplicate status=%d body=%s", duplicate.StatusCode, bytes)
	}
	canonical, err := io.ReadAll(duplicate.Body)
	if err != nil {
		t.Fatal(err)
	}
	var problem struct {
		Code            string `json:"code"`
		SuggestedAction string `json:"suggested_action"`
	}
	if err := json.Unmarshal(canonical, &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "email_already_registered" || problem.SuggestedAction != "password_reset" {
		t.Fatalf("duplicate problem=%+v", problem)
	}
	var expected map[string]any
	if err := json.Unmarshal(canonical, &expected); err != nil {
		t.Fatal(err)
	}
	delete(expected, "instance")
	for _, status := range []string{"invited", "disabled"} {
		err := runner.InTenantTx(context.Background(), result.Tenant.ID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE app.users SET status=$1 WHERE tenant_id=$2 AND id=$3`,
				status, result.Tenant.ID, result.User.ID)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		resp := post(body)
		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != 409 {
			t.Fatalf("duplicate %s status=%d err=%v", status, resp.StatusCode, err)
		}
		var actual map[string]any
		if err := json.Unmarshal(data, &actual); err != nil {
			t.Fatal(err)
		}
		delete(actual, "instance")
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("duplicate %s body differs: %s", status, data)
		}
	}
}

func postSignup(t *testing.T, server *apitest.Server, body, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		server.URL+"/api/v1/auth/signup", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := server.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestSignupHTTPValidationSizeAndRateLimit(t *testing.T) {
	server := phase3Server(t)
	email := uuid.NewString() + "@example.com"
	valid := `{"name":"Ana","email":"` + email + `","password":"example-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`
	for _, tc := range []struct {
		body, contentType string
		status            int
	}{
		{valid, "", 415},
		{strings.TrimSuffix(valid, "}") + `,"unexpected":true}`, "application/json", 400},
		{strings.Replace(valid, `"industry_template_code":"generic"`, `"industry_template_code":"missing"`, 1), "application/json", 422},
		{valid + strings.Repeat(" ", 65537-len(valid)), "application/json", 413},
		{valid + strings.Repeat(" ", 65536-len(valid)), "application/json", 201},
	} {
		resp := postSignup(t, server, tc.body, tc.contentType)
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != tc.status || resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("signup status=%d want=%d cache=%q body=%s", resp.StatusCode, tc.status,
				resp.Header.Get("Cache-Control"), body)
		}
	}
	resp := postSignup(t, server, valid, "application/json")
	if resp.StatusCode != 429 || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("sixth signup status=%d retry=%q", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
	_ = resp.Body.Close()
}

func TestSignupDuplicateBodyDoesNotRevealAccount(t *testing.T) {
	server := phase3Server(t)
	email := uuid.NewString() + "@example.com"
	first := `{"name":"Secret Person","email":"` + email + `","password":"example-password","company_name":"Private Company","base_currency":"USD","industry_template_code":"generic"}`
	resp := postSignup(t, server, first, "application/json")
	if resp.StatusCode != 201 {
		t.Fatalf("first signup status=%d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	duplicate := `{"name":"Different Person","email":"` + email + `","password":"example-password","company_name":"Another Company","base_currency":"ARS","industry_template_code":"generic"}`
	resp = postSignup(t, server, duplicate, "application/json")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 409 || strings.Contains(string(body), "Private Company") ||
		strings.Contains(string(body), "Secret Person") || strings.Contains(string(body), email) {
		t.Fatalf("duplicate response status=%d body=%s", resp.StatusCode, body)
	}
	var problem struct {
		Title           string `json:"title"`
		Detail          string `json:"detail"`
		SuggestedAction string `json:"suggested_action"`
	}
	if err := json.Unmarshal(body, &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Title != "Ya existe un usuario con ese email" ||
		problem.Detail != "Ya existe un usuario con ese email. ¿Querés recuperar la contraseña?" ||
		problem.SuggestedAction != "password_reset" {
		t.Fatalf("duplicate problem=%+v", problem)
	}
}

func TestSignupLockTimeoutReturnsRetryable503(t *testing.T) {
	server := phase3Server(t)
	ctx := context.Background()
	super := pgtest.SuperuserPool(t)
	role := "crm_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := super.Exec(ctx, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = super.Exec(context.Background(), `DROP ROLE `+pgx.Identifier{role}.Sanitize()) })
	lock, err := super.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Rollback(context.Background()) }()
	if _, err := lock.Exec(ctx, `GRANT crm_tenant TO `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	email := uuid.NewString() + "@example.com"
	input := `{"name":"Ana","email":"` + email + `","password":"example-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`
	before := tenant.SignupLockTimeoutCount()
	resp := postSignup(t, server, input, "application/json")
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(data, &problem); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 503 || problem.Code != "service_unavailable" ||
		resp.Header.Get("Retry-After") != "2" || resp.Header.Get("Cache-Control") != "no-store" ||
		len(resp.Cookies()) != 0 || tenant.SignupLockTimeoutCount() != before+1 {
		t.Fatalf("lock response status=%d code=%s retry=%s cookies=%d metric=%d", resp.StatusCode,
			problem.Code, resp.Header.Get("Retry-After"), len(resp.Cookies()), tenant.SignupLockTimeoutCount()-before)
	}
	if err := lock.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	resp = postSignup(t, server, input, "application/json")
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("signup after lock released status=%d", resp.StatusCode)
	}
}
