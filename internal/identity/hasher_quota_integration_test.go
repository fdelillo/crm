//go:build integration

package identity

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

// quotaHasher is a Hasher with ONE seat whose Slot methods wait for a gate that the test opens
// (T-B1003). It reports when an Acquire has to wait for the seat and when a Slot method reaches the
// gate, and it fails the test if a flow of identity uses the methods of the Hasher itself, which
// would wait for the semaphore with a connection already taken (INV-36). The derivation is real.
type quotaHasher struct {
	t       *testing.T
	real    password.Hasher
	seat    chan struct{}
	entered chan struct{} // a Slot method started waiting at the gate
	waiting chan struct{} // an Acquire is waiting for the seat

	mu       sync.Mutex
	gate     chan struct{} // nil: open
	slots    []*quotaSlot
	acquires atomic.Int32
}

func newQuotaHasher(t *testing.T) *quotaHasher {
	h := &quotaHasher{t: t, real: password.NewHasher(4), seat: make(chan struct{}, 1),
		entered: make(chan struct{}, 16), waiting: make(chan struct{}, 16)}
	t.Cleanup(h.openGate)
	return h
}

func (h *quotaHasher) closeGate() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.gate = make(chan struct{})
	for len(h.entered) > 0 {
		<-h.entered
	}
	for len(h.waiting) > 0 {
		<-h.waiting
	}
}
func (h *quotaHasher) openGate() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.gate != nil {
		close(h.gate)
		h.gate = nil
	}
}
func (h *quotaHasher) awaitGate() {
	h.mu.Lock()
	gate := h.gate
	h.mu.Unlock()
	select {
	case h.entered <- struct{}{}:
	default:
	}
	if gate != nil {
		select {
		case <-gate:
		case <-time.After(30 * time.Second):
			h.t.Error("quotaHasher: the gate was never opened")
		}
	}
}

