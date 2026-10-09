package outbox

import (
	"context"
	"expvar"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox/store"
	"sync/atomic"
	"time"
)

type pendingSample struct {
	count  int64
	oldest time.Time
}

// Stats stores an immutable queue sample. Scrapes read memory and the injected clock only.
type Stats struct {
	runner db.TxRunner
	clock  clock.Clock
	sample atomic.Pointer[pendingSample]
}

var publishedStats atomic.Pointer[Stats]
var failedTotal = expvar.NewInt("outbox_failed_total")
var deferredTotal = expvar.NewInt("outbox_deferred_total")
var deliveryErrors = expvar.NewMap("outbox_delivery_errors_total")

func init() {
	for _, cause := range []Cause{CauseNetwork, CauseTransient, CauseConfig, CauseRecipient, CauseBug} {
		deliveryErrors.Add(string(cause), 0)
	}
	expvar.Publish("outbox_pending", expvar.Func(func() any {
		stats := publishedStats.Load()
		if stats == nil {
			return int64(-1)
		}
		sample := stats.sample.Load()
		if sample == nil {
			return int64(-1)
		}
		return sample.count
	}))
	expvar.Publish("outbox_oldest_pending_seconds", expvar.Func(func() any {
		stats := publishedStats.Load()
		if stats == nil {
			return float64(-1)
		}
		sample := stats.sample.Load()
		if sample == nil {
			return float64(-1)
		}
		if sample.count == 0 {
			return float64(0)
		}
		return max(0, stats.clock.Now().Sub(sample.oldest).Seconds())
	}))
}
func NewStats(runner db.TxRunner, clock clock.Clock) *Stats {
	task := &Stats{runner: runner, clock: clock}
	publishedStats.Store(task)
	return task
}
func (*Stats) Name() string         { return "outbox_stats" }
func (*Stats) Every() time.Duration { return time.Minute }
func (s *Stats) Run(ctx context.Context) error {
	var sample pendingSample
	err := s.runner.InSystemTx(ctx, db.RoleWorker, func(ctx context.Context, tx db.Tx) error {
		row, err := store.New(tx).PendingStats(ctx)
		sample = pendingSample{row.Pending, row.Oldest}
		return db.MapError(err)
	})
	if err != nil {
		s.sample.Store(nil)
		return err
	}
	s.sample.Store(&sample)
	return nil
}

// Delivery counters are published only after the delivery transaction commits.
type deliveryMetric struct {
	cause  Cause
	failed bool
}

func (m deliveryMetric) publish() {
	if m.cause != "" {
		deliveryErrors.Add(string(m.cause), 1)
	}
	if m.failed {
		failedTotal.Add(1)
	}
}
