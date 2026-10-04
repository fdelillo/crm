//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/testsupport/apitest"
	"github.com/google/uuid"
)

func postAuth(t *testing.T, server *apitest.Server, path, body string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/v1/auth/"+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := server.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestPhase4LoginLockAndLogout(t *testing.T) {
	server := phase3Server(t)
	email := uuid.NewString() + "@example.com"
	signup := `{"name":"Ana","email":"` + email + `","password":"example-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`
	created := postSignup(t, server, signup, "application/json")
	_, _ = io.Copy(io.Discard, created.Body)
	_ = created.Body.Close()
	if created.StatusCode != 201 || len(created.Cookies()) != 1 {
		t.Fatalf("signup=%d cookies=%v", created.StatusCode, created.Cookies())
	}
	original := created.Cookies()[0]
	logout := postAuth(t, server, "logout", "", original)
	_, _ = io.Copy(io.Discard, logout.Body)
	_ = logout.Body.Close()
	if logout.StatusCode != 204 || len(logout.Cookies()) != 1 || logout.Cookies()[0].MaxAge >= 0 {
		t.Fatalf("logout=%d cookies=%v", logout.StatusCode, logout.Cookies())
	}
	meReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/api/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	meReq.AddCookie(original)
	me, err := server.Client.Do(meReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = me.Body.Close()
	if me.StatusCode != 401 {
		t.Fatalf("revoked cookie /me=%d", me.StatusCode)
	}
	loggedIn := postAuth(t, server, "login", `{"email":"`+email+`","password":"example-password"}`, nil)
	if loggedIn.StatusCode != 200 || len(loggedIn.Cookies()) != 1 {
		t.Fatalf("login=%d cookies=%v", loggedIn.StatusCode, loggedIn.Cookies())
	}
	var sessionInfo struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.NewDecoder(loggedIn.Body).Decode(&sessionInfo); err != nil {
		t.Fatal(err)
	}
	_ = loggedIn.Body.Close()
	if sessionInfo.User.Email != email {
		t.Fatalf("login user=%s", sessionInfo.User.Email)
	}
	secondLogout := postAuth(t, server, "logout", "", loggedIn.Cookies()[0])
	_ = secondLogout.Body.Close()
	if secondLogout.StatusCode != 204 {
		t.Fatalf("second logout=%d", secondLogout.StatusCode)
	}
	for i := 0; i < 5; i++ {
		resp := postAuth(t, server, "login", `{"email":"`+email+`","password":"wrong"}`, nil)
		_ = resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("bad login %d=%d", i, resp.StatusCode)
		}
	}
	locked := postAuth(t, server, "login", `{"email":"`+email+`","password":"example-password"}`, nil)
	_ = locked.Body.Close()
	if locked.StatusCode != 429 || locked.Header.Get("Retry-After") == "" {
		t.Fatalf("locked=%d retry=%s", locked.StatusCode, locked.Header.Get("Retry-After"))
	}
	// A different account can still use the same IP budget.
	unknown := postAuth(t, server, "login", `{"email":"`+uuid.NewString()+`@example.com","password":"example-password"}`, nil)
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(unknown.Body).Decode(&problem); err != nil {
		t.Fatal(err)
	}
	_ = unknown.Body.Close()
	if unknown.StatusCode != 401 || problem.Code != "invalid_credentials" {
		t.Fatalf("unknown=%d %+v", unknown.StatusCode, problem)
	}
	invalid := postAuth(t, server, "logout", "", &http.Cookie{Name: identity.SessionCookieName, Value: "invalid"})
	_ = invalid.Body.Close()
	if invalid.StatusCode != 204 {
		t.Fatalf("invalid logout=%d", invalid.StatusCode)
	}
}

func TestPhase4LoginIPRateLimit(t *testing.T) {
	server := phase3Server(t)
	for i := 0; i < 21; i++ {
		email := uuid.NewString() + "@example.com"
		resp := postAuth(t, server, "login", `{"email":"`+email+`","password":"wrong"}`, nil)
		_ = resp.Body.Close()
		want := 401
		if i == 20 {
			want = 429
		}
		if resp.StatusCode != want {
			t.Fatalf("attempt %d=%d want %d", i+1, resp.StatusCode, want)
		}
		if i == 20 && resp.Header.Get("Retry-After") == "" {
			t.Fatal("missing Retry-After")
		}
	}
}
