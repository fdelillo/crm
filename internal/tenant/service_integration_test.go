//go:build integration

package tenant_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/platform/securetoken"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func newRegistrationServices(t *testing.T) (*tenant.Service, *identity.Service, db.TxRunner) {
	t.Helper()
	runner := db.NewTxRunner(pgtest.AppPool(t))
	c := clock.Real{}
	ident := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour)
	svc := tenant.NewService(runner, ident, industrytemplate.NoopSeeder{}, password.NewHasher(2),
		audit.NewRecorder(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, ident, runner
}

func TestRegisterIsAtomicAndAuditsOnlyCatalogData(t *testing.T) {
	svc, ident, runner := newRegistrationServices(t)
	email := "  ANA-" + uuid.NewString() + "@example.com  "
	ua := strings.Repeat("a", 600) + "\xff\x00"
	meta := identity.RequestMeta{IP: netip.MustParseAddr("192.0.2.42"), UserAgent: ua}
	input := tenant.Signup{Name: "Ana", Email: email, Password: "a safe password 123",
		CompanyName: "Empresa", BaseCurrency: "ARS", IndustryTemplateCode: "generic"}
	var result tenant.Registration
	var err error
	var requestID string
	httpx.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		requestID = httpx.RequestIDFrom(r.Context())
		result, err = svc.Register(r.Context(), input, meta)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Email != strings.ToLower(strings.TrimSpace(email)) || result.Session.RawToken == "" || result.User.Role != "admin" {
		t.Fatalf("unexpected registration result: %+v", result.User)
	}
	var dataText, auditAction, auditTargetType, auditIP, auditRequestID string
	var auditActor, auditTarget uuid.UUID
	var auditUA, sessionUA *string
	var outboxPayload []byte
	var storedSessionHash []byte
	var storedEmail, passwordHash string
	var tokenCount, messageCount, auditCount int
	err = runner.InTenantTx(context.Background(), result.Tenant.ID, func(ctx context.Context, tx db.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT email, password_hash FROM app.users WHERE tenant_id=$1 AND id=$2`, result.Tenant.ID, result.User.ID).Scan(&storedEmail, &passwordHash); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT token_hash, user_agent FROM app.sessions WHERE tenant_id=$1 AND id=$2`, result.Tenant.ID, result.Session.Principal.SessionID).Scan(&storedSessionHash, &sessionUA); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='email_verification'`, result.Tenant.ID, result.User.ID).Scan(&tokenCount); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1 AND template='email_verification'`, result.Tenant.ID).Scan(&messageCount); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT payload FROM app.outbox_messages WHERE tenant_id=$1 AND template='email_verification'`, result.Tenant.ID).Scan(&outboxPayload); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id=$1`, result.Tenant.ID).Scan(&auditCount); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT action, actor_user_id, target_type, target_id, data::text,
			host(ip), user_agent, request_id FROM app.audit_log WHERE tenant_id=$1`, result.Tenant.ID).
			Scan(&auditAction, &auditActor, &auditTargetType, &auditTarget, &dataText,
				&auditIP, &auditUA, &auditRequestID)
	})
	if err != nil {
		t.Fatal(err)
	}
	if storedEmail != result.User.Email || !strings.HasPrefix(passwordHash, "$argon2id$") || string(storedSessionHash) == result.Session.RawToken || string(storedSessionHash) != string(securetoken.Hash(result.Session.RawToken)) {
		t.Fatal("stored credentials do not match the required hash/normalization")
	}
	if tokenCount != 1 || messageCount != 1 || auditCount != 1 {
		t.Fatalf("token=%d message=%d audit=%d", tokenCount, messageCount, auditCount)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(dataText), &data); err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 || data["industry_template_code"] != "generic" {
		t.Fatalf("audit data=%v", data)
	}
	if auditAction != "tenant.registered" || auditActor != result.User.ID || auditTargetType != "tenant" ||
		auditTarget != result.Tenant.ID || auditIP != meta.IP.String() || auditRequestID != requestID {
		t.Fatalf("audit metadata: action=%s actor=%s target=%s/%s ip=%s request=%s",
			auditAction, auditActor, auditTargetType, auditTarget, auditIP, auditRequestID)
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(outboxPayload, &payload); err != nil || payload.Token == "" {
		t.Fatalf("verification payload missing token: %v", err)
	}
	for _, secret := range []string{result.Session.RawToken, payload.Token, input.Password, result.User.Email} {
		if strings.Contains(dataText, secret) || auditUA != nil && strings.Contains(*auditUA, secret) {
			t.Fatal("audit contains registration secret or email")
		}
	}
	wantUA := httpx.NormalizeUserAgent(ua)
	if auditUA == nil || sessionUA == nil || *auditUA != wantUA || *sessionUA != wantUA {
		t.Fatalf("user agent not normalized identically")
	}
	principal, err := ident.ResolveSession(context.Background(), result.Session.RawToken)
	if err != nil || principal.UserID != result.User.ID {
		t.Fatalf("resolve session: %+v %v", principal, err)
	}
	_, err = svc.Register(context.Background(), input, meta)
	if !errors.Is(err, identity.ErrEmailTaken) {
		t.Fatalf("duplicate registration error=%v", err)
	}
}

type failingSeeder struct{ tenantID uuid.UUID }

func (s *failingSeeder) Seed(ctx context.Context, tx db.Tx, id uuid.UUID, _ industrytemplate.Template) error {
	s.tenantID = id
	var timeout string
	if err := tx.QueryRow(ctx, `SELECT current_setting('lock_timeout')`).Scan(&timeout); err != nil {
		return err
	}
	if timeout != "2s" {
		return errors.New("registration transaction has no 2s lock timeout")
	}
	return errors.New("seeder failed")
}

func signupWithEmail(email string) tenant.Signup {
	return tenant.Signup{Name: "Ana", Email: email, Password: "a safe password 123",
		CompanyName: "Empresa", BaseCurrency: "ARS", IndustryTemplateCode: "generic"}
}

func TestRegisterSeederRollbackIncludesRole(t *testing.T) {
	runner := db.NewTxRunner(pgtest.AppPool(t))
	c := clock.Real{}
	ident := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour)
	seeder := &failingSeeder{}
	svc := tenant.NewService(runner, ident, seeder, password.NewHasher(2), audit.NewRecorder(),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := svc.Register(context.Background(), signupWithEmail(uuid.NewString()+"@example.com"), identity.RequestMeta{})
	if err == nil || seeder.tenantID == uuid.Nil {
		t.Fatalf("expected seeder error and captured tenant: %v %s", err, seeder.tenantID)
	}
	var roles, tenants, users, sessions, tokens, messages int
	err = pgtest.SuperuserPool(t).QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM pg_roles WHERE rolname=$1),
		(SELECT count(*) FROM app.tenants WHERE id=$2),
		(SELECT count(*) FROM app.users WHERE tenant_id=$2),
		(SELECT count(*) FROM app.sessions WHERE tenant_id=$2),
		(SELECT count(*) FROM app.user_tokens WHERE tenant_id=$2),
		(SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$2)`,
		db.TenantRoleName(seeder.tenantID), seeder.tenantID).Scan(&roles, &tenants, &users, &sessions, &tokens, &messages)
	if err != nil || roles+tenants+users+sessions+tokens+messages != 0 {
		t.Fatalf("rollback: role=%d tenant=%d users=%d sessions=%d tokens=%d messages=%d err=%v",
			roles, tenants, users, sessions, tokens, messages, err)
	}
}

