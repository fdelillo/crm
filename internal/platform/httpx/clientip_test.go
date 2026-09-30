package httpx

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("2001:db8:ffff::/48")}
	cases := []struct {
		name, remote string
		xff          []string
		want         string
		untrusted    bool
		warn         bool
	}{
		{"untrusted", "203.0.113.5:4711", []string{"192.0.2.66"}, "203.0.113.5", false, false},
		{"empty trust", "203.0.113.5:4711", []string{"192.0.2.66"}, "203.0.113.5", true, false},
		{"absent", "198.51.100.10:4711", nil, "198.51.100.10", false, false},
		{"one hop", "198.51.100.10:4711", []string{"192.0.2.66"}, "192.0.2.66", false, false},
		{"spoofed left", "198.51.100.10:4711", []string{"192.0.2.99, 192.0.2.66"}, "192.0.2.66", false, false},
		{"two proxies", "198.51.100.10:4711", []string{"192.0.2.66, 198.51.100.20"}, "192.0.2.66", false, false},
		{"all trusted", "198.51.100.10:4711", []string{"198.51.100.30, 198.51.100.20"}, "198.51.100.30", false, false},
		{"multiple headers", "198.51.100.10:4711", []string{"192.0.2.99", "192.0.2.66"}, "192.0.2.66", false, false},
		{"invalid right", "198.51.100.10:4711", []string{"192.0.2.66, basura"}, "198.51.100.10", false, true},
		{"invalid left", "198.51.100.10:4711", []string{"basura, 192.0.2.66"}, "192.0.2.66", false, false},
		{"v4 port", "198.51.100.10:4711", []string{"192.0.2.66:5555"}, "192.0.2.66", false, false},
		{"v6 port", "198.51.100.10:4711", []string{"[2001:db8::7]:5555"}, "2001:db8::7", false, false},
		{"mapped", "[::ffff:198.51.100.10]:4711", []string{"::ffff:192.0.2.66"}, "192.0.2.66", false, false},
		{"v6 trust", "[2001:db8:ffff::1]:4711", []string{"2001:db8:1::5"}, "2001:db8:1::5", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var log bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&log, nil))
			previous := slog.Default()
			slog.SetDefault(logger)
			defer slog.SetDefault(previous)
			prefixes := trusted
			if tc.untrusted {
				prefixes = nil
			}
			var got netip.Addr
			h := ClientIP(prefixes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = ClientIPFrom(r.Context())
			}))
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.remote
			for _, h := range tc.xff {
				r.Header.Add("X-Forwarded-For", h)
			}
			h.ServeHTTP(httptest.NewRecorder(), r)
			if !got.IsValid() || got.String() != tc.want {
				t.Fatalf("got %v want %s", got, tc.want)
			}
			if tc.warn && (!strings.Contains(log.String(), "bad_forwarded_for") || strings.Contains(log.String(), "basura")) {
				t.Fatalf("warning=%q", log.String())
			}
		})
	}
	if ClientIPFrom(context.Background()).IsValid() {
		t.Fatal("IP outside middleware must be invalid")
	}
}
