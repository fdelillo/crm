package identity

import (
	"strings"
	"testing"
)

func TestValidEmail(t *testing.T) {
	for _, tc := range []struct {
		name, email string
		valid       bool
	}{
		{"normal", "person@example.com", true},
		{"254 bytes", strings.Repeat("a", 242) + "@example.com", true},
		{"255 bytes", strings.Repeat("a", 243) + "@example.com", false},
		{"empty", "", false},
		{"missing domain", "person@", false},
		{"display name", "Person <person@example.com>", false},
		{"nul", "person\x00@example.com", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidEmail(tc.email); got != tc.valid {
				t.Fatalf("valid=%v want=%v", got, tc.valid)
			}
		})
	}
}