func TestRegisterTemplatesTimezoneAndUserAgent(t *testing.T) {
	svc, _, runner := newRegistrationServices(t)
	for _, template := range industrytemplate.All() {
		for _, timezone := range []struct{ input, want string }{
			{"America/Argentina/Cordoba", "America/Argentina/Cordoba"},
			{"Asia/Kathmandu", "Asia/Kathmandu"},
			{"Marte/Olympus", "America/Argentina/Buenos_Aires"},
			{"", "America/Argentina/Buenos_Aires"},
		} {
			input := signupWithEmail(uuid.NewString() + "@example.com")
			input.IndustryTemplateCode = template.Code
			input.Timezone = timezone.input
			got, err := svc.Register(context.Background(), input, identity.RequestMeta{})
			if err != nil || got.Tenant.Timezone != timezone.want {
				t.Fatalf("template=%s timezone=%s: got=%s err=%v", template.Code, timezone.input, got.Tenant.Timezone, err)
			}
			var storedCode, storedTimezone string
			var storedVersion int
			var sessionUA, auditUA *string
			err = runner.InTenantTx(context.Background(), got.Tenant.ID, func(ctx context.Context, tx db.Tx) error {
				return tx.QueryRow(ctx, `SELECT t.industry_template_code, t.industry_template_version, t.timezone,
					s.user_agent, a.user_agent FROM app.tenants t JOIN app.sessions s ON s.tenant_id=t.id
					JOIN app.audit_log a ON a.tenant_id=t.id WHERE t.id=$1`, got.Tenant.ID).
					Scan(&storedCode, &storedVersion, &storedTimezone, &sessionUA, &auditUA)
			})
			if err != nil || storedCode != template.Code || storedVersion != template.Version || storedTimezone != timezone.want || sessionUA != nil || auditUA != nil {
				t.Fatalf("persisted template/timezone/UA: code=%s version=%d timezone=%s sessionUA=%v auditUA=%v err=%v",
					storedCode, storedVersion, storedTimezone, sessionUA, auditUA, err)
			}
		}
	}
}

