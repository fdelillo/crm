package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

// ClientIPFrom returns the selected address, or the invalid zero value outside ClientIP.
func ClientIPFrom(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(clientIPKey{}).(netip.Addr)
	return ip
}

// ClientIP accepts X-Forwarded-For only from explicitly trusted peer proxies (DD-32). logger is
// required (not the global slog.Default()): with the server's own JSON handler not installed as
// the default (cmd/crm/serve.go never calls slog.SetDefault), a global log call would print in
// plain text, outside the request's JSON line and without its request_id (M4 of the PR #8 review).
func ClientIP(trusted []netip.Prefix, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, err := selectClientIP(r.RemoteAddr, r.Header.Values("X-Forwarded-For"), trusted)
			if err != nil {
				logger.WarnContext(r.Context(), "invalid forwarded address", "event", "bad_forwarded_for")
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip)))
		})
	}
}

// selectClientIP walks from the peer toward the client. The first untrusted valid address
// is authoritative; entries further left may be supplied by the client and are never read.
func selectClientIP(remote string, values []string, trusted []netip.Prefix) (netip.Addr, error) {
	peer, err := parseAddress(remote)
	if err != nil {
		return netip.Addr{}, err
	}
	if !containsIP(trusted, peer) {
		return peer, nil
	}
	current := peer
	for valueIndex := len(values) - 1; valueIndex >= 0; valueIndex-- {
		parts := strings.Split(values[valueIndex], ",")
		for i := len(parts) - 1; i >= 0; i-- {
			candidate, err := parseAddress(strings.TrimSpace(parts[i]))
			if err != nil {
				return peer, errors.New("bad forwarded address")
			}
			current = candidate
			if !containsIP(trusted, candidate) {
				return candidate, nil
			}
		}
	}
	return current, nil
}

func containsIP(prefixes []netip.Prefix, ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, prefix := range prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func parseAddress(raw string) (netip.Addr, error) {
	if ip, err := netip.ParseAddr(raw); err == nil {
		return ip.Unmap(), nil
	}
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		return netip.Addr{}, err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, err
	}
	return ip.Unmap(), nil
}
