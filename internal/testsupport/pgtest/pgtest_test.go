package pgtest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fakeTB records Fatalf and stops the calling goroutine like the real one (via panic). Any Skip is a
// test error: an integration test must never skip silently (ADR-012).
type fakeTB struct {
	testing.TB
	fatal   string
	skipped bool
}

type fatalStop struct{}

func (f *fakeTB) Helper() {}
func (f *fakeTB) Fatalf(format string, args ...any) {
	f.fatal = fmt.Sprintf(format, args...)
	panic(fatalStop{})
}
func (f *fakeTB) Skip(...any)          { f.skipped = true; panic(fatalStop{}) }
func (f *fakeTB) Skipf(string, ...any) { f.skipped = true; panic(fatalStop{}) }
func (f *fakeTB) SkipNow()             { f.skipped = true; panic(fatalStop{}) }

// call runs fn and reports whether it ended with Fatalf.
func call(fn func()) (stopped bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fatalStop); !ok {
				panic(r)
			}
			stopped = true
		}
	}()
	fn()
	return false
}

// Without Docker (or with any start failure) the test FAILS with a clear message, every time, and
// never skips.
func TestProvider_StartFailureFailsLoudly(t *testing.T) {
	calls := 0
	p := &provider{start: func(context.Context) (*cluster, error) {
		calls++
		return nil, errors.New("cannot connect to the Docker daemon")
	}}
	for i := range 3 {
		f := &fakeTB{}
		if !call(func() { p.get(f) }) {
			t.Fatalf("call %d: get returned without failing the test", i)
		}
		if f.skipped {
			t.Fatalf("call %d: the test was skipped", i)
		}
		for _, want := range []string{"Docker is required", "make test-int", "cannot connect to the Docker daemon"} {
			if !strings.Contains(f.fatal, want) {
				t.Errorf("call %d: message %q lacks %q", i, f.fatal, want)
			}
		}
	}
	if calls != 1 {
		t.Errorf("start ran %d times, want exactly once per package", calls)
	}
}
