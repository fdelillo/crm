//go:build integration

package outbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox/store"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

// fakeHandler is the Handler double of every test: fn, when set, computes the result of the
// call-th invocation (0-based, across the fake's whole life); wait makes Handle block on ctx.Done()
// first, then return afterWait if afterWaitSet, or ctx.Err() otherwise (the apagado cases).
type fakeHandler struct {
	mu           sync.Mutex
	messages     []Message
	deadlines    []time.Time
	fn           func(ctx context.Context, m Message, call int) error
	wait         bool
	afterWait    error
	afterWaitSet bool
	// entered, when set, is closed the instant Handle starts blocking (wait=true): a test cancels
	// the context only after reading from it, instead of guessing a sleep duration (M8 of the PR #8
	// review: the previous version raced a fixed 20ms sleep against goroutine scheduling).
	entered chan struct{}
}

func (f *fakeHandler) Handle(ctx context.Context, m Message) error {
	f.mu.Lock()
	call := len(f.messages)
	f.messages = append(f.messages, m)
	if deadline, ok := ctx.Deadline(); ok {
		f.deadlines = append(f.deadlines, deadline)
	}
	wait, afterWait, afterWaitSet, fn, entered := f.wait, f.afterWait, f.afterWaitSet, f.fn, f.entered
	f.mu.Unlock()
	if wait {
		if entered != nil {
			close(entered)
		}
		<-ctx.Done()
		if afterWaitSet {
			return afterWait
		}
		return ctx.Err()
	}
	if fn != nil {
		return fn(ctx, m, call)
	}
	return nil
}
func (f *fakeHandler) Count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.messages) }
func (f *fakeHandler) Deadline(i int) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deadlines[i]
}

