//go:build integration

package app_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/config"
	"github.com/google/uuid"
)

// invalidSignup is a body that answers 422 without creating a company (and so without creating a
// role of the cluster, DD-33 R-f).
const invalidSignup = `{"name":"","email":"","password":"x","company_name":"","base_currency":"","industry_template_code":""}`

// signupIP returns the i-th client address with its own rate-limit key: IPv4 addresses and /64
// prefixes of IPv6, all of the documentation ranges.
func signupIP(i int) string {
	if i%2 == 0 {
		return fmt.Sprintf("192.0.2.%d", i/2+1)
	}
	return fmt.Sprintf("2001:db8:0:%x::1", i/2+1)
}

// globalLimited reads signup_global_limited_total the way the operator does, from /debug/vars.
func globalLimited(t *testing.T) int64 {
	t.Helper()
	rec := httptest.NewRecorder()
	app.NewMetricsServer(config.Config{}).Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/debug/vars", nil))
	var values map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &values); err != nil {
		t.Fatal(err)
	}
	raw, ok := values["signup_global_limited_total"]
	if !ok {
		return -1
	}
	n, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		t.Fatalf("signup_global_limited_total=%s", raw)
	}
	return n
}

func (e *phase10Env) signup(ip, body string, want int) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.doFrom("", ip, http.MethodPost, "/api/v1/auth/signup", body, want)
}

// TestSignupGlobalQuota covers T-B1007 (DD-49). It creates companies only in G5, and does not run in
// parallel with other signups (T-B303 convention): the quota is process-wide state of its own router.
func TestSignupGlobalQuota(t *testing.T) {
	newEnv := func(t *testing.T) (*phase10Env, *bytes.Buffer) {
		logs := &bytes.Buffer{}
		return newPhase10EnvWithLogger(t, slog.New(slog.NewJSONHandler(logs, nil))), logs
	}
	exhaust := func(e *phase10Env, from int) {
		for i := from; i < 30; i++ {
			e.signup(signupIP(i), invalidSignup, 422)
		}
	}
	t.Run("G1, G2, G3 and G5 the global quota", func(t *testing.T) {
		e, logs := newEnv(t)
		before := globalLimited(t)
		exhaust(e, 0) // G1
		// G2: the body has an email, so the check below that the event does not carry it proves something.
		leaked := "leak-" + uuid.NewString() + "@example.test"
		rec := e.signup(signupIP(30), `{"name":"","email":"`+leaked+`","password":"x","company_name":"","base_currency":"","industry_template_code":""}`, 429)
		if seconds, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil || seconds <= 0 {
			t.Fatalf("Retry-After=%q", rec.Header().Get("Retry-After"))
		}
		if got := globalLimited(t) - before; got != 1 {
			t.Fatalf("signup_global_limited_total moved by %d, want 1", got)
		}
		var event map[string]any
		for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
			var entry map[string]any
			if json.Unmarshal(line, &entry) == nil && entry["event"] == "signup_global_limited" {
				event = entry
			}
		}
		if event == nil || event["level"] != "WARN" || event["ip"] != signupIP(30) {
			t.Fatalf("log event signup_global_limited: %v", event)
		}
		if strings.Contains(logs.String(), leaked) || strings.Contains(logs.String(), "leak-") {
			t.Fatal("the event carries an email")
		}
		// G3: one token per minute.
		e.clock.advance(time.Minute)
		e.signup(signupIP(31), invalidSignup, 422)
		e.signup(signupIP(32), invalidSignup, 429)
		// G5: with the quota refilled a valid signup is created as always.
		e.clock.advance(time.Minute)
		created := e.signup(signupIP(33), `{"name":"Ana","email":"`+uuid.NewString()+`@example.test","password":"valid-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`, 201)
		var body struct {
			Tenant struct {
				ID uuid.UUID `json:"id"`
			} `json:"tenant"`
		}
		if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		settleOutbox(t, e.runner, body.Tenant.ID)
	})
	t.Run("G4 a quota refusal by IP does not spend the global quota", func(t *testing.T) {
		e, _ := newEnv(t)
		before := globalLimited(t)
		for i := 0; i < 5; i++ {
			e.signup(signupIP(0), invalidSignup, 422)
		}
		e.signup(signupIP(0), invalidSignup, 429) // the sixth, by IP
		// 5 of the 30 tokens went to the address above; the 25 that remain are all there.
		exhaust(e, 5)
		e.signup(signupIP(30), invalidSignup, 429)
		if got := globalLimited(t) - before; got != 1 {
			t.Fatalf("signup_global_limited_total moved by %d, want 1", got)
		}
	})
}
