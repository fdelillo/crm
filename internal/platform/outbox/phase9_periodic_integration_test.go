//go:build integration

package outbox

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

type periodicProbe struct {
	calls int
	fn    func(context.Context) error
}

func (*periodicProbe) Name() string         { return "hourly_probe" }
func (*periodicProbe) Every() time.Duration { return time.Hour }
func (p *periodicProbe) Run(ctx context.Context) error {
	p.calls++
	if p.fn != nil {
		return p.fn(ctx)
	}
	return nil
}

func TestPhase9PeriodicSchedulingAndFailure(t *testing.T) {
	var logs bytes.Buffer
	runner, id, handler, d := newLoggedFixture(t, &logs)
	now := d.clock.Now()
	task := &periodicProbe{fn: func(context.Context) error { return errors.New("probe failure") }}
	d.tasks = []PeriodicTask{task}
	d.runTasks(context.Background())
	if task.calls != 1 || !strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), `"task":"hourly_probe"`) {
		t.Fatalf("first task: calls=%d logs=%s", task.calls, logs.String())
	}
	for i, elapsed := range []time.Duration{29 * time.Minute, time.Hour, time.Hour + 30*time.Minute, 2 * time.Hour} {
		d.clock = fakeClock{now.Add(elapsed)}
		enqueue(t, runner, d.clock, id, "email_verification")
		if err := d.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		d.runTasks(context.Background())
		want := 1
		if elapsed >= time.Hour {
			want = 2
		}
		if elapsed >= 2*time.Hour {
			want = 3
		}
		if task.calls != want {
			t.Errorf("elapsed=%s calls=%d want=%d", elapsed, task.calls, want)
		}
		if handler.Count() != i+1 {
			t.Errorf("worker stopped between tasks: sent=%d", handler.Count())
		}
	}
}

func TestPhase9PeriodicCancellationRollsBack(t *testing.T) {
	var logs bytes.Buffer
	runner, id, _, d := newLoggedFixture(t, &logs)
	var idMessage uuid.UUID
	if err := pgtest.SuperuserPool(t).QueryRow(context.Background(), `INSERT INTO app.outbox_messages(tenant_id,kind,template,recipient,payload,status,created_at,next_attempt_at,sent_at) VALUES($1,'email','password_reset','periodic@example.test',NULL,'sent',now()-interval '40 days',now()-interval '40 days',now()-interval '40 days') RETURNING id`, id).Scan(&idMessage); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	task := &periodicProbe{fn: func(ctx context.Context) error {
		return runner.InSystemTx(ctx, db.RoleWorker, func(ctx context.Context, tx db.Tx) error {
			tag, err := tx.Exec(ctx, `DELETE FROM app.outbox_messages WHERE id=$1`, idMessage)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				t.Fatal("cancellation probe did not delete inside transaction")
			}
			cancel()
			return context.Canceled
		})
	}}
	d.tasks = []PeriodicTask{task}
	d.runTasks(ctx)
	var exists bool
	if err := pgtest.SuperuserPool(t).QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM app.outbox_messages WHERE id=$1)`, idMessage).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("canceled periodic transaction committed")
	}
	if strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), `"level":"INFO"`) || !strings.Contains(logs.String(), `"task":"hourly_probe"`) {
		t.Fatalf("cancellation logs: %s", logs.String())
	}
}
