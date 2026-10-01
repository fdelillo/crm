package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const pollInterval = 2 * time.Second
const batchSize = 10
const maxAttempts = 8

type failureKind uint8

const (
	failureCanceled failureKind = iota
	failureRecoverable
	failurePermanent
)

func classifyFailure(err error) failureKind {
	if errors.Is(err, context.Canceled) || errors.Is(err, db.ErrCanceled) {
		return failureCanceled
	}
	var permanent *PermanentError
	if errors.As(err, &permanent) {
		return failurePermanent
	}
	return failureRecoverable
}
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

type Dispatcher struct {
	runner   db.TxRunner
	handler  Handler
	clock    clock.Clock
	logger   *slog.Logger
	tasks    []PeriodicTask
	onTenant func(context.Context, db.Tx) // test hook; invoked after AsTenant, before reading payload
}

func NewDispatcher(runner db.TxRunner, handler Handler, c clock.Clock, logger *slog.Logger, tasks ...PeriodicTask) *Dispatcher {
	if c == nil {
		c = clock.Real{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{runner: runner, handler: handler, clock: c, logger: logger, tasks: tasks}
}

// Run polls immediately and then every two seconds until shutdown.
func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	lastRun := map[string]time.Time{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := d.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			d.logger.ErrorContext(ctx, "outbox cycle failed", "event", "outbox_cycle_failed")
		}
		now := d.clock.Now()
		for _, task := range d.tasks {
			if now.Sub(lastRun[task.Name()]) < task.Every() {
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			if err := task.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				d.logger.ErrorContext(ctx, "periodic task failed", "task", task.Name())
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

// RunOnce processes at most ten due messages. Each message has its own transaction so
// AsTenant binds it to exactly one company and SKIP LOCKED divides work across dispatchers.
func (d *Dispatcher) RunOnce(ctx context.Context) error {
	for i := 0; i < batchSize; i++ {
		handled, err := d.processOne(ctx)
		if err != nil {
			return err
		}
		if !handled {
			return nil
		}
	}
	return nil
}

func (d *Dispatcher) processOne(ctx context.Context) (bool, error) {
	handled := false
	err := d.runner.InSystemTx(ctx, db.RoleWorker, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		item, err := q.LockDueMessage(ctx, d.clock.Now())
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return db.MapError(err)
		}
		handled = true
		if err := tx.AsTenant(ctx, item.TenantID); err != nil {
			return err
		}
		if d.onTenant != nil {
			d.onTenant(ctx, tx)
		}
		row, err := q.GetMessage(ctx, store.GetMessageParams{TenantID: item.TenantID, ID: item.ID})
		if err != nil {
			return db.MapError(err)
		}
		var payload map[string]string
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			return d.fail(ctx, q, item.TenantID, item.ID, "invalid_payload", row.Attempts, failurePermanent)
		}
		m := Message{TenantID: row.TenantID, Kind: row.Kind, Template: row.Template,
			Recipient: row.Recipient, Payload: payload}
		sendErr := d.handler.Handle(ctx, m)
		if sendErr == nil {
			now := d.clock.Now()
			if err := q.MarkSent(ctx, store.MarkSentParams{TenantID: item.TenantID, ID: item.ID, SentAt: &now}); err != nil {
				return db.MapError(err)
			}
			d.logger.InfoContext(ctx, "outbox delivered", "event", "outbox_delivery", "outcome", "sent", "tenant_id", item.TenantID)
			return nil
		}
		kind := classifyFailure(sendErr)
		if ctx.Err() != nil || kind == failureCanceled {
			d.logger.InfoContext(ctx, "outbox delivery canceled", "event", "outbox_delivery", "outcome", "canceled", "tenant_id", item.TenantID)
			return context.Canceled // rollback: leave every queue field unchanged
		}
		return d.fail(ctx, q, item.TenantID, item.ID, "delivery_failure", row.Attempts, kind)
	})
	return handled, err
}

func (d *Dispatcher) fail(ctx context.Context, q *store.Queries, tenantID, id uuid.UUID, reason string, previous int32, kind failureKind) error {
	now := d.clock.Now()
	last := pgtype.Text{String: reason, Valid: true}
	if kind == failurePermanent || previous+1 >= maxAttempts {
		if err := q.MarkFailed(ctx, store.MarkFailedParams{TenantID: tenantID, ID: id, FailedAt: &now, LastError: last}); err != nil {
			return db.MapError(err)
		}
		level := slog.LevelWarn
		if kind != failurePermanent {
			level = slog.LevelError
		}
		d.logger.Log(ctx, level, "outbox delivery failed", "event", "outbox_delivery", "outcome", "failed", "tenant_id", tenantID)
		return nil
	}
	next := now.Add(retryDelay(int(previous) + 1))
	if err := q.MarkRecoverable(ctx, store.MarkRecoverableParams{TenantID: tenantID, ID: id,
		NextAttemptAt: next, LastError: last}); err != nil {
		return db.MapError(err)
	}
	d.logger.WarnContext(ctx, "outbox delivery will retry", "event", "outbox_delivery", "outcome", "retry", "tenant_id", tenantID)
	return nil
}
