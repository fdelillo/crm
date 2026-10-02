package httpx

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
)

// DD-38 / INV-33 (T-B209): the User-Agent is chosen by the client, so what is stored must always
// satisfy the column's CHECK and PostgreSQL's text rules.
func TestNormalizeUserAgent(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"exactly 512", strings.Repeat("a", 512), strings.Repeat("a", 512)},
		{"513", strings.Repeat("a", 513), strings.Repeat("a", 512)},
		{"never half a character", strings.Repeat("a", 511) + "ñ" + "x", strings.Repeat("a", 511) + "ñ"},
		{"invalid byte", "\xff", "�"},
		{"each invalid byte", "a\xff\xfeb", "a��b"},
		{"NUL", "a\x00b", "ab"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeUserAgent(tc.in); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeUserAgentProperty(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 1000 {
		raw := make([]byte, rng.IntN(3000))
		for i := range raw {
			raw[i] = byte(rng.IntN(256))
		}
		out := NormalizeUserAgent(string(raw))
		if !utf8.ValidString(out) || strings.ContainsRune(out, 0) || utf8.RuneCountInString(out) > MaxUserAgentRunes {
			t.Fatalf("invalid output for %q: %q", raw, out)
		}
	}
}
