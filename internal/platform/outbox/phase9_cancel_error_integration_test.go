//go:build integration

package outbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/platform/db"
)

type failDeferralRunner struct {
	db.TxRunner
	calls int
}

func (r *failDeferralRunner) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	r.calls++
	if r.calls == 2 {
		return r.TxRunner.InSystemTx(ctx, role, func(context.Context, db.Tx) error { return db.ErrUnavailable })
	}
	return r.TxRunner.InSystemTx(ctx, role, fn)
}

func TestPhase9CanceledCyclePreservesRealDeferralFailure(t *testing.T) {
	var logs bytes.Buffer
	runner, id, _, d := newLoggedFixture(t, &logs)
	enqueue(t, runner, d.clock, id, "password_reset")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.runner = &failDeferralRunner{TxRunner: runner}
	d.failAsTenant = func() error { cancel(); return db.ErrPrivilege }
	err := d.RunOnce(ctx)
	if !errors.Is(err, db.ErrUnavailable) {
		t.Errorf("deferral error lost: %v", err)
	}
	if !strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), `"event":"outbox_cycle_failed"`) || !strings.Contains(logs.String(), `"err":"defer message`) || !strings.Contains(logs.String(), db.ErrUnavailable.Error()) {
		t.Fatalf("real error hidden by canceled ctx: %s", logs.String())
	}
}

func TestPhase9CanceledTaskPreservesRealFailure(t *testing.T) {
	for _, failure := range []error{db.ErrUnavailable, fmt.Errorf("wrapped: %w", db.ErrUnavailable), context.Canceled, db.ErrCanceled} {
		t.Run(failure.Error(), func(t *testing.T) {
			var logs bytes.Buffer
			_, _, _, d := newLoggedFixture(t, &logs)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			d.tasks = []PeriodicTask{&periodicProbe{fn: func(context.Context) error { cancel(); return failure }}}
			d.runTasks(ctx)
			canceled := errors.Is(failure, context.Canceled) || errors.Is(failure, db.ErrCanceled)
			if canceled {
				if strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), `"outcome":"canceled"`) {
					t.Fatal(logs.String())
				}
			} else if !strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), `"err":`) || !strings.Contains(logs.String(), failure.Error()) {
				t.Fatal(logs.String())
			}
		})
	}
}
