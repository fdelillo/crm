package securetoken

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestNewAndHash(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		raw, hash, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if seen[raw] {
			t.Fatal("duplicate token")
		}
		seen[raw] = true
		if len(raw) != 43 || len(hash) != sha256.Size {
			t.Fatalf("lengths %d %d", len(raw), len(hash))
		}
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil || len(decoded) != 32 {
			t.Fatalf("raw=%q err=%v", raw, err)
		}
		if string(hash) != string(Hash(raw)) {
			t.Fatal("hash mismatch")
		}
		sum := sha256.Sum256([]byte(raw))
		if string(hash) != string(sum[:]) {
			t.Fatal("not SHA-256")
		}
	}
}
