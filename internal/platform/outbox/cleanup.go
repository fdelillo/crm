package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox/store"
)

type terminalCleanup struct {
	runner db.TxRunner
	logger *slog.Logger
}

func (terminalCleanup) Name() string         { return "outbox_cleanup" }
func (terminalCleanup) Every() time.Duration { return time.Hour }
func (c terminalCleanup) Run(ctx context.Context) error {
	start := time.Now()
	var deleted int64
	err := c.runner.InSystemTx(ctx, db.RoleWorker, func(ctx context.Context, tx db.Tx) error {
		rows, err := store.New(tx).DeleteTerminalMessages(ctx)
		deleted = rows
		if err != nil {
			return fmt.Errorf("outbox cleanup: %w", db.MapError(err))
		}
		return nil
	})
	if err != nil {
		return err
	}
	c.logger.InfoContext(ctx, "periodic cleanup completed", "task", c.Name(), "deleted_rows", deleted, "duration_ms", time.Since(start).Milliseconds())
	return nil
}
