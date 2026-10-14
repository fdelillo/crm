package ratelimit

import (
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"golang.org/x/time/rate"
)

// IPKey is the rate-limit key of ip (DD-9, DD-32): the full address for IPv4, the /64 for IPv6
// (one client usually controls a whole /64 and could otherwise rotate addresses within it).
func IPKey(ip netip.Addr) netip.Prefix {
	if !ip.IsValid() {
		return netip.Prefix{}
	}
	ip = ip.Unmap()
	if ip.Is4() {
		return netip.PrefixFrom(ip, 32)
	}
	return netip.PrefixFrom(ip, 64).Masked()
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter is a token bucket per key (DD-9), with idle buckets evicted so memory does not grow
// without bound.
type Limiter struct {
	mu          sync.Mutex
	buckets     map[string]*bucket
	limit       rate.Limit
	burst       int
	clock       clock.Clock
	idleTTL     time.Duration
	nextCleanup time.Time
}

// NewLimiter returns a token-bucket Limiter of limit events/second with the given burst, keyed by
// caller (DD-9). idleTTL is raised to at least burst/limit if it is shorter (M3 of the PR #8
// review): a bucket evicted for being idle, then recreated on the next request, starts full again,
// so an idleTTL shorter than the time a full refill takes would let a caller that paces its
// requests just past idleTTL get more than burst requests per refill window.
func NewLimiter(limit rate.Limit, burst int, c clock.Clock, idleTTL time.Duration) *Limiter {
	if limit <= 0 || burst <= 0 || idleTTL <= 0 || c == nil {
		panic("ratelimit: invalid configuration")
	}
	if refill := time.Duration(float64(burst) / float64(limit) * float64(time.Second)); refill > idleTTL {
		idleTTL = refill
	}
	return &Limiter{buckets: map[string]*bucket{}, limit: limit, burst: burst, clock: c, idleTTL: idleTTL}
}

// Allow spends one token before the request reaches its handler. A rejected reservation is
// canceled immediately, so it cannot move the next admitted request further into the future.
func (l *Limiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	now := l.clock.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if !now.Before(l.nextCleanup) {
		for name, entry := range l.buckets {
			if now.Sub(entry.lastSeen) > l.idleTTL {
				delete(l.buckets, name)
			}
		}
		l.nextCleanup = now.Add(l.idleTTL / 2)
	}
	entry := l.buckets[key]
	if entry == nil {
		entry = &bucket{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.buckets[key] = entry
	}
	entry.lastSeen = now
	if entry.limiter.AllowN(now, 1) {
		return true, 0
	}
	reservation := entry.limiter.ReserveN(now, 1)
	if !reservation.OK() {
		return false, time.Hour
	}
	retryAfter = reservation.DelayFrom(now)
	reservation.CancelAt(now)
	return false, retryAfter
}

// KeyFunc extracts the key of a request's quota. ok == false is a wiring bug (the route lacks what
// the key needs), and the middleware answers 500 (DD-48).
type KeyFunc func(r *http.Request) (key string, ok bool)

func ipKeyFunc(r *http.Request) (string, bool) {
	ip := httpx.ClientIPFrom(r.Context())
	if !ip.IsValid() {
		return "", false
	}
	return IPKey(ip).String(), true
}

// Middleware is registered within a chi route group, after the common ClientIP middleware. It
// limits by client address.
func Middleware(limiter *Limiter) func(http.Handler) http.Handler {
	return MiddlewareBy(limiter, ipKeyFunc)
}

// MiddlewareBy spends one token of the quota of key(r) before the request reaches its handler:
// by company (DD-48), by user (DD-46 (3)) or with a constant key (DD-49). Over the quota: 429
// rate_limited with Retry-After.
func MiddlewareBy(limiter *Limiter, key KeyFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			k, ok := key(r)
			if !ok {
				httpx.WriteProblem(w, r, httpx.CodeInternal)
				return
			}
			allowed, retry := limiter.Allow(k)
			if !allowed {
				httpx.WriteProblem(w, r, httpx.CodeRateLimited, httpx.RetryAfter(retry))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
