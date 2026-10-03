package securetoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// New returns a 256-bit random token (reset, verification, invitation, session; DD-6) and its
// hash: raw is what goes in the link or cookie, hash is what gets stored, so a leaked database
// never exposes a usable token.
func New() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, Hash(raw), nil
}

// Hash is SHA-256 of raw: fast on purpose (unlike password hashing, this is high-entropy random
// data, not something to slow a brute force of).
func Hash(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
