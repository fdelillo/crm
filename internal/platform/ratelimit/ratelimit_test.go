package ratelimit

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/httpx"
	"golang.org/x/time/rate"
)

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }

func TestTokenBucketAndCleanup(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	limiter := NewLimiter(rate.Every(time.Hour/5), 5, clock, time.Hour)
	for i := 0; i < 5; i++ {
		if allowed, _ := limiter.Allow("a"); !allowed {
			t.Fatalf("request %d denied", i+1)
		}
	}
	if allowed, retry := limiter.Allow("a"); allowed || retry <= 0 {
		t.Fatalf("sixth: allowed=%v retry=%s", allowed, retry)
	}
	if allowed, _ := limiter.Allow("b"); !allowed {
		t.Fatal("independent key denied")
	}
	clock.now = clock.now.Add(time.Hour / 5)
	if allowed, _ := limiter.Allow("a"); !allowed {
		t.Fatal("token did not refill")
	}
	clock.now = clock.now.Add(2 * time.Hour)
	limiter.Allow("c")
	if len(limiter.buckets) != 1 {
		t.Fatalf("inactive keys retained: %d", len(limiter.buckets))
	}
}

// M3 of the PR #8 review: with idleTTL (10 min) shorter than a full refill (burst/limit = 1h for
// 5 requests/hour), a pause longer than idleTTL used to evict the bucket and recreate it with a
// full burst instead of the single token a real refill grants.
func TestNewLimiterRaisesIdleTTLToCoverFullRefill(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	limiter := NewLimiter(rate.Every(time.Hour/5), 5, clock, 10*time.Minute)
	for i := 0; i < 5; i++ {
		if allowed, _ := limiter.Allow("a"); !allowed {
			t.Fatalf("request %d denied", i+1)
		}
	}
	// One token refills every 12 minutes (5/hour): enough time for the old, too-short idleTTL to
	// have evicted the bucket at least once by now.
	clock.now = clock.now.Add(12*time.Minute + time.Second)
	if allowed, _ := limiter.Allow("a"); !allowed {
		t.Fatal("the naturally refilled token was denied")
	}
	if allowed, retry := limiter.Allow("a"); allowed || retry <= 0 {
		t.Fatalf("bucket was reset to a full burst instead of refilling one token: allowed=%v retry=%s", allowed, retry)
	}
}

func TestIPKey(t *testing.T) {
	ipv4 := netip.MustParseAddr("192.0.2.7")
	if got := IPKey(ipv4).String(); got != "192.0.2.7/32" {
		t.Fatalf("IPv4 key=%s", got)
	}
	if IPKey(netip.MustParseAddr("::ffff:192.0.2.7")) != IPKey(ipv4) {
		t.Fatal("mapped IPv4 key differs")
	}
	a := IPKey(netip.MustParseAddr("2001:db8:1:2:aaaa::1"))
	b := IPKey(netip.MustParseAddr("2001:db8:1:2:bbbb::9"))
	c := IPKey(netip.MustParseAddr("2001:db8:1:3::1"))
	if a != b || a == c || a.String() != "2001:db8:1:2::/64" {
		t.Fatalf("IPv6 keys %s %s %s", a, b, c)
	}
}

func TestMiddlewareUsesClientIPAndIPv6Prefix(t *testing.T) {
	cases := []struct {
		name         string
		remotes, xff []string
		trusted      []netip.Prefix
		lastStatus   int
	}{
		{"same IPv6 prefix", []string{"[2001:db8:1:2::1]:1234", "[2001:db8:1:2::ffff]:1234"}, nil, nil, 429},
		{"different IPv6 prefix", []string{"[2001:db8:1:2::1]:1234", "[2001:db8:1:3::1]:1234"}, nil, nil, 204},
		{"trusted proxy distinct clients", []string{"198.51.100.10:1234", "198.51.100.10:1234"},
			[]string{"192.0.2.1", "192.0.2.2"}, []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}, 204},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clock := &fakeClock{now: time.Now()}
			limiter := NewLimiter(rate.Every(time.Hour/5), 5, clock, time.Hour)
			called := 0
			h := httpx.ClientIP(tc.trusted, slog.New(slog.NewTextHandler(io.Discard, nil)))(Middleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called++
				w.WriteHeader(204)
			})))
			for i := 0; i < 6; i++ {
				r := httptest.NewRequest("POST", "/", nil)
				r.RemoteAddr = tc.remotes[0]
				if i == 5 {
					r.RemoteAddr = tc.remotes[1]
				}
				if len(tc.xff) > 0 {
					value := tc.xff[0]
					if i == 5 {
						value = tc.xff[1]
					}
					r.Header.Set("X-Forwarded-For", value)
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if i == 5 {
					if w.Code != tc.lastStatus {
						t.Fatalf("last status=%d want=%d", w.Code, tc.lastStatus)
					}
					if tc.lastStatus == 429 && w.Header().Get("Retry-After") == "" {
						t.Fatal("missing Retry-After")
					}
				}
			}
			wantCalled := 6
			if tc.lastStatus == 429 {
				wantCalled = 5
			}
			if called != wantCalled {
				t.Fatalf("handler called %d want %d", called, wantCalled)
			}
		})
	}
}

func TestMiddlewareCountsRejectedHandlerResponses(t *testing.T) {
	clock := &fakeClock{now: time.Now()}
	limiter := NewLimiter(rate.Every(time.Hour/5), 5, clock, time.Hour)
	called := 0
	h := httpx.ClientIP(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))(Middleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusForbidden)
	})))
	for i := 0; i < 6; i++ {
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = "192.0.2.7:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if i < 5 && w.Code != 403 {
			t.Fatalf("request %d status=%d", i+1, w.Code)
		}
		if i == 5 && w.Code != 429 {
			t.Fatalf("sixth status=%d", w.Code)
		}
	}
	if called != 5 {
		t.Fatalf("handler called %d times", called)
	}
}
