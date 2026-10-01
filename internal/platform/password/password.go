package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

type Hasher interface {
	Hash(ctx context.Context, plain string) (string, error)
	Verify(ctx context.Context, plain, encoded string) (ok bool, needsRehash bool, err error)
	VerifyDummy(ctx context.Context, plain string)
}

type parameters struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

var current = parameters{memory: 19456, iterations: 2, parallelism: 1}

type deriveFunc func(plain, salt []byte, params parameters) []byte
type hasher struct {
	sem    chan struct{}
	params parameters
	derive deriveFunc
	dummy  string
}

func NewHasher(concurrency int) Hasher { return newHasher(concurrency, current, nil) }
func newHasher(concurrency int, params parameters, derive deriveFunc) *hasher {
	if concurrency < 1 {
		concurrency = 4
	}
	if derive == nil {
		derive = func(plain, salt []byte, p parameters) []byte {
			return argon2.IDKey(plain, salt, p.iterations, p.memory, p.parallelism, 32)
		}
	}
	h := &hasher{sem: make(chan struct{}, concurrency), params: params, derive: derive}
	salt := []byte("crm-dummy-salt!!")
	h.dummy = encode(params, salt, derive([]byte("dummy-password"), salt, params))
	return h
}

func (h *hasher) acquire(ctx context.Context) error {
	select {
	case h.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (h *hasher) release() { <-h.sem }

func (h *hasher) Hash(ctx context.Context, plain string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	return encode(h.params, salt, h.derive([]byte(plain), salt, h.params)), nil
}

func (h *hasher) Verify(ctx context.Context, plain, encoded string) (bool, bool, error) {
	params, salt, expected, err := parse(encoded)
	if err != nil {
		return false, false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	defer h.release()
	actual := h.derive([]byte(plain), salt, params)
	ok := subtle.ConstantTimeCompare(actual, expected) == 1
	// Key length is not part of parameters: every hash this package has ever produced derives with
	// the same hardcoded 32-byte key (see derive above), so comparing it could never trigger a
	// rehash. Only the three Argon2id cost parameters can actually fall behind current.
	rehash := params.memory < h.params.memory || params.iterations < h.params.iterations ||
		params.parallelism < h.params.parallelism
	return ok, ok && rehash, nil
}

func (h *hasher) VerifyDummy(ctx context.Context, plain string) {
	_, _, _ = h.Verify(ctx, plain, h.dummy)
}

func encode(p parameters, salt, hash []byte) string {
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", p.memory, p.iterations, p.parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash))
}
func parse(encoded string) (parameters, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return parameters{}, nil, nil, errors.New("invalid argon2id PHC")
	}
	fields := strings.Split(parts[3], ",")
	if len(fields) != 3 {
		return parameters{}, nil, nil, errors.New("invalid argon2id parameters")
	}
	number := func(s, prefix string, bits int) (uint64, error) {
		value, ok := strings.CutPrefix(s, prefix)
		if !ok {
			return 0, errors.New("invalid argon2id parameter")
		}
		return strconv.ParseUint(value, 10, bits)
	}
	memory, err := number(fields[0], "m=", 32)
	if err != nil {
		return parameters{}, nil, nil, err
	}
	iterations, err := number(fields[1], "t=", 32)
	if err != nil {
		return parameters{}, nil, nil, err
	}
	parallelism, err := number(fields[2], "p=", 8)
	if err != nil {
		return parameters{}, nil, nil, err
	}
	// A corrupt database value must not allocate unbounded memory during verification.
	if memory < 8*parallelism || memory > 65536 || iterations == 0 || iterations > 10 || parallelism == 0 || parallelism > 4 {
		return parameters{}, nil, nil, errors.New("argon2id parameters out of range")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return parameters{}, nil, nil, errors.New("invalid argon2id salt")
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) == 0 || len(hash) > 64 {
		return parameters{}, nil, nil, errors.New("invalid argon2id hash")
	}
	return parameters{uint32(memory), uint32(iterations), uint8(parallelism)}, salt, hash, nil
}

// Validate returns the stable field error code, or an empty string when valid.
func Validate(plain, email string) string {
	length := utf8.RuneCountInString(plain)
	if length < 10 {
		return "too_short"
	}
	if length > 128 {
		return "too_long"
	}
	if strings.EqualFold(plain, strings.TrimSpace(email)) {
		return "same_as_email"
	}
	return ""
}
