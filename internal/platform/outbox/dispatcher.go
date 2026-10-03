package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const pollInterval = 2 * time.Second
const batchSize = 10
const maxAttempts = 8

// errCycleStop tells RunOnce that the batch already ended on its own after a connection-phase
// delivery failure (ADR-024 §4): the failure was already handled and logged at the right level,
// so RunOnce must stop without treating it as a crash.
var errCycleStop = errors.New("outbox: a connection-phase delivery failure ended the cycle")

// retryDelay is the fixed backoff of plan §9.4: 1, 5, 15, 60 minutes, then 6 hours for every
// attempt after that (the 8th exhausts it and the message becomes failed).
func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 15 * time.Minute
	case 4:
		return time.Hour
	default:
		return 6 * time.Hour
	}
}

// deferDelay is the pure backoff of the Dispatcher's own failures (AsTenant, GetMessage or the
// mark; ADR-024 §6): clamp(age, 10s, 15m), with age = now - created_at. It needs no state besides
// the message's own created_at, so it doubles naturally on successive deferrals (10s, 20s, 40s…)
// without a counter column. A negative age (a clock skew, or created_at in the future) is treated
// as 0: the minimum of 10s still applies.
func deferDelay(age time.Duration) time.Duration {
	switch {
	case age < 10*time.Second:
		return 10 * time.Second
	case age > 15*time.Minute:
		return 15 * time.Minute
	default:
		return age
	}
}

// classifyHandleResult turns the result of Handler.Handle into either a cancellation or a
// *DeliveryError with a cause and a phase (ADR-024 §1). cycleAlive is whether the cycle's own
// context (not the per-send one SendBudget wraps) was still alive when Handle returned: it tells
// "the SendBudget ran out" (network, recoverable) from "the cycle itself was cancelled or timed
// out" (no classification, the message is left untouched). unclassified marks a bare error the
// Handler returned that is neither a *DeliveryError nor a budget timeout: ADR-024 treats it as our
// own bug (config/unknown) and the caller must log its %T, never its text (it could hold an
// address).
func classifyHandleResult(err error, cycleAlive bool) (de *DeliveryError, canceled, unclassified bool) {
	// *DeliveryError is checked first (N1 of the second PR #8 review): a dial/TLS timeout wraps
	// context.DeadlineExceeded inside a connection-phase *DeliveryError (classifyConnectionError in
	// platform/mailer), and errors.Is traverses Unwrap. Checking the bare DeadlineExceeded branch
	// first would replace that phase with "unknown" and the cycle would not stop (ADR-024 §4,
	// INV-31). This is safe: once SendBudget itself runs out, the adapter returns ctx.Err()
	// unwrapped (platform/mailer/smtp.go), never inside a *DeliveryError.
	var existing *DeliveryError
	if errors.As(err, &existing) {
		return existing, false, false
	}
	if errors.Is(err, context.DeadlineExceeded) && cycleAlive {
		return &DeliveryError{Cause: CauseNetwork, Phase: PhaseUnknown, Detail: "timeout"}, false, false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, db.ErrCanceled) {
		return nil, true, false
	}
	return &DeliveryError{Cause: CauseConfig, Phase: PhaseUnknown, Detail: "unclassified handler error", Err: err}, false, true
}

// deliveryStep names one of the per-message steps processOne runs under the company's role.
type deliveryStep string

const (
	stepAsTenant   deliveryStep = "as_tenant"
	stepGetMessage deliveryStep = "get_message"
	stepMark       deliveryStep = "mark"
)

// stepFailure tags an error from one of the per-message steps that run under the company's role
// (AsTenant, GetMessage, or marking the outcome) once the transaction that attempted it has
// already rolled back: processOne uses step to log and to pick the "mark" record it sends to
// DeferMessage. A failure without this wrapper (LockDueMessage, DeferMessage itself) always ends
// the cycle; one with it is isolated to its own message when it is not db.ErrUnavailable
// (ADR-024 §6).
type stepFailure struct {
	step deliveryStep
	err  error
}

