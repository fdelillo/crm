package identity

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
)

// Cleanup removes old identity routing data as crm_worker. RLS protects recent rows
// and open invitations even if a cleanup query becomes too broad (DD-25).
type Cleanup struct {
	runner db.TxRunner
	logger *slog.Logger
}

var _ outbox.PeriodicTask = (*Cleanup)(nil)

func NewCleanup(runner db.TxRunner, logger *slog.Logger) *Cleanup {
	if logger == nil {
		logger = slog.Default()
	}
	return &Cleanup{runner: runner, logger: logger}
}
func (*Cleanup) Name() string         { return "identity_cleanup" }
func (*Cleanup) Every() time.Duration { return time.Hour }
func (c *Cleanup) Run(ctx context.Context) error {
	start := time.Now()
	var deleted int64
	err := c.runner.InSystemTx(ctx, db.RoleWorker, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		for _, remove := range []func(context.Context) (int64, error){q.DeleteExpiredSessions, q.DeleteExpiredUserTokens, q.DeleteOldLoginThrottles} {
			rows, err := remove(ctx)
			if err != nil {
				return fmt.Errorf("identity cleanup: %w", db.MapError(err))
			}
			deleted += rows
		}
		return nil
	})
	if err != nil {
		return err
	}
	c.logger.InfoContext(ctx, "periodic cleanup completed", "task", c.Name(), "deleted_rows", deleted, "duration_ms", time.Since(start).Milliseconds())
	return nil
}
