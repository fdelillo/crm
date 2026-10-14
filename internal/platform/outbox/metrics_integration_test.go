//go:build integration

package outbox

import (
	"bytes"
	"context"
	"errors"
	"github.com/fdelillo/crm/internal/platform/db"
	"strings"
	"testing"
)

func TestPhase9DeliveryMetrics(t *testing.T) {
	for _, cause := range []Cause{CauseNetwork, CauseTransient, CauseConfig, CauseRecipient, CauseBug} {
		t.Run(string(cause), func(t *testing.T) {
			runner, id, handler, d := newFixture(t)
			before := deliveryErrors.Get(string(cause)).String()
			failed := failedTotal.Value()
			handler.fn = func(context.Context, Message, int) error {
				return &DeliveryError{Cause: cause, Phase: PhaseData, Detail: "test"}
			}
			enqueue(t, runner, d.clock, id, "invitation")
			if err := d.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if deliveryErrors.Get(string(cause)).String() == before {
				t.Error("delivery error metric not incremented")
			}
			delta := int64(0)
			if cause.Permanent() {
				delta = 1
			}
			if failedTotal.Value() != failed+delta {
				t.Error("terminal counter")
			}
		})
	}
	runner, id, _, d := newFixture(t)
	deferred := deferredTotal.Value()
	enqueue(t, runner, d.clock, id, "invitation")
	d.failAsTenant = func() error { return errors.New("forced role failure") }
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if deferredTotal.Value() != deferred+1 {
		t.Error("deferral counter")
	}
	runner, id, handler, d := newFixture(t)
	handler.fn = func(context.Context, Message, int) error {
		return &DeliveryError{Cause: CauseBug, Phase: PhaseCompose, Detail: "test"}
	}
	enqueue(t, runner, d.clock, id, "invitation")
	d.runner = &rollbackMetricsRunner{TxRunner: runner}
	failed := failedTotal.Value()
	bugs := deliveryErrors.Get(string(CauseBug)).String()
	if _, err := d.processOne(context.Background()); err == nil {
		t.Fatal("rollback injection did not fail")
	}
	if failedTotal.Value() != failed || deliveryErrors.Get(string(CauseBug)).String() != bugs {
		t.Error("rolled back delivery counted")
	}
	var logs bytes.Buffer
	runner, _, _, d = newLoggedFixture(t, &logs)
	stats := NewStats(&rollbackMetricsRunner{TxRunner: runner}, d.clock)
	d.tasks = []PeriodicTask{stats}
	d.runTasks(context.Background())
	if !strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), `"task":"outbox_stats"`) {
		t.Error(logs.String())
	}
}

type rollbackMetricsRunner struct{ db.TxRunner }

func (r *rollbackMetricsRunner) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	return r.TxRunner.InSystemTx(ctx, role, func(ctx context.Context, tx db.Tx) error {
		if err := fn(ctx, tx); err != nil {
			return err
		}
		return errors.New("forced rollback before commit")
	})
}
