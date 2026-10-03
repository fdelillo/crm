package clock

import "time"

// Clock is the single source of "now" for code that needs to fake time in tests (DD-18): outbox
// backoff, the rate limiter, and anything else that would otherwise call time.Now() directly.
type Clock interface{ Now() time.Time }

// Real is the production Clock: time.Now().
type Real struct{}

func (Real) Now() time.Time { return time.Now() }