func newFixture(t *testing.T) (db.TxRunner, uuid.UUID, *fakeHandler, *Dispatcher) {
	return newLoggedFixture(t, io.Discard)
}
func newLoggedFixture(t *testing.T, logs io.Writer) (db.TxRunner, uuid.UUID, *fakeHandler, *Dispatcher) {
	t.Helper()
	c := cleanCompany(t)
	runner := db.NewTxRunner(pgtest.AppPool(t))
	fake := &fakeHandler{}
	now := time.Now().UTC().Truncate(time.Microsecond)
	dispatcher := NewDispatcher(runner, fake, fakeClock{now}, slog.New(slog.NewJSONHandler(logs, nil)))
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

// enqueue writes a message with the given clock (usually the dispatcher's own), so LockDueMessage's
// "next_attempt_at <= now" sees the exact instant the test controls (M6: no +1s compensation).
func enqueue(t *testing.T, runner db.TxRunner, c clock.Clock, tenant uuid.UUID, template string) {
	t.Helper()
	err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		return NewEnqueuer(c).Enqueue(ctx, tx, Message{TenantID: tenant, Kind: "email", Template: template,
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

// stateAsSuperuser reads a message's queue columns bypassing RLS, for a company whose role was
// deliberately dropped (the only way to inspect it without AsTenant failing too).
func stateAsSuperuser(t *testing.T, tenant uuid.UUID, template string) (status string, attempts int32, next time.Time, lastError string) {
	t.Helper()
	err := pgtest.SuperuserPool(t).QueryRow(context.Background(),
		`SELECT status, attempts, next_attempt_at, coalesce(last_error, '') FROM app.outbox_messages
		 WHERE tenant_id = $1 AND template = $2 ORDER BY created_at DESC LIMIT 1`, tenant, template).
		Scan(&status, &attempts, &next, &lastError)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestEnqueueCommitRollbackAndDelivery(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, attempts, payload, _, sent, _, _ := state(t, runner, tenant, "password_reset")
	if fake.Count() != 1 || status != "sent" || attempts != 0 || payload || !sent {
		t.Fatalf("count=%d status=%s attempts=%d payload=%v sent=%v", fake.Count(), status, attempts, payload, sent)
	}
	abort := errors.New("rollback")
	err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		if err := NewEnqueuer(dispatcher.clock).Enqueue(ctx, tx, Message{TenantID: tenant, Kind: "email", Template: "invitation",
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

func TestEnqueueFixesCreatedAtAndNextAttemptAtFromTheClock(t *testing.T) {
	runner, tenant, _, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	_, _, _, next, _, _, _ := state(t, runner, tenant, "password_reset")
	if !next.Equal(dispatcher.clock.Now()) {
		t.Fatalf("next_attempt_at=%s want %s", next, dispatcher.clock.Now())
	}
	var created time.Time
	err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT created_at FROM app.outbox_messages WHERE tenant_id = $1 AND template = 'password_reset'
			ORDER BY created_at DESC LIMIT 1`, tenant).Scan(&created)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.Equal(dispatcher.clock.Now()) {
		t.Fatalf("created_at=%s want %s", created, dispatcher.clock.Now())
	}
}

func TestNetworkFailureRetriesWithBackoffThenFails(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, fake, dispatcher := newLoggedFixture(t, &logs)
	fake.fn = func(ctx context.Context, m Message, call int) error {
		return &DeliveryError{Cause: CauseNetwork, Phase: PhaseData, Detail: "timeout"}
	}
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	for attempt, delay := range []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 6 * time.Hour, 6 * time.Hour, 6 * time.Hour, 6 * time.Hour} {
		if err := dispatcher.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		status, count, payload, next, _, failed, last := state(t, runner, tenant, "password_reset")
		if attempt < 7 {
			if status != "pending" || count != attempt+1 || !payload || !next.Equal(dispatcher.clock.Now().Add(delay)) {
				t.Fatalf("attempt %d: %s %d %v %s", attempt+1, status, count, payload, next)
			}
			if last != "network data: timeout" {
				t.Fatalf("last_error=%q", last)
			}
			dispatcher.clock = fakeClock{next}
		} else if status != "failed" || payload || !failed || count != 8 {
			t.Fatalf("terminal: %s %d %v %v", status, count, payload, failed)
		}
	}
	if !strings.Contains(logs.String(), `"outcome":"failed"`) || !strings.Contains(logs.String(), `"reason":"max_attempts"`) {
		t.Fatalf("missing max_attempts log: %s", logs.String())
	}
}

func TestConfigFailureIsRecoverableButLoggedAtError(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, fake, dispatcher := newLoggedFixture(t, &logs)
	fake.fn = func(ctx context.Context, m Message, call int) error {
		return &DeliveryError{Cause: CauseConfig, Phase: PhaseConnection, SMTPCode: 535, Enhanced: "5.7.8",
			Detail: "Authentication credentials invalid"}
	}
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, attempts, payload, _, _, failed, last := state(t, runner, tenant, "password_reset")
	if status != "pending" || attempts != 1 || !payload || failed {
		t.Fatalf("status=%s attempts=%d payload=%v failed=%v", status, attempts, payload, failed)
	}
	if last != "config connection 535 5.7.8: Authentication credentials invalid" {
		t.Fatalf("last_error=%q", last)
	}
	if !strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), `"outcome":"retry"`) ||
		!strings.Contains(logs.String(), `"error_cause":"config"`) || !strings.Contains(logs.String(), `"smtp_code":535`) {
		t.Fatalf("missing config error log: %s", logs.String())
	}
}

func TestConnectionPhaseFailureEndsTheCycle(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	enqueue(t, runner, dispatcher.clock, tenant, "invitation")
	fake.fn = func(ctx context.Context, m Message, call int) error {
		if call == 0 {
			return &DeliveryError{Cause: CauseConfig, Phase: PhaseConnection, SMTPCode: 554, Detail: "client host blocked"}
		}
		return nil
	}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 1 {
		t.Fatalf("calls=%d want 1", fake.Count())
	}
	statusSecond, _, _, _, _, _, _ := state(t, runner, tenant, "invitation")
	if statusSecond != "pending" {
		t.Fatalf("second message status=%s", statusSecond)
	}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 2 {
		t.Fatalf("second cycle calls=%d want 2", fake.Count())
	}
}

// Variant of TestConnectionPhaseFailureEndsTheCycle for N1 of the second PR #8 review: the
// connection-phase failure wraps context.DeadlineExceeded (a dial/TLS timeout from go-mail), as
// opposed to a plain *DeliveryError. The cycle must still stop after the first message.
func TestConnectionPhaseFailureFromWrappedDeadlineExceededEndsTheCycle(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	enqueue(t, runner, dispatcher.clock, tenant, "invitation")
	fake.fn = func(ctx context.Context, m Message, call int) error {
		if call == 0 {
			return &DeliveryError{Phase: PhaseConnection, Err: fmt.Errorf("dial failed: %w", context.DeadlineExceeded)}
		}
		return nil
	}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 1 {
		t.Fatalf("calls=%d want 1", fake.Count())
	}
	statusSecond, _, _, _, _, _, _ := state(t, runner, tenant, "invitation")
	if statusSecond != "pending" {
		t.Fatalf("second message status=%s", statusSecond)
	}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 2 {
		t.Fatalf("second cycle calls=%d want 2", fake.Count())
	}
}

func TestTransientFailureDoesNotEndTheCycle(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	enqueue(t, runner, dispatcher.clock, tenant, "invitation")
	fake.fn = func(ctx context.Context, m Message, call int) error {
		if call == 0 {
			return &DeliveryError{Cause: CauseTransient, Phase: PhaseRcptTo, SMTPCode: 452, Enhanced: "4.2.2", Detail: "Mailbox full"}
		}
		return nil
	}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 2 {
		t.Fatalf("calls=%d want 2", fake.Count())
	}
	statusFirst, _, _, _, _, _, _ := state(t, runner, tenant, "password_reset")
	if statusFirst != "pending" {
		t.Fatalf("first status=%s", statusFirst)
	}
	statusSecond, _, _, _, sent, _, _ := state(t, runner, tenant, "invitation")
	if statusSecond != "sent" || !sent {
		t.Fatalf("second status=%s sent=%v", statusSecond, sent)
	}
}

func TestRecipientFailureIsPermanentAndRedacted(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, fake, dispatcher := newLoggedFixture(t, &logs)
	fake.fn = func(ctx context.Context, m Message, call int) error {
		return &DeliveryError{Cause: CauseRecipient, Phase: PhaseRcptTo, SMTPCode: 550, Enhanced: "5.1.1",
			Detail: "<persona@example.com>: User unknown"}
	}
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, count, payload, _, _, failed, last := state(t, runner, tenant, "password_reset")
	if status != "failed" || count != 1 || payload || !failed {
		t.Fatalf("status=%s count=%d payload=%v failed=%v", status, count, payload, failed)
	}
	if last != "recipient rcpt_to 550 5.1.1: [redacted] User unknown" {
		t.Fatalf("last_error=%q", last)
	}
	if strings.Contains(last, "@") || strings.Contains(logs.String(), "persona@example.com") {
		t.Fatalf("leaked an address: last_error=%q logs=%s", last, logs.String())
	}
	if !strings.Contains(logs.String(), `"level":"WARN"`) || !strings.Contains(logs.String(), `"outcome":"failed"`) {
		t.Fatalf("missing warn/failed log: %s", logs.String())
	}
}

func TestBugFailureIsPermanentAndLoggedAtError(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, fake, dispatcher := newLoggedFixture(t, &logs)
	fake.fn = func(ctx context.Context, m Message, call int) error {
		return &DeliveryError{Cause: CauseBug, Phase: PhaseCompose, Detail: "unknown email template"}
	}
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, count, payload, _, _, failed, _ := state(t, runner, tenant, "password_reset")
	if status != "failed" || count != 1 || payload || !failed {
		t.Fatalf("status=%s count=%d payload=%v failed=%v", status, count, payload, failed)
	}
	if !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("missing error log: %s", logs.String())
	}
}

func TestUnclassifiedHandlerErrorIsConfigAndLogsErrType(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, fake, dispatcher := newLoggedFixture(t, &logs)
	fake.fn = func(ctx context.Context, m Message, call int) error { return errors.New("x") }
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, _, _, _, _, _, last := state(t, runner, tenant, "password_reset")
	if status != "pending" || last != "config unknown: unclassified handler error" {
		t.Fatalf("status=%s last_error=%q", status, last)
	}
	if strings.Contains(logs.String(), `"x"`) || !strings.Contains(logs.String(), `"err_type"`) {
		t.Fatalf("log leaked the bare error or missing err_type: %s", logs.String())
	}
}

func TestBudgetTimeoutWithLiveCycleIsNetworkRecoverable(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	fake.fn = func(ctx context.Context, m Message, call int) error { return context.DeadlineExceeded }
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, _, _, _, _, _, last := state(t, runner, tenant, "password_reset")
	if status != "pending" || last != "network unknown: timeout" {
		t.Fatalf("status=%s last_error=%q", status, last)
	}
}

func TestHandleReceivesSendBudgetDeadline(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 1 {
		t.Fatalf("calls=%d", fake.Count())
	}
	if deadline := fake.Deadline(0); deadline.After(time.Now().Add(SendBudget + time.Second)) {
		t.Fatalf("deadline too far in the future: %s", deadline)
	}
}

func TestInvalidPayloadJSONIsABugFailedAtError(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, _, dispatcher := newLoggedFixture(t, &logs)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	if err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.outbox_messages SET payload = '"not an object"'
			WHERE tenant_id = $1 AND template = 'password_reset' AND status = 'pending'`, tenant)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, _, payload, _, _, failed, last := state(t, runner, tenant, "password_reset")
	if status != "failed" || payload || !failed || last != "bug compose: invalid payload" {
		t.Fatalf("status=%s payload=%v failed=%v last=%q", status, payload, failed, last)
	}
	if !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("missing error log: %s", logs.String())
	}
}

func TestFutureMessageIsNotTaken(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
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
}

func TestCancellationLeavesTheMessageUntouched(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, fake, dispatcher := newLoggedFixture(t, &logs)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	fake.wait, fake.entered = true, make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-fake.entered; cancel() }()
	if err := dispatcher.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	status, count, payload, next, _, _, last := state(t, runner, tenant, "password_reset")
	if status != "pending" || count != 0 || !payload || last != "" || !next.Equal(dispatcher.clock.Now()) {
		t.Fatalf("canceled: %s %d %v %s %q", status, count, payload, next, last)
	}
	if !strings.Contains(logs.String(), `"outcome":"canceled"`) {
		t.Fatalf("missing INFO outcome=canceled log: %s", logs.String())
	}
	if strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("unexpected ERROR log on cancellation: %s", logs.String())
	}
	// The message is retaken on the next cycle, with a clock past the cancellation.
	fake.wait = false
	dispatcher.clock = fakeClock{dispatcher.clock.Now().Add(time.Second)}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 2 {
		t.Fatalf("message was not retaken: calls=%d", fake.Count())
	}
}

func TestCanceledSendThatSucceedsAnywayIsMarkedSent(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	fake.wait, fake.afterWait, fake.afterWaitSet, fake.entered = true, nil, true, make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-fake.entered; cancel() }()
	if err := dispatcher.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	status, _, payload, _, sent, _, _ := state(t, runner, tenant, "password_reset")
	if status != "sent" || payload || !sent {
		t.Fatalf("status=%s payload=%v sent=%v", status, payload, sent)
	}
}

func TestRunOnceWithAlreadyCanceledContextTakesNothing(t *testing.T) {
	runner, tenant, fake, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 0 {
		t.Fatal("handled a message with an already canceled context")
	}
	status, _, _, _, _, _, _ := state(t, runner, tenant, "password_reset")
	if status != "pending" {
		t.Fatalf("status=%s", status)
	}
}

func TestDispatcherDefersMessageWhenCompanyRoleIsMissing(t *testing.T) {
	var logs bytes.Buffer
	runner, x, fake, dispatcher := newLoggedFixture(t, &logs)
	a := cleanCompany(t)
	enqueue(t, runner, dispatcher.clock, x, "password_reset")
	dispatcher.clock = fakeClock{dispatcher.clock.Now().Add(time.Millisecond)}
	enqueue(t, runner, dispatcher.clock, a.ID, "invitation")

	role := db.TenantRoleName(x)
	if _, err := pgtest.SuperuserPool(t).Exec(context.Background(), "DROP ROLE "+role); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fixture.ProvisionRole(t, pgtest.AppPool(t), x) })

	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 1 {
		t.Fatalf("calls=%d want 1", fake.Count())
	}
	statusX, attemptsX, nextX, lastX := stateAsSuperuser(t, x, "password_reset")
	if statusX != "pending" || attemptsX != 0 || lastX != "" {
		t.Fatalf("x message changed: status=%s attempts=%d last=%q", statusX, attemptsX, lastX)
	}
	wantNext := dispatcher.clock.Now().Add(10 * time.Second)
	if diff := nextX.Sub(wantNext); diff < -2*time.Second || diff > 2*time.Second {
		t.Fatalf("next_attempt_at=%s want ~%s", nextX, wantNext)
	}
	statusA, _, _, _, sentA, _, _ := state(t, runner, a.ID, "invitation")
	if statusA != "sent" || !sentA {
		t.Fatalf("a status=%s sent=%v", statusA, sentA)
	}
	if !strings.Contains(logs.String(), `"outcome":"deferred"`) || !strings.Contains(logs.String(), `"step":"as_tenant"`) {
		t.Fatalf("missing deferred log: %s", logs.String())
	}
}

func TestDispatcherDefersMessageWhenGetMessageFailsAndKeepsGoing(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, fake, dispatcher := newLoggedFixture(t, &logs)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	enqueue(t, runner, dispatcher.clock, tenant, "invitation")
	calls := 0
	dispatcher.failGetMessage = func() error {
		calls++
		if calls == 1 {
			return errors.New("boom")
		}
		return nil
	}
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 1 {
		t.Fatalf("calls=%d want 1", fake.Count())
	}
	statusSecond, _, _, _, sentSecond, _, _ := state(t, runner, tenant, "invitation")
	if statusSecond != "sent" || !sentSecond {
		t.Fatalf("second status=%s sent=%v", statusSecond, sentSecond)
	}
	if !strings.Contains(logs.String(), `"outcome":"deferred"`) || !strings.Contains(logs.String(), `"step":"get_message"`) {
		t.Fatalf("missing deferred log: %s", logs.String())
	}
}

func TestDispatcherDefersMessageWhenMarkSentFails(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, fake, dispatcher := newLoggedFixture(t, &logs)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	dispatcher.failMarkSent = func() error { return errors.New("boom") }
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.Count() != 1 {
		t.Fatalf("calls=%d want 1", fake.Count())
	}
	status, _, payload, _, sent, _, _ := state(t, runner, tenant, "password_reset")
	if status != "pending" || !payload || sent {
		t.Fatalf("status=%s payload=%v sent=%v", status, payload, sent)
	}
	if !strings.Contains(logs.String(), `"outcome":"deferred"`) || !strings.Contains(logs.String(), `"step":"mark"`) {
		t.Fatalf("missing deferred log: %s", logs.String())
	}
}

func TestDispatcherEndsTheCycleWhenAsTenantIsUnavailable(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, _, dispatcher := newLoggedFixture(t, &logs)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	dispatcher.failAsTenant = func() error { return db.ErrUnavailable }
	err := dispatcher.RunOnce(context.Background())
	if err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("expected a cycle-ending error, got %v", err)
	}
	if !errors.Is(err, db.ErrUnavailable) {
		t.Fatalf("err=%v", err)
	}
	status, attempts, _, next, _, _, _ := state(t, runner, tenant, "password_reset")
	if status != "pending" || attempts != 0 || !next.Equal(dispatcher.clock.Now()) {
		t.Fatalf("message changed: status=%s attempts=%d next=%s", status, attempts, next)
	}
	if !strings.Contains(logs.String(), `"event":"outbox_cycle_failed"`) {
		t.Fatalf("missing cycle failure log: %s", logs.String())
	}
}

