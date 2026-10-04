//go:build integration

package audit_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/platform/securetoken"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func TestRecordTransactionIsolationAndPrivileges(t *testing.T) {
	a := fixture.NewCompany(t, pgtest.AppPool(t))
	b := fixture.NewCompany(t, pgtest.AppPool(t))
	runner := db.NewTxRunner(pgtest.AppPool(t))
	recorder := audit.NewRecorder()
	var requestCtx context.Context
	httpx.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requestCtx = r.Context() })).ServeHTTP(
		httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	requestID := httpx.RequestIDFrom(requestCtx)
	if requestID == "" {
		t.Fatal("missing request id")
	}
	entry := audit.Entry{TenantID: a.ID, Action: "user.role_changed", TargetType: "user", TargetID: &a.UserID,
		ActorUserID: &a.UserID, Data: map[string]any{"role": "operator"}}
	err := runner.InTenantTx(requestCtx, a.ID, func(ctx context.Context, tx db.Tx) error { return recorder.Record(ctx, tx, entry) })
	if err != nil {
		t.Fatal(err)
	}
	err = runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
		var count int
		var gotRequestID string
		if err := tx.QueryRow(ctx, `SELECT count(*), max(request_id) FROM app.audit_log
   WHERE tenant_id = $1 AND action = 'user.role_changed'`, a.ID).Scan(&count, &gotRequestID); err != nil {
			return err
		}
		if count != 1 || gotRequestID != requestID {
			t.Errorf("count=%d request_id=%q", count, gotRequestID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
		wrong := entry
		wrong.TenantID = b.ID
		return recorder.Record(ctx, tx, wrong)
	})
	if err == nil {
		t.Fatal("cross-tenant audit inserted")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("cross-tenant error=%v", err)
	}

	abort := errors.New("abort")
	err = runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
		entry.Action = "user.invited"
		if err := recorder.Record(ctx, tx, entry); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("rollback error=%v", err)
	}
	err = runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id = $1 AND action = 'user.invited'`, a.ID).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			t.Errorf("rolled back rows=%d", count)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE app.audit_log SET action = 'user.invited' WHERE tenant_id = $1`,
		`DELETE FROM app.audit_log WHERE tenant_id = $1`,
	} {
		err := runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, statement, a.ID)
			return err
		})
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("append-only error=%v", err)
		}
	}
}

// recordAndCommit runs Record inside a valid InTenantTx(tenant) with a valid Entry and the given
// data, IGNORES Record's error and lets the transaction COMMIT. That way "0 new rows" proves Record
// itself inserted nothing, not that the operation rolled back (T-B209, DD-37).
func recordAndCommit(t *testing.T, tenant, actor uuid.UUID, data map[string]any) error {
	t.Helper()
	var recordErr error
	err := db.NewTxRunner(pgtest.AppPool(t)).InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		recordErr = audit.NewRecorder().Record(ctx, tx, audit.Entry{TenantID: tenant, ActorUserID: &actor,
			Action: "test.recorded", TargetType: "user", Data: data})
		return nil
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
	return recordErr
}