func (h *quotaHasher) Acquire(ctx context.Context) (password.Slot, error) {
	h.acquires.Add(1)
	select {
	case h.seat <- struct{}{}:
	default:
		select {
		case h.waiting <- struct{}{}:
		default:
		}
		select {
		case h.seat <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	inner, err := h.real.Acquire(ctx)
	if err != nil {
		<-h.seat
		return nil, err
	}
	s := &quotaSlot{h: h, inner: inner}
	h.mu.Lock()
	h.slots = append(h.slots, s)
	h.mu.Unlock()
	return s, nil
}

func (h *quotaHasher) lastSlot() *quotaSlot {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.slots[len(h.slots)-1]
}

// The methods of the Hasher wait for the semaphore, so using them from identity is the bug of DD-45:
// they fail the test. They go through the gate, without a seat, so that the flow still finishes.
func (h *quotaHasher) Hash(ctx context.Context, plain string) (string, error) {
	h.t.Error("DD-45: identity called Hasher.Hash while it may hold a connection")
	h.awaitGate()
	return h.real.Hash(ctx, plain)
}
func (h *quotaHasher) Verify(ctx context.Context, plain, encoded string) (bool, bool, error) {
	h.t.Error("DD-45: identity called Hasher.Verify while it may hold a connection")
	h.awaitGate()
	return h.real.Verify(ctx, plain, encoded)
}
func (h *quotaHasher) VerifyDummy(ctx context.Context, plain string) {
	h.t.Error("DD-45: identity called Hasher.VerifyDummy while it may hold a connection")
	h.awaitGate()
	h.real.VerifyDummy(ctx, plain)
}

type quotaSlot struct {
	h                         *quotaHasher
	inner                     password.Slot
	once                      sync.Once
	hashes, verifies, dummies atomic.Int32
}

func (s *quotaSlot) Hash(plain string) (string, error) {
	s.hashes.Add(1)
	s.h.awaitGate()
	return s.inner.Hash(plain)
}
func (s *quotaSlot) Verify(plain, encoded string) (bool, bool, error) {
	s.verifies.Add(1)
	s.h.awaitGate()
	return s.inner.Verify(plain, encoded)
}
func (s *quotaSlot) VerifyDummy(plain string) {
	s.dummies.Add(1)
	s.h.awaitGate()
	s.inner.VerifyDummy(plain)
}
func (s *quotaSlot) calls() int32 { return s.hashes.Load() + s.verifies.Load() + s.dummies.Load() }
func (s *quotaSlot) Release() {
	s.once.Do(func() {
		s.inner.Release()
		<-s.h.seat
	})
}

type quotaEnv struct {
	t    *testing.T
	s    *Service
	r    db.TxRunner
	pool *pgxpool.Pool
	h    *quotaHasher
	ctx  context.Context
	now  *loginClock
}

// newQuotaEnv builds a Service over its own pool of 2 connections (pool_max_conns=2) and a
// one-seat hasher: with one login holding a connection and a second one waiting for the seat, the
// pool shows whether the waiting flow took the other connection.
func newQuotaEnv(t *testing.T) *quotaEnv {
	t.Helper()
	url := pgtest.AppURL(t)
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	pool, err := pgxpool.New(context.Background(), url+sep+"pool_max_conns=2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	clk := &loginClock{now: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)}
	runner := db.NewTxRunner(pool)
	h := newQuotaHasher(t)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	s := NewService(runner, outbox.NewEnqueuer(clk), clk, 24*time.Hour, 7*24*time.Hour,
		WithAuthentication(h, audit.NewRecorder(), []byte("0123456789abcdef0123456789abcdef"), logger))
	return &quotaEnv{t: t, s: s, r: runner, pool: pool, h: h, ctx: ctx, now: clk}
}

func (e *quotaEnv) await(ch chan struct{}, what string) {
	e.t.Helper()
	select {
	case <-ch:
	case <-e.ctx.Done():
		e.t.Fatalf("timed out waiting for %s", what)
	}
}

func (e *quotaEnv) login(ctx context.Context, email, plain string) chan error {
	done := make(chan error, 1)
	go func() { _, err := e.s.Login(ctx, email, plain, RequestMeta{}); done <- err }()
	return done
}

func (e *quotaEnv) result(done chan error) error {
	e.t.Helper()
	select {
	case err := <-done:
		return err
	case <-e.ctx.Done():
		e.t.Fatal("flow did not finish")
		return nil
	}
}

// validSession is a session of another company, created before the pool is put under pressure.
func (e *quotaEnv) validSession() string {
	e.t.Helper()
	other := fixture.NewCompany(e.t, pgtest.AppPool(e.t))
	var session SessionResult
	if err := e.r.InTenantTx(e.ctx, other.ID, func(ctx context.Context, tx db.Tx) error {
		var err error
		session, err = e.s.CreateSession(ctx, tx, authz.Principal{TenantID: other.ID, UserID: other.UserID, Role: authz.RoleAdmin}, RequestMeta{})
		return err
	}); err != nil {
		e.t.Fatal(err)
	}
	return session.RawToken
}

// company is a fixture company whose only user has a real password hash.
func (e *quotaEnv) company(plain string) (fixture.Company, string) {
	e.t.Helper()
	c := fixture.NewCompany(e.t, pgtest.AppPool(e.t))
	hash, err := e.h.real.Hash(e.ctx, plain)
	if err != nil {
		e.t.Fatal(err)
	}
	setLoginPassword(e.t, e.r, c, hash, "active")
	var email string
	if err := e.r.InTenantTx(e.ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, c.ID, c.UserID).Scan(&email)
	}); err != nil {
		e.t.Fatal(err)
	}
	return c, email
}

func (e *quotaEnv) failedCount(email string) int {
	e.t.Helper()
	n := 0
	if err := e.r.InSystemTx(e.ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		var count *int
		err := tx.QueryRow(ctx, `SELECT (SELECT failed_count FROM app.login_throttles WHERE email_hmac=$1)`, e.s.emailHMAC(email)).Scan(&count)
		if count != nil {
			n = *count
		} else {
			n = -1 // no row
		}
		return err
	}); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// holdSeat starts a login of an unknown email that takes the seat and the first connection and
// stops at the gate. It returns the channel with that login's result.
func (e *quotaEnv) holdSeat() chan error {
	e.t.Helper()
	e.h.closeGate()
	done := e.login(e.ctx, uuid.NewString()+"@example.com", "whatever-password")
	e.await(e.h.entered, "the first login at the gate")
	return done
}

// requireOneConnection checks, while a second flow waits for the seat, that only the first login
// holds a connection and that another company's session still resolves within 250 ms.
func (e *quotaEnv) requireOneConnection(raw string) {
	e.t.Helper()
	e.await(e.h.waiting, "the second flow waiting for the seat")
	if got := e.pool.Stat().AcquiredConns(); got != 1 {
		e.t.Errorf("INV-36: AcquiredConns()=%d while a flow waits for the seat, want 1", got)
	}
	ctx, cancel := context.WithTimeout(e.ctx, 250*time.Millisecond)
	defer cancel()
	if _, err := e.s.ResolveSession(ctx, raw); err != nil {
		e.t.Errorf("INV-36: a session of another company did not resolve within 250 ms: %v", err)
	}
}

func TestHasherSeatIsTakenBeforeTheConnection(t *testing.T) {
	t.Run("H1 login waits for the seat without a connection", func(t *testing.T) {
		e := newQuotaEnv(t)
		raw := e.validSession()
		first := e.holdSeat()
		secondEmail := uuid.NewString() + "@example.com"
		second := e.login(e.ctx, secondEmail, "whatever-password")
		e.requireOneConnection(raw)
		e.h.openGate()
		if err := e.result(first); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("L1: %v", err)
		}
		if err := e.result(second); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("L2: %v", err)
		}
		if got := e.failedCount(secondEmail); got != 1 {
			t.Errorf("throttle of L2: %d, want 1 failure", got)
		}
	})
	t.Run("H2 accepting an invitation waits for the seat without a connection", func(t *testing.T) {
		e := newQuotaEnv(t)
		raw := e.validSession()
		c, _ := e.company("correct-password")
		if err := e.r.InTenantTx(e.ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE app.users SET email_verified_at=now() WHERE tenant_id=$1 AND id=$2`, c.ID, c.UserID)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		admin := authz.Principal{TenantID: c.ID, UserID: c.UserID, SessionID: c.Rows["sessions"], Role: authz.RoleAdmin}
		email := uuid.NewString() + "@example.com"
		if _, _, err := e.s.Invite(e.ctx, admin, email, authz.RoleOperator, RequestMeta{}); err != nil {
			t.Fatal(err)
		}
		token := deliveredToken(t, e.r, c.ID, email, "invitation")
		first := e.holdSeat()
		second := make(chan error, 1)
		go func() {
			_, err := e.s.AcceptInvitation(e.ctx, token, "Invitee", "valid-password", RequestMeta{})
			second <- err
		}()
		e.requireOneConnection(raw)
		e.h.openGate()
		if err := e.result(first); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("L1: %v", err)
		}
		if err := e.result(second); err != nil {
			t.Errorf("acceptance: %v", err)
		}
	})
	t.Run("H3 confirming a reset waits for the seat without a connection", func(t *testing.T) {
		e := newQuotaEnv(t)
		raw := e.validSession()
		c, email := e.company("correct-password")
		if err := e.s.RequestPasswordReset(e.ctx, email, RequestMeta{}); err != nil {
			t.Fatal(err)
		}
		token := deliveredToken(t, e.r, c.ID, email, "password_reset")
		first := e.holdSeat()
		second := make(chan error, 1)
		go func() { second <- e.s.ConfirmPasswordReset(e.ctx, token, "valid-password", RequestMeta{}) }()
		e.requireOneConnection(raw)
		e.h.openGate()
		if err := e.result(first); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("L1: %v", err)
		}
		if err := e.result(second); err != nil {
			t.Errorf("reset: %v", err)
		}
	})
	t.Run("H4 a canceled waiter leaves without a connection, throttle row or audit", func(t *testing.T) {
		e := newQuotaEnv(t)
		c, email := e.company("correct-password")
		first := e.holdSeat()
		ctx, cancel := context.WithCancel(e.ctx)
		defer cancel()
		second := e.login(ctx, email, "wrong-password")
		e.await(e.h.waiting, "the second login waiting for the seat")
		if got := e.pool.Stat().AcquiredConns(); got != 1 {
			t.Errorf("INV-36: AcquiredConns()=%d while L2 waits, want 1", got)
		}
		cancel()
		if err := e.result(second); !errors.Is(err, context.Canceled) {
			t.Errorf("L2: got %v, want context.Canceled", err)
		}
		if got := e.failedCount(email); got != -1 {
			t.Errorf("a throttle row exists for the canceled login: %d", got)
		}
		var audits int
		if err := e.r.InTenantTx(e.ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND target_id=$2 AND action LIKE 'auth.login%'`, c.ID, c.UserID).Scan(&audits)
		}); err != nil {
			t.Fatal(err)
		}
		if audits != 0 {
			t.Errorf("the canceled login left %d audit entries", audits)
		}
		e.h.openGate()
		if err := e.result(first); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("L1: %v", err)
		}
	})
	t.Run("H5 a rehash uses one Slot for the verification and the new hash", func(t *testing.T) {
		e := newQuotaEnv(t)
		c, email := e.company("correct-password")
		salt := []byte("old-hash-salt!!!")
		derived := argon2.IDKey([]byte("correct-password"), salt, 2, 8192, 1, 32)
		old := fmt.Sprintf("$argon2id$v=19$m=8192,t=2,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived))
		setLoginPassword(t, e.r, c, old, "active")
		before := e.h.acquires.Load()
		if _, err := e.s.Login(e.ctx, email, "correct-password", RequestMeta{}); err != nil {
			t.Fatal(err)
		}
		if got := e.h.acquires.Load() - before; got != 1 {
			t.Fatalf("Acquire calls: %d, want 1", got)
		}
		slot := e.h.lastSlot()
		if slot.verifies.Load() != 1 || slot.hashes.Load() != 1 || slot.dummies.Load() != 0 {
			t.Errorf("slot calls: verify=%d hash=%d dummy=%d, want 1, 1, 0", slot.verifies.Load(), slot.hashes.Load(), slot.dummies.Load())
		}
		var stored string
		if err := e.r.InTenantTx(e.ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT password_hash FROM app.users WHERE tenant_id=$1 AND id=$2`, c.ID, c.UserID).Scan(&stored)
		}); err != nil {
			t.Fatal(err)
		}
		if stored == old || !strings.Contains(stored, "m=19456") {
			t.Errorf("the stored hash does not have the current parameters: %q", stored)
		}
	})
	t.Run("H6 a locked email waits for the seat without a connection and derives nothing", func(t *testing.T) {
		e := newQuotaEnv(t)
		raw := e.validSession()
		_, email := e.company("correct-password")
		for i := 0; i < 5; i++ {
			if _, err := e.s.Login(e.ctx, email, "wrong-password", RequestMeta{}); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("failure %d: %v", i+1, err)
			}
		}
		first := e.holdSeat()
		second := e.login(e.ctx, email, "correct-password")
		e.requireOneConnection(raw)
		e.h.openGate()
		if err := e.result(first); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("L1: %v", err)
		}
		var locked *LockedError
		if err := e.result(second); !errors.As(err, &locked) {
			t.Errorf("L2: got %v, want LockedError", err)
		}
		if n := e.h.lastSlot().calls(); n != 0 {
			t.Errorf("the locked login derived %d times, want 0", n)
		}
	})
}