func TestDeferMessageIsANoopWhenAnotherWorkerAlreadyResolvedIt(t *testing.T) {
	runner, tenant, _, dispatcher := newFixture(t)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	var id uuid.UUID
	var originalNext time.Time
	if err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT id, next_attempt_at FROM app.outbox_messages
			WHERE tenant_id = $1 AND template = 'password_reset'`, tenant).Scan(&id, &originalNext)
	}); err != nil {
		t.Fatal(err)
	}
	now := dispatcher.clock.Now()
	if err := runner.InTenantTx(context.Background(), tenant, func(ctx context.Context, tx db.Tx) error {
		return store.New(tx).MarkSent(ctx, store.MarkSentParams{TenantID: tenant, ID: id, SentAt: &now})
	}); err != nil {
		t.Fatal(err)
	}
	var affected int64
	err := runner.InSystemTx(context.Background(), db.RoleWorker, func(ctx context.Context, tx db.Tx) error {
		var err error
		affected, err = store.New(tx).DeferMessage(ctx, store.DeferMessageParams{
			NextAttemptAt: now.Add(time.Minute), ID: id, ExpectedNextAttemptAt: originalNext,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if affected != 0 {
		t.Fatalf("affected=%d want 0", affected)
	}
}

func TestRunLogsCycleFailureWithErr(t *testing.T) {
	var logs bytes.Buffer
	runner, tenant, _, dispatcher := newLoggedFixture(t, &logs)
	enqueue(t, runner, dispatcher.clock, tenant, "password_reset")
	dispatcher.failAsTenant = func() error { return db.ErrUnavailable }
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = dispatcher.Run(ctx)
	if !strings.Contains(logs.String(), `"event":"outbox_cycle_failed"`) || !strings.Contains(logs.String(), `"err"`) {
		t.Fatalf("missing cycle failure log with err: %s", logs.String())
	}
}

func TestIdleInTransactionTimeoutCoversSendBudget(t *testing.T) {
	var raw string
	if err := pgtest.AppPool(t).QueryRow(context.Background(), "SHOW idle_in_transaction_session_timeout").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	got := parsePGDuration(t, raw)
	if want := SendBudget + 10*time.Second; got < want {
		t.Fatalf("idle_in_transaction_session_timeout=%s, want >= %s", got, want)
	}
}

func parsePGDuration(t *testing.T, raw string) time.Duration {
	t.Helper()
	for suffix, unit := range map[string]time.Duration{
		"ms": time.Millisecond, "min": time.Minute, "s": time.Second, "h": time.Hour, "d": 24 * time.Hour,
	} {
		if n, ok := strings.CutSuffix(raw, suffix); ok {
			value, err := strconv.Atoi(n)
			if err == nil {
				return time.Duration(value) * unit
			}
		}
	}
	t.Fatalf("unrecognized PostgreSQL duration %q", raw)
	return 0
}

func TestConcurrentDispatchersAndTenantRole(t *testing.T) {
	runner, a, fake, dispatcher := newFixture(t)
	b := cleanCompany(t)
	for i := 0; i < 10; i++ {
		enqueue(t, runner, dispatcher.clock, a, "password_reset")
		enqueue(t, runner, dispatcher.clock, b.ID, "invitation")
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
	// Every row of a was already sent above (the loop's 10 messages and onTenant's own reads), so
	// the scrub constraint is verified on a fresh pending row instead of reusing one of those.
	enqueue(t, runner, dispatcher.clock, a, "email_verification")
	err := runner.InTenantTx(context.Background(), a, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.outbox_messages SET status = 'sent', sent_at = now()
   WHERE tenant_id = $1 AND template = 'email_verification'`, a)
		return err
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "outbox_scrub_chk" {
		t.Fatalf("scrub constraint=%v", err)
	}
}
