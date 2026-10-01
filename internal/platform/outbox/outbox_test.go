package outbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	want := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 6 * time.Hour, 6 * time.Hour}
	for i, expected := range want {
		if got := retryDelay(i + 1); got != expected {
			t.Errorf("attempt %d: %s != %s", i+1, got, expected)
		}
	}
}
func TestClassifyDeliveryError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want failureKind
	}{
		{context.Canceled, failureCanceled},
		{errors.New("temporary"), failureRecoverable},
		{&PermanentError{Err: errors.New("bad address")}, failurePermanent},
	} {
		if got := classifyFailure(tc.err); got != tc.want {
			t.Errorf("%v: %v != %v", tc.err, got, tc.want)
		}
	}
}
