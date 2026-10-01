//go:build integration

package outbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

type fakeHandler struct {
	mu       sync.Mutex
	messages []Message
	err      error
	wait     bool
}

func (f *fakeHandler) Handle(ctx context.Context, m Message) error {
	if f.wait {
		<-ctx.Done()
		return ctx.Err()
	}
	f.mu.Lock()
	f.messages = append(f.messages, m)
	err := f.err
	f.mu.Unlock()
	return err
}
func (f *fakeHandler) Count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.messages) }
func newFixture(t *testing.T) (db.TxRunner, uuid.UUID, *fakeHandler, *Dispatcher) {
	t.Helper()
	c := cleanCompany(t)
	runner := db.NewTxRunner(pgtest.AppPool(t))
	fake := &fakeHandler{}
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)
	dispatcher := NewDispatcher(runner, fake, fakeClock{now}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return runner, c.ID, fake, dispatcher
}

func cleanCompany(t *testing.T) fixture.Company {
	t.Helper()
	c := fixture.NewCompany(t, pgtest.AppPool(t))
	runner := db.NewTxRunner(pgtest.AppPool(t))
	clearPending := func() error {
		return runner.InTenantTx(context.Background(), c.ID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE app.outbox_messages SET status = 'sent', payload = NULL, sent_at = now()
				WHERE tenant_id = $1 AND status = 'pending'`, c.ID)
			return err
		})
	}
	if err := clearPending(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := clearPending(); err != nil {
			t.Errorf("clean outbox for tenant %s: %v", c.ID, err)
		}
	})
	return c
}
func enqueue(t *testing.T, runner db.TxRunner, tenant uuid.UUID, template string) {
	t.Helper()
	err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		return NewEnqueuer().Enqueue(ctx, tx, Message{TenantID: tenant, Kind: "email", Template: template,
			Recipient: "user@example.com", Payload: map[string]string{"link": "https://example.com/#token=secret"}})
	})
	if err != nil {
		t.Fatal(err)
	}
}
func state(t *testing.T, runner db.TxRunner, tenant uuid.UUID, template string) (status string, attempts int, payloadPresent bool, next time.Time, sent, failed bool, lastError string) {
	t.Helper()
	err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT status, attempts, payload IS NOT NULL, next_attempt_at,
    sent_at IS NOT NULL, failed_at IS NOT NULL, coalesce(last_error, '')
    FROM app.outbox_messages WHERE tenant_id = $1 AND template = $2
    ORDER BY created_at DESC LIMIT 1`, tenant, template).
			Scan(&status, &attempts, &payloadPresent, &next, &sent, &failed, &lastError)
	})
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestEnqueueCommitRollbackAndDelivery(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, tenant, "password_reset")
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, attempts, payload, _, sent, _, _ := state(t, runner, tenant, "password_reset")
	if fake.Count() != 1 || status != "sent" || attempts != 0 || payload || !sent {
		t.Fatalf("count=%d status=%s attempts=%d payload=%v sent=%v", fake.Count(), status, attempts, payload, sent)
	}
	abort := errors.New("rollback")
	err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		if err := NewEnqueuer().Enqueue(ctx, tx, Message{TenantID: tenant, Kind: "email", Template: "invitation",
			Recipient: "user@example.com", Payload: map[string]string{"link": "x"}}); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("rollback=%v", err)
	}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 1 {
		t.Fatalf("sent rolled back message: %d", fake.Count())
	}
}

func TestRecoverableAndPermanentFailures(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	fake.err = errors.New("SMTP failed user@example.com token=secret")
	enqueue(t, runner, tenant, "password_reset")
	for attempt, delay := range []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 6 * time.Hour, 6 * time.Hour, 6 * time.Hour, 6 * time.Hour} {
		if err := dispatcher.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		status, count, payload, next, _, failed, last := state(t, runner, tenant, "password_reset")
		if attempt < 7 {
			if status != "pending" || count != attempt+1 || !payload || !next.Equal(dispatcher.clock.Now().Add(delay)) {
				t.Fatalf("attempt %d: %s %d %v %s", attempt+1, status, count, payload, next)
			}
			dispatcher.clock = fakeClock{next}
		} else if status != "failed" || payload || !failed || count != 8 {
			t.Fatalf("terminal: %s %d %v %v", status, count, payload, failed)
		}
		if last == "" || strings.Contains(last, "user@example.com") || strings.Contains(last, "secret") {
			t.Fatalf("unsafe last_error=%q", last)
		}
	}
	fake.err = &PermanentError{Err: errors.New("SMTP 550")}
	enqueue(t, runner, tenant, "invitation")
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, count, payload, _, _, failed, _ := state(t, runner, tenant, "invitation")
	if status != "failed" || count != 1 || payload || !failed {
		t.Fatalf("permanent: %s %d %v %v", status, count, payload, failed)
	}
}

func TestFutureAndCancellation(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, tenant, "password_reset")
	err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.outbox_messages SET next_attempt_at = $1 WHERE tenant_id = $2 AND template = 'password_reset'`, dispatcher.clock.Now().Add(time.Hour), tenant)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 0 {
		t.Fatal("future message sent")
	}
	dispatcher.clock = fakeClock{dispatcher.clock.Now().Add(time.Hour)}
	fake.wait = true
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	if err := dispatcher.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	status, count, payload, next, _, _, last := state(t, runner, tenant, "password_reset")
	if status != "pending" || count != 0 || !payload || last != "" || !next.Equal(dispatcher.clock.Now()) {
		t.Fatalf("canceled: %s %d %v %s %q", status, count, payload, next, last)
	}
}

func TestConcurrentDispatchersAndTenantRole(t *testing.T) {
	runner, a, fake, dispatcher := newFixture(t)
	b := cleanCompany(t)
	for i := 0; i < 10; i++ {
		enqueue(t, runner, a, "password_reset")
		enqueue(t, runner, b.ID, "invitation")
	}
	var roles sync.Map
	dispatcher.onTenant = func(ctx context.Context, tx db.Tx) {
		var role string
		if err := tx.QueryRow(ctx, `SELECT current_user`).Scan(&role); err != nil {
			t.Error(err)
			return
		}
		tenant, _ := tx.TenantID()
		roles.Store(tenant, role)
	}
	other := NewDispatcher(runner, fake, dispatcher.clock, slog.New(slog.NewTextHandler(io.Discard, nil)))
	other.onTenant = dispatcher.onTenant
	var wg sync.WaitGroup
	for _, d := range []*Dispatcher{dispatcher, other} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 3; i++ {
				if err := d.RunOnce(context.Background()); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if fake.Count() != 20 {
		t.Fatalf("sent %d messages, want 20", fake.Count())
	}
	for tenant, want := range map[uuid.UUID]string{a: db.TenantRoleName(a), b.ID: db.TenantRoleName(b.ID)} {
		got, ok := roles.Load(tenant)
		if !ok || got != want {
			t.Errorf("role for %s = %v", tenant, got)
		}
	}
	err := runner.InTenantTx(context.Background(), a, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.outbox_messages SET status = 'sent', sent_at = now()
   WHERE tenant_id = $1 AND status = 'pending'`, a)
		return err
	})
	// All rows were already sent above, so verify the constraint separately on a new pending row.
	_ = err
	enqueue(t, runner, a, "email_verification")
	err = runner.InTenantTx(context.Background(), a, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.outbox_messages SET status = 'sent', sent_at = now()
   WHERE tenant_id = $1 AND template = 'email_verification'`, a)
		return err
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "outbox_scrub_chk" {
		t.Fatalf("scrub constraint=%v", err)
	}
}