func TestRegisterConcurrentDistinctAndDuplicateEmails(t *testing.T) {
	svc, _, _ := newRegistrationServices(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errorsByAttempt := make(chan error, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Register(ctx, signupWithEmail(uuid.NewString()+"@example.com"), identity.RequestMeta{})
			errorsByAttempt <- err
		}()
	}
	wg.Wait()
	close(errorsByAttempt)
	for err := range errorsByAttempt {
		if err != nil {
			t.Fatalf("distinct concurrent registration: %v", err)
		}
	}
	email := uuid.NewString() + "@example.com"
	errorsByAttempt = make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Register(ctx, signupWithEmail(email), identity.RequestMeta{})
			errorsByAttempt <- err
		}()
	}
	wg.Wait()
	close(errorsByAttempt)
	var ok, taken int
	for err := range errorsByAttempt {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, identity.ErrEmailTaken):
			taken++
		default:
			t.Fatalf("same-email concurrent registration: %v", err)
		}
	}
	if ok != 1 || taken != 1 {
		t.Fatalf("same-email outcomes: success=%d taken=%d", ok, taken)
	}
}

func TestRegisterLockTimeoutIsUnavailableAndRetryWorks(t *testing.T) {
	svc, _, _ := newRegistrationServices(t)
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
	input := signupWithEmail(uuid.NewString() + "@example.com")
	start := time.Now()
	_, err = svc.Register(ctx, input, identity.RequestMeta{})
	elapsed := time.Since(start)
	if !errors.Is(err, db.ErrUnavailable) || elapsed < 2*time.Second || elapsed >= 4*time.Second {
		t.Fatalf("lock timeout: err=%v elapsed=%s", err, elapsed)
	}
	if err := lock.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, input, identity.RequestMeta{}); err != nil {
		t.Fatalf("retry after lock release: %v", err)
	}
}

func TestRegisterLocalTimezoneDefaultsAndLogs(t *testing.T) {
	_, ident, runner := newRegistrationServices(t)
	var logs bytes.Buffer
	svc := tenant.NewService(runner, ident, industrytemplate.NoopSeeder{}, password.NewHasher(2), audit.NewRecorder(), slog.New(slog.NewTextHandler(&logs, nil)))
	input := signupWithEmail(uuid.NewString() + "@example.com")
	input.Timezone = "Local"
	got, err := svc.Register(context.Background(), input, identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Tenant.Timezone != "America/Argentina/Buenos_Aires" {
		t.Fatalf("DD-27: stored timezone=%q", got.Tenant.Timezone)
	}
	if !strings.Contains(logs.String(), "level=INFO") || !strings.Contains(logs.String(), "event=signup_timezone_defaulted") {
		t.Fatalf("default log missing: %s", logs.String())
	}
	stored, err := svc.Get(context.Background(), got.Session.Principal)
	if err != nil || stored.Timezone != got.Tenant.Timezone {
		t.Fatalf("persisted timezone=%s err=%v", stored.Timezone, err)
	}
}