func countAudit(t *testing.T, tenant uuid.UUID) int {
	t.Helper()
	var count int
	err := db.NewTxRunner(pgtest.AppPool(t)).InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id = $1 AND action = 'test.recorded'`, tenant).Scan(&count)
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

// DD-37 / INV-32: credential keys are rejected whatever their case or separators, the error names
// the key and never the value, and nothing is inserted.
func TestRecordRejectsCredentialKeys(t *testing.T) {
	company := fixture.NewCompany(t, pgtest.AppPool(t))
	keys := []string{"password", "new_password", "passwd", "passphrase", "token", "reset_token", "Authorization",
		"proxy-authorization", "Cookie", "Set-Cookie", "secret", "client_secret", "api_key", "X-API-Key", "apiKey",
		"private_key", "credentials", "signature", "csrf_token", "X-CSRF-Token", "xsrf", "auth", "session",
		"session_id", "SID", "otp", "pin"}
	for i, key := range keys {
		t.Run(key, func(t *testing.T) {
			value := fmt.Sprintf("valor-secreto-de-prueba-%d", i)
			err := recordAndCommit(t, company.ID, company.UserID, map[string]any{key: value})
			if !errors.Is(err, audit.ErrSecretInData) {
				t.Fatalf("err=%v", err)
			}
			if !strings.Contains(err.Error(), key) || strings.Contains(err.Error(), value) {
				t.Fatalf("error must name the key and never the value: %v", err)
			}
		})
	}
	if n := countAudit(t, company.ID); n != 0 {
		t.Fatalf("rows=%d", n)
	}
}

func TestRecordRejectsNestedCredentialKeysWithPath(t *testing.T) {
	company := fixture.NewCompany(t, pgtest.AppPool(t))
	const value = "valor-secreto-de-prueba-nested"
	cases := []struct {
		name string
		data map[string]any
		path string
	}{
		{"map[string]string", map[string]any{"headers": map[string]string{"Authorization": value}}, "headers.Authorization"},
		{"map[string]any", map[string]any{"headers": map[string]any{"Set-Cookie": value}}, "headers.Set-Cookie"},
		{"in a list", map[string]any{"items": []any{map[string]any{"api_key": value}}}, "items[0].api_key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := recordAndCommit(t, company.ID, company.UserID, tc.data)
			if !errors.Is(err, audit.ErrSecretInData) {
				t.Fatalf("err=%v", err)
			}
			if !strings.Contains(err.Error(), tc.path) || strings.Contains(err.Error(), value) {
				t.Fatalf("want path %q without the value: %v", tc.path, err)
			}
		})
	}
	if n := countAudit(t, company.ID); n != 0 {
		t.Fatalf("rows=%d", n)
	}
}

func TestRecordRejectsCredentialValues(t *testing.T) {
	company := fixture.NewCompany(t, pgtest.AppPool(t))
	raw, _, err := securetoken.New()
	if err != nil {
		t.Fatal(err)
	}
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTYifQ.c2lnbmF0dXJlLXZhbHVl"
	cases := []struct {
		name string
		data map[string]any
		leak string
	}{
		{"bearer", map[string]any{"note": "Bearer abc.def"}, "abc.def"},
		{"basic", map[string]any{"note": "basic dXNlcjpwYXNz"}, "dXNlcjpwYXNz"},
		{"digest with spaces", map[string]any{"note": "  Digest username=x"}, "username=x"},
		{"jwt", map[string]any{"note": jwt}, jwt},
		{"securetoken raw", map[string]any{"note": raw}, raw},
		{"securetoken in a []string", map[string]any{"fields": []string{"name", raw}}, raw},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := recordAndCommit(t, company.ID, company.UserID, tc.data)
			if !errors.Is(err, audit.ErrSecretInData) {
				t.Fatalf("err=%v", err)
			}
			if strings.Contains(err.Error(), tc.leak) {
				t.Fatalf("error leaked the value: %v", err)
			}
		})
	}
	if n := countAudit(t, company.ID); n != 0 {
		t.Fatalf("rows=%d", n)
	}
}

func TestRecordRejectsEmbeddedCredentialsWithoutInserting(t *testing.T) {
	company := fixture.NewCompany(t, pgtest.AppPool(t))
	raw, _, err := securetoken.New()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, path string
		data       map[string]any
	}{
		{"reset link", "note", map[string]any{"note": "see https://crm.example/reset-password#token=" + raw}},
		{"nested invitation link", "items[0].note", map[string]any{"items": []any{map[string]any{"note": "/accept-invitation#token=" + raw}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := recordAndCommit(t, company.ID, company.UserID, tc.data)
			if !errors.Is(err, audit.ErrSecretInData) {
				t.Fatalf("error = %v", err)
			}
			if !strings.Contains(err.Error(), tc.path) || strings.Contains(err.Error(), raw) || strings.Contains(err.Error(), "token=") || strings.Contains(err.Error(), "://") {
				t.Fatalf("error leaked value or omitted path: %v", err)
			}
		})
	}
	if count := countAudit(t, company.ID); count != 0 {
		t.Fatalf("rows after committed transactions = %d", count)
	}
}

func TestRecordRejectsUnsupportedTypes(t *testing.T) {
	company := fixture.NewCompany(t, pgtest.AppPool(t))
	text := "a"
	cases := map[string]any{
		"struct":          struct{ A string }{},
		"[]byte":          []byte("a"),
		"json.RawMessage": json.RawMessage("{}"),
		"*string":         &text,
		"map[string]int":  map[string]int{},
		"float64":         float64(1),
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			err := recordAndCommit(t, company.ID, company.UserID, map[string]any{"note": value})
			if !errors.Is(err, audit.ErrUnsupportedData) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if n := countAudit(t, company.ID); n != 0 {
		t.Fatalf("rows=%d", n)
	}
}

// The catalog of data-model.md §2.6 and the allowed leaf types must never trip the policy.
func TestRecordAcceptsCatalogDataAndAllowedTypes(t *testing.T) {
	company := fixture.NewCompany(t, pgtest.AppPool(t))
	cases := []map[string]any{
		{"industry_template_code": "generic"},
		{"fields": []string{"name", "timezone"}},
		{"content_type": "image/png"},
		{"reason": "bad_password"},
		{"sessions_revoked": int64(2)},
		{"role": "operator", "trigger": "password_reset_request"},
		{"from": "admin", "to": "operator", "status": "invited"},
		{"to_status": "active"},
		{"flag": true, "n": 1, "n32": int32(2), "id": uuid.New(), "at": time.Now(), "nothing": nil},
	}
	for i, data := range cases {
		if err := recordAndCommit(t, company.ID, company.UserID, data); err != nil {
			t.Fatalf("case %d %v: %v", i, data, err)
		}
	}
	if n := countAudit(t, company.ID); n != len(cases) {
		t.Fatalf("rows=%d want %d", n, len(cases))
	}
}

func TestRecordWithoutTransaction(t *testing.T) {
	recorder := audit.NewRecorder()
	if err := recorder.Record(context.Background(), nil, audit.Entry{Data: map[string]any{"note": "ok"}}); !errors.Is(err, audit.ErrTxRequired) {
		t.Fatalf("err=%v", err)
	}
	// Validation runs before the transaction is looked at: a credential is reported as such.
	if err := recorder.Record(context.Background(), nil, audit.Entry{Data: map[string]any{"password": "x"}}); !errors.Is(err, audit.ErrSecretInData) {
		t.Fatalf("err=%v", err)
	}
}

// DD-38 / INV-33: any User-Agent is stored without failing the operation.
func TestRecordNormalizesUserAgent(t *testing.T) {
	cases := []struct {
		name, in string
		runes    int
		want     string // "" = only checked through runes
		null     bool
	}{
		{"513 ASCII", strings.Repeat("a", 513), 512, strings.Repeat("a", 512), false},
		{"600 two-byte runes", strings.Repeat("ñ", 600), 512, strings.Repeat("ñ", 512), false},
		{"invalid UTF-8", "Mozilla/5.0 \xff\xfe x", 0, "Mozilla/5.0 \uFFFD\uFFFD x", false},
		{"NUL", "a\x00b", 0, "ab", false},
		{"empty", "", 0, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const action = "test.user_agent"
			company := fixture.NewCompany(t, pgtest.AppPool(t))
			err := db.NewTxRunner(pgtest.AppPool(t)).InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
				return audit.NewRecorder().Record(ctx, tx, audit.Entry{TenantID: company.ID, Action: action, UserAgent: tc.in})
			})
			if err != nil {
				t.Fatalf("Record failed: %v", err)
			}
			var got *string
			var chars int
			err = db.NewTxRunner(pgtest.AppPool(t)).InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
				return tx.QueryRow(ctx, `SELECT user_agent, coalesce(char_length(user_agent), 0) FROM app.audit_log
   WHERE tenant_id = $1 AND action = $2`, company.ID, action).Scan(&got, &chars)
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.null {
				if got != nil {
					t.Fatalf("user_agent=%q want NULL", *got)
				}
				return
			}
			if got == nil || !utf8.ValidString(*got) || *got != tc.want {
				t.Fatalf("user_agent=%v want %q", got, tc.want)
			}
			if tc.runes != 0 && chars != tc.runes {
				t.Fatalf("char_length=%d want %d", chars, tc.runes)
			}
		})
	}
}
