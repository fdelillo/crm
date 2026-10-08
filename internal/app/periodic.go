package app

import (
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"log/slog"
)

// PeriodicTasks registers system work with the dispatcher, keeping each query with its owner.
func PeriodicTasks(runner db.TxRunner, _ clock.Clock, logger *slog.Logger) []outbox.PeriodicTask {
	return []outbox.PeriodicTask{identity.NewCleanup(runner, logger)}
}
