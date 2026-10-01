package securetoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func New() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, Hash(raw), nil
}

func Hash(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