func (s *stepFailure) Error() string { return s.err.Error() }
func (s *stepFailure) Unwrap() error { return s.err }

// Dispatcher polls app.outbox_messages and delivers pending messages (ADR-010). Each message runs
// in its own InSystemTx(crm_worker) -> AsTenant transaction, so SKIP LOCKED divides work across
// dispatchers and a failure never holds another company's messages.
type Dispatcher struct {
	runner  db.TxRunner
	handler Handler
	clock   clock.Clock
	logger  *slog.Logger
	tasks   []PeriodicTask

	// Test hooks (same package only). Each runs after the real call it shadows succeeded, and may
	// turn that success into a failure, so a test can simulate db.ErrUnavailable or any other
	// error without a broken connection or a missing role (ADR-024 §6).
	onTenant       func(context.Context, db.Tx) // invoked after AsTenant, before reading the payload
	failAsTenant   func() error
	failGetMessage func() error
	failMarkSent   func() error
}

// NewDispatcher returns a Dispatcher. c defaults to clock.Real{} and logger to slog.Default() when nil.
func NewDispatcher(runner db.TxRunner, handler Handler, c clock.Clock, logger *slog.Logger, tasks ...PeriodicTask) *Dispatcher {
	if c == nil {
		c = clock.Real{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{runner: runner, handler: handler, clock: c, logger: logger, tasks: tasks}
}

// Run polls immediately and then every two seconds until ctx is done. A cycle's own failures are
// already logged where they happen (processOne, RunOnce); Run only drives the loop and the
// periodic tasks (T-B902).
func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	lastRun := map[string]time.Time{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		_ = d.RunOnce(ctx)
		now := d.clock.Now()
		for _, task := range d.tasks {
			if now.Sub(lastRun[task.Name()]) < task.Every() {
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			if err := task.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				d.logger.ErrorContext(ctx, "periodic task failed", "task", task.Name(), "err", err)
			}
			lastRun[task.Name()] = now
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// RunOnce processes at most ten due messages, checking ctx before taking each one (DD-35): an
// already-cancelled ctx takes none. It stops early, without error, on cancellation and on a
// connection-phase delivery failure (ADR-024 §4); any other failure is logged here, at the point
// it ends the cycle, with the event every caller (including Run) relies on.
func (d *Dispatcher) RunOnce(ctx context.Context) error {
	for i := 0; i < batchSize; i++ {
		if ctx.Err() != nil {
			return nil
		}
		handled, err := d.processOne(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, errCycleStop) {
				return nil
			}
			d.logger.ErrorContext(ctx, "outbox cycle failed", "event", "outbox_cycle_failed", "err", err)
			return err
		}
		if !handled {
			return nil
		}
	}
	return nil
}

// processOne claims at most one due message and resolves it. The transaction runs with
// context.WithoutCancel(ctx) (DD-35): its statements are already bounded by statement_timeout and
// idle_in_transaction_session_timeout, and letting the cycle's own cancellation reach it would
// risk a half-finished mark. Only Handler.Handle receives ctx directly, wrapped with SendBudget,
// so an apagado can still cut a send short.
func (d *Dispatcher) processOne(ctx context.Context) (bool, error) {
	var item store.LockDueMessageRow
	handled := false
	// stopCycle is set by resolveDelivery, inside fn, when the failure was in the connection phase
	// (ADR-024 §4): it is read only after the transaction below has committed, so a stop never
	// rolls back the mark that was just made.
	stopCycle := false
	err := d.runner.InSystemTx(context.WithoutCancel(ctx), db.RoleWorker, func(fnCtx context.Context, tx db.Tx) error {
		q := store.New(tx)
		row, lockErr := q.LockDueMessage(fnCtx, d.clock.Now())
		if errors.Is(lockErr, pgx.ErrNoRows) {
			return nil
		}
		if lockErr != nil {
			return fmt.Errorf("lock due message: %w", db.MapError(lockErr))
		}
		item = row
		handled = true

		asTenantErr := tx.AsTenant(fnCtx, item.TenantID)
		if asTenantErr == nil && d.failAsTenant != nil {
			asTenantErr = d.failAsTenant()
		}
		if asTenantErr != nil {
			return &stepFailure{step: stepAsTenant, err: db.MapError(asTenantErr)}
		}
		if d.onTenant != nil {
			d.onTenant(fnCtx, tx)
		}

		message, getErr := q.GetMessage(fnCtx, store.GetMessageParams{TenantID: item.TenantID, ID: item.ID})
		if getErr == nil && d.failGetMessage != nil {
			getErr = d.failGetMessage()
		}
		if getErr != nil {
			return &stepFailure{step: stepGetMessage, err: db.MapError(getErr)}
		}

		var payload map[string]string
		if jsonErr := json.Unmarshal(message.Payload, &payload); jsonErr != nil {
			var resolveErr error
			stopCycle, resolveErr = d.resolveDelivery(fnCtx, q, item, message.Attempts,
				&DeliveryError{Cause: CauseBug, Phase: PhaseCompose, Detail: "invalid payload"}, true)
			return resolveErr
		}

		m := Message{TenantID: message.TenantID, Kind: message.Kind, Template: message.Template,
			Recipient: message.Recipient, Payload: payload}
		sendCtx, cancel := context.WithTimeout(ctx, SendBudget)
		handleErr := d.handler.Handle(sendCtx, m)
		cycleAlive := ctx.Err() == nil
		cancel()
		var resolveErr error
		stopCycle, resolveErr = d.resolveDelivery(fnCtx, q, item, message.Attempts, handleErr, cycleAlive)
		return resolveErr
	})

	switch {
	case err == nil:
		if stopCycle {
			return handled, errCycleStop
		}
		return handled, nil
	case errors.Is(err, context.Canceled), errors.Is(err, db.ErrCanceled):
		return handled, context.Canceled
	}

	var sf *stepFailure
	if errors.As(err, &sf) {
		if errors.Is(sf.err, db.ErrUnavailable) {
			return handled, fmt.Errorf("%s: %w", sf.step, sf.err)
		}
		if deferErr := d.deferMessage(ctx, item, sf.step, sf.err); deferErr != nil {
			return handled, deferErr
		}
		return handled, nil
	}
	return handled, err
}

// resolveDelivery marks the outcome of one delivery attempt and logs it (plan §9.4, ADR-024
// §1-§3). handleErr is nil on success, or whatever Handler.Handle returned. A failure to mark is
// returned as a *stepFailure so processOne can defer the message instead (ADR-024 §6): the
// message is not lost, it is retried once the transient condition clears. stop is true only for a
// connection-phase failure, and only once err is nil: the mark must still commit (ADR-024 §4 ends
// the cycle only *after* the failure is recorded), so processOne reads stop once this transaction
// has succeeded.
func (d *Dispatcher) resolveDelivery(ctx context.Context, q *store.Queries, item store.LockDueMessageRow, attempts int32, handleErr error, cycleAlive bool) (stop bool, err error) {
	if handleErr == nil {
		now := d.clock.Now()
		markErr := q.MarkSent(ctx, store.MarkSentParams{TenantID: item.TenantID, ID: item.ID, SentAt: &now})
		if markErr == nil && d.failMarkSent != nil {
			markErr = d.failMarkSent()
		}
		if markErr != nil {
			return false, &stepFailure{step: stepMark, err: db.MapError(markErr)}
		}
		d.logger.InfoContext(ctx, "outbox delivery sent", "event", "outbox_delivery", "outcome", "sent",
			"tenant_id", item.TenantID, "message_id", item.ID)
		return false, nil
	}

	de, canceled, unclassified := classifyHandleResult(handleErr, cycleAlive)
	if canceled {
		d.logger.InfoContext(ctx, "outbox delivery canceled", "event", "outbox_delivery", "outcome", "canceled",
			"tenant_id", item.TenantID, "message_id", item.ID)
		return false, context.Canceled
	}

	last := de.LastError()
	fields := []any{"event", "outbox_delivery", "tenant_id", item.TenantID, "message_id", item.ID,
		"error_cause", string(de.Cause), "smtp_phase", string(de.Phase), "last_error", last}
	if de.SMTPCode != 0 {
		fields = append(fields, "smtp_code", de.SMTPCode)
	}
	if unclassified {
		fields = append(fields, "err_type", fmt.Sprintf("%T", handleErr))
	}

	now := d.clock.Now()
	if de.Cause.Permanent() || attempts+1 >= maxAttempts {
		if err := q.MarkFailed(ctx, store.MarkFailedParams{TenantID: item.TenantID, ID: item.ID,
			FailedAt: &now, LastError: pgtype.Text{String: last, Valid: true}}); err != nil {
			return false, &stepFailure{step: stepMark, err: db.MapError(err)}
		}
		level := de.Cause.LogLevel()
		outcome := []any{"outcome", "failed"}
		if !de.Cause.Permanent() {
			outcome = append(outcome, "reason", "max_attempts")
			level = slog.LevelError
		}
		d.logger.Log(ctx, level, "outbox delivery failed", append(fields, outcome...)...)
	} else {
		next := now.Add(retryDelay(int(attempts) + 1))
		if err := q.MarkRecoverable(ctx, store.MarkRecoverableParams{TenantID: item.TenantID, ID: item.ID,
			NextAttemptAt: next, LastError: pgtype.Text{String: last, Valid: true}}); err != nil {
			return false, &stepFailure{step: stepMark, err: db.MapError(err)}
		}
		d.logger.Log(ctx, de.Cause.LogLevel(), "outbox delivery will retry",
			append(fields, "outcome", "retry", "next_attempt_at", next)...)
	}

	return de.Phase == PhaseConnection, nil
}

// deferMessage isolates a failure of AsTenant, GetMessage or the mark to its own message
// (ADR-024 §6): the message's transaction has already rolled back, so this runs in a new one, as
// crm_worker, moving only next_attempt_at. A failure here (unlike a deferral itself) always ends
// the cycle: retrying it inside the same batch risks taking the same message again. cause is
// included in that failure (nit of the second PR #8 review): otherwise the step that originally
// failed, and a possible security_event=rls_violation, would never reach a log line.
func (d *Dispatcher) deferMessage(ctx context.Context, item store.LockDueMessageRow, step deliveryStep, cause error) error {
	now := d.clock.Now()
	delay := deferDelay(now.Sub(item.CreatedAt))
	next := now.Add(delay)
	var affected int64
	err := d.runner.InSystemTx(context.WithoutCancel(ctx), db.RoleWorker, func(fnCtx context.Context, tx db.Tx) error {
		rows, err := store.New(tx).DeferMessage(fnCtx, store.DeferMessageParams{
			NextAttemptAt: next, ID: item.ID, ExpectedNextAttemptAt: item.NextAttemptAt,
		})
		affected = rows
		return err
	})
	if err != nil {
		return fmt.Errorf("defer message %s after %s failure (%w): %w", item.ID, step, cause, db.MapError(err))
	}
	// 0 rows affected means another worker already claimed and resolved this message (DeferMessage's
	// own doc comment): nothing was deferred, so there is nothing to log.
	if affected == 0 {
		return nil
	}
	fields := []any{"event", "outbox_delivery", "outcome", "deferred", "step", string(step),
		"message_id", item.ID, "tenant_id", item.TenantID, "defer_seconds", int(delay.Seconds()), "err", cause}
	if errors.Is(cause, db.ErrPrivilege) {
		fields = append(fields, "security_event", "rls_violation")
	}
	d.logger.ErrorContext(ctx, "outbox message deferred", fields...)
	return nil
}
