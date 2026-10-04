package identity

import (
	"testing"
	"time"
)

func TestNextThrottle(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	state := throttle{}
	for i := int32(1); i <= 5; i++ {
		var locked bool
		var retry time.Duration
		state, locked, retry = nextThrottle(state, false, now)
		if locked || retry != 0 || state.FailedCount != i {
			t.Fatalf("failure %d: state=%+v locked=%v retry=%v", i, state, locked, retry)
		}
		if i < 5 && state.LockedUntil != nil {
			t.Fatalf("early lock after %d failures", i)
		}
	}
	if state.LockedUntil == nil || !state.LockedUntil.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("fifth failure: %+v", state)
	}
	before := state
	got, locked, retry := nextThrottle(state, true, now.Add(time.Minute))
	if !locked || retry != 14*time.Minute || got.FailedCount != before.FailedCount || !got.LockedUntil.Equal(*before.LockedUntil) {
		t.Fatalf("locked attempt changed state: %+v %v %v", got, locked, retry)
	}
	got, locked, retry = nextThrottle(state, false, now.Add(15*time.Minute+time.Second))
	if locked || retry != 0 || got.FailedCount != 1 || got.LockedUntil != nil {
		t.Fatalf("expired lock: %+v %v %v", got, locked, retry)
	}
	got, locked, retry = nextThrottle(throttle{FailedCount: 3, LastFailedAt: now}, true, now.Add(time.Hour))
	if locked || retry != 0 || got.FailedCount != 0 {
		t.Fatalf("successful login: %+v %v %v", got, locked, retry)
	}
}
