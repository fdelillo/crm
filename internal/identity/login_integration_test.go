//go:build integration

package identity

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

type loginClock struct{ now time.Time }

func (c *loginClock) Now() time.Time { return c.now }

func loginFixture(t *testing.T, h password.Hasher) (*Service, db.TxRunner, fixture.Company, string, *loginClock, *bytes.Buffer) {
	t.Helper()
	company := fixture.NewCompany(t, pgtest.AppPool(t))
	runner := db.NewTxRunner(pgtest.AppPool(t))
	clock := &loginClock{now: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)}
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	svc := NewService(runner, outbox.NewEnqueuer(clock), clock, 24*time.Hour, 7*24*time.Hour,
		WithAuthentication(h, audit.NewRecorder(), []byte("0123456789abcdef0123456789abcdef"), logger))
	var email string
	err := runner.InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, company.ID, company.UserID).Scan(&email)
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, runner, company, email, clock, logs
}

func setLoginPassword(t *testing.T, runner db.TxRunner, company fixture.Company, hash, status string) {
	t.Helper()
	err := runner.InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.users SET password_hash=$1, status=$2 WHERE tenant_id=$3 AND id=$4`, hash, status, company.ID, company.UserID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoginServiceSuccessFailureLockAndLogout(t *testing.T) {
	hasher := password.NewHasher(2)
	svc, runner, company, email, clock, _ := loginFixture(t, hasher)
	hash, err := hasher.Hash(context.Background(), "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	setLoginPassword(t, runner, company, hash, "active")
	for i := 0; i < 5; i++ {
		_, err := svc.Login(context.Background(), email, "wrong", RequestMeta{})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("failure %d: %v", i+1, err)
		}
	}
	_, err = svc.Login(context.Background(), email, "correct-password", RequestMeta{})
	var locked *LockedError
	if !errors.As(err, &locked) || locked.RetryAfter != 15*time.Minute {
		t.Fatalf("locked: %v", err)
	}
	clock.now = clock.now.Add(15*time.Minute + time.Second)
	session, err := svc.Login(context.Background(), "  "+strings.ToUpper(email)+"  ", "correct-password", RequestMeta{})
	if err != nil || session.Principal.UserID != company.UserID || session.ExpiresAt != clock.now.Add(7*24*time.Hour) {
		t.Fatalf("success: %+v %v", session, err)
	}
	if err := svc.Logout(context.Background(), session.RawToken, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(context.Background(), session.RawToken, RequestMeta{}); err != nil {
		t.Fatalf("idempotent logout: %v", err)
	}
	if _, err := svc.ResolveSession(context.Background(), session.RawToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("revoked session: %v", err)
	}
	if _, err := svc.Login(context.Background(), email, "wrong", RequestMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("counter not cleared: %v", err)
	}
	var failed, lockedAudits, succeeded, logouts int
	err = runner.InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE action='auth.login_failed'), count(*) FILTER (WHERE action='auth.login_locked'), count(*) FILTER (WHERE action='auth.login_succeeded'), count(*) FILTER (WHERE action='auth.logout') FROM app.audit_log WHERE tenant_id=$1`, company.ID).Scan(&failed, &lockedAudits, &succeeded, &logouts)
	})
	if err != nil || failed != 6 || lockedAudits != 1 || succeeded != 1 || logouts != 1 {
		t.Fatalf("audit: %d %d %d %d: %v", failed, lockedAudits, succeeded, logouts, err)
	}
	var correctlyShaped int
	err = runner.InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND
			(action='auth.login_failed' AND actor_user_id IS NULL AND target_type='user' AND target_id=$2 AND data->>'reason'='bad_password'
			 OR action='auth.login_locked' AND actor_user_id IS NULL AND target_type='user' AND target_id=$2 AND data='{}'::jsonb
			 OR action='auth.login_succeeded' AND actor_user_id=$2 AND target_type='session' AND target_id=$3 AND data='{}'::jsonb
			 OR action='auth.logout' AND actor_user_id=$2 AND target_type='session' AND target_id=$3 AND data='{}'::jsonb)`,
			company.ID, company.UserID, session.Principal.SessionID).Scan(&correctlyShaped)
	})
	if err != nil || correctlyShaped != 9 {
		t.Fatalf("audit fields: rows=%d err=%v", correctlyShaped, err)
	}
}

func TestLoginServiceNonexistentAndDisabled(t *testing.T) {
	hasher := password.NewHasher(2)
	svc, runner, company, email, _, logs := loginFixture(t, hasher)
	hash, err := hasher.Hash(context.Background(), "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	setLoginPassword(t, runner, company, hash, "disabled")
	if _, err := svc.Login(context.Background(), email, "wrong", RequestMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong disabled: %v", err)
	}
	if _, err := svc.Login(context.Background(), email, "correct-password", RequestMeta{}); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("correct disabled: %v", err)
	}
	if !strings.Contains(logs.String(), `"security_event":"login_disabled"`) {
		t.Fatal("correct password for disabled user lacks security event")
	}
	missing := uuid.NewString() + "@example.com"
	for i := 0; i < 5; i++ {
		if _, err := svc.Login(context.Background(), missing, "wrong", RequestMeta{}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("missing %d: %v", i, err)
		}
	}
	if _, err := svc.Login(context.Background(), missing, "wrong", RequestMeta{}); err == nil {
		t.Fatal("missing account was not locked")
	} else {
		var locked *LockedError
		if !errors.As(err, &locked) {
			t.Fatalf("missing lock: %v", err)
		}
	}
	setLoginPassword(t, runner, company, hash, "invited")
	if _, err := svc.Login(context.Background(), email, "correct-password", RequestMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("invited: %v", err)
	}
}

func TestLoginSecuritySignalsIncludeUnknownEmailsWithoutLeakingThem(t *testing.T) {
	svc, _, company, email, _, logs := loginFixture(t, &countingHasher{})
	missing := uuid.NewString() + "@example.com"
	meta := RequestMeta{IP: netip.MustParseAddr("192.0.2.42"), RequestID: "request-42"}
	failedBefore, lockedBefore := loginFailedTotal.Value(), loginLockedTotal.Value()
	for _, candidate := range []string{email, missing} {
		for i := 0; i < 5; i++ {
			if _, err := svc.Login(context.Background(), candidate, "wrong", meta); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("%s failure %d: %v", candidate, i+1, err)
			}
		}
		var locked *LockedError
		if _, err := svc.Login(context.Background(), candidate, "wrong", meta); !errors.As(err, &locked) {
			t.Fatalf("%s locked attempt: %v", candidate, err)
		}
	}
	if got := loginFailedTotal.Value() - failedBefore; got != 10 {
		t.Fatalf("login_failed_total delta=%d want 10", got)
	}
	if got := loginLockedTotal.Value() - lockedBefore; got != 2 {
		t.Fatalf("login_locked_total delta=%d want 2", got)
	}
	if strings.Contains(logs.String(), email) || strings.Contains(logs.String(), missing) {
		t.Fatal("security log contains a plaintext email")
	}
	knownKey := hex.EncodeToString(svc.emailHMAC(email))
	unknownKey := hex.EncodeToString(svc.emailHMAC(missing))
	counts := map[string]map[string]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatal(err)
		}
		key, _ := entry["email_hmac"].(string)
		if key != knownKey && key != unknownKey {
			t.Fatalf("unexpected email_hmac %q", key)
		}
		if entry["ip"] != "192.0.2.42" || entry["request_id"] != "request-42" {
			t.Fatalf("missing IP or request ID: %v", entry)
		}
		// An already-locked attempt stops before the user lookup, so its tenant is unknown
		// even if the HMAC corresponds to an existing account.
		blockedBeforeLookup := entry["security_event"] == "login_locked" && entry["lock_started"] != true
		if key == knownKey && !blockedBeforeLookup && entry["tenant_id"] != company.ID.String() {
			t.Fatalf("resolved account lacks tenant_id: %v", entry)
		}
		if key == unknownKey {
			if _, hasTenant := entry["tenant_id"]; hasTenant {
				t.Fatalf("unknown account was assigned a tenant: %v", entry)
			}
		}
		event, _ := entry["security_event"].(string)
		if counts[key] == nil {
			counts[key] = map[string]int{}
		}
		counts[key][event]++
	}
	for _, key := range []string{knownKey, unknownKey} {
		if counts[key]["login_failed"] != 5 || counts[key]["login_locked"] != 2 {
			t.Fatalf("events for %s: %v", key, counts[key])
		}
	}
}

type rejectedAudit struct{}

func (rejectedAudit) Record(context.Context, db.Tx, audit.Entry) error {
	return errors.New("forced audit failure")
}

func TestRolledBackLoginDoesNotEmitSecuritySignal(t *testing.T) {
	svc, _, _, email, _, logs := loginFixture(t, &countingHasher{})
	svc.audit = rejectedAudit{}
	failedBefore, lockedBefore := loginFailedTotal.Value(), loginLockedTotal.Value()
	if _, err := svc.Login(context.Background(), email, "wrong", RequestMeta{}); err == nil || errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("login should fail with audit error: %v", err)
	}
	if logs.Len() != 0 || loginFailedTotal.Value() != failedBefore || loginLockedTotal.Value() != lockedBefore {
		t.Fatalf("rolled-back attempt emitted a signal: logs=%q failed=%d locked=%d",
			logs.String(), loginFailedTotal.Value()-failedBefore, loginLockedTotal.Value()-lockedBefore)
	}
}

type countingHasher struct{ verifies atomic.Int32 }

func (h *countingHasher) Hash(context.Context, string) (string, error) { return "unused", nil }
func (h *countingHasher) Verify(context.Context, string, string) (bool, bool, error) {
	h.verifies.Add(1)
	return false, false, nil
}
func (h *countingHasher) VerifyDummy(context.Context, string) {}

func TestConcurrentLoginVerifiesOnlyFivePasswords(t *testing.T) {
	h := &countingHasher{}
	svc, _, _, email, _, _ := loginFixture(t, h)
	var wg sync.WaitGroup
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Login(context.Background(), email, "wrong", RequestMeta{})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	invalid, locked := 0, 0
	for err := range results {
		var lock *LockedError
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			invalid++
		case errors.As(err, &lock):
			locked++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if invalid != 5 || locked != 5 || h.verifies.Load() != 5 {
		t.Fatalf("invalid=%d locked=%d verifies=%d", invalid, locked, h.verifies.Load())
	}
}

func TestLoginRehashesOldPasswordParameters(t *testing.T) {
	hasher := password.NewHasher(2)
	svc, runner, company, email, _, _ := loginFixture(t, hasher)
	salt := []byte("old-hash-salt!!!")
	plain := "correct-password"
	derived := argon2.IDKey([]byte(plain), salt, 2, 8192, 1, 32)
	old := fmt.Sprintf("$argon2id$v=19$m=8192,t=2,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived))
	setLoginPassword(t, runner, company, old, "active")
	if _, err := svc.Login(context.Background(), email, plain, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	var stored string
	err := runner.InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT password_hash FROM app.users WHERE tenant_id=$1 AND id=$2`, company.ID, company.UserID).Scan(&stored)
	})
	if err != nil || stored == old || !strings.Contains(stored, "m=19456") {
		t.Fatalf("rehash=%q err=%v", stored, err)
	}
}
