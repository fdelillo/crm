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
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Hasher is argon2id password hashing (ADR-007), bounded to a fixed concurrency so a burst of
// logins cannot exhaust memory (argon2id's cost is memory, not just CPU).
type Hasher interface {
	Hash(ctx context.Context, plain string) (string, error)
	// Verify also reports needsRehash: true when encoded used weaker parameters than current, so
	// the caller can re-hash the plaintext it already has while it is still available.
	Verify(ctx context.Context, plain, encoded string) (ok bool, needsRehash bool, err error)
	// VerifyDummy spends the same work as Verify against a fixed dummy hash, so a login attempt
	// for an email that does not exist takes as long as one that does (timing side channel).
	VerifyDummy(ctx context.Context, plain string)
	// Acquire reserves one of the concurrent derivations and returns it as a Slot (DD-45).
	Acquire(ctx context.Context) (Slot, error)
}

// Slot is a reserved derivation of the Hasher's semaphore. Its methods never wait for the
// semaphore, so a flow that holds a database connection reserves the Slot first (INV-36) and
// derives with it afterwards. Release is idempotent.
type Slot interface {
	Hash(plain string) (string, error)
	Verify(plain, encoded string) (ok bool, needsRehash bool, err error)
	VerifyDummy(plain string)
	Release()
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

// NewHasher returns a Hasher that runs at most concurrency derivations at once (a semaphore, not a
// pool): concurrency < 1 defaults to 4.
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

// derive* do the work without touching the semaphore: the caller already holds a seat.
func (h *hasher) deriveHash(plain string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return encode(h.params, salt, h.derive([]byte(plain), salt, h.params)), nil
}

func (h *hasher) deriveVerify(plain, encoded string) (bool, bool, error) {
	params, salt, expected, err := parse(encoded)
	if err != nil {
		return false, false, err
	}
	actual := h.derive([]byte(plain), salt, params)
	ok := subtle.ConstantTimeCompare(actual, expected) == 1
	// Key length is not part of parameters: every hash this package has ever produced derives with
	// the same hardcoded 32-byte key (see derive above), so comparing it could never trigger a
	// rehash. Only the three Argon2id cost parameters can actually fall behind current.
	rehash := params.memory < h.params.memory || params.iterations < h.params.iterations ||
		params.parallelism < h.params.parallelism
	return ok, ok && rehash, nil
}

// Hash, Verify and VerifyDummy take a seat for the length of one derivation: they are for callers
// that hold no database connection (the registration hashes before its transaction).
func (h *hasher) Hash(ctx context.Context, plain string) (string, error) {
	s, err := h.Acquire(ctx)
	if err != nil {
		return "", err
	}
	defer s.Release()
	return s.Hash(plain)
}

func (h *hasher) Verify(ctx context.Context, plain, encoded string) (bool, bool, error) {
	// A corrupt hash fails before it asks for a seat, as before.
	if _, _, _, err := parse(encoded); err != nil {
		return false, false, err
	}
	s, err := h.Acquire(ctx)
	if err != nil {
		return false, false, err
	}
	defer s.Release()
	return s.Verify(plain, encoded)
}

func (h *hasher) VerifyDummy(ctx context.Context, plain string) {
	_, _, _ = h.Verify(ctx, plain, h.dummy)
}

// Acquire waits for a seat of the semaphore or for ctx (DD-45).
func (h *hasher) Acquire(ctx context.Context) (Slot, error) {
	if err := h.acquire(ctx); err != nil {
		return nil, err
	}
	return &slot{h: h}, nil
}

type slot struct {
	h    *hasher
	once sync.Once
}

func (s *slot) Hash(plain string) (string, error) { return s.h.deriveHash(plain) }
func (s *slot) Verify(plain, encoded string) (bool, bool, error) {
	return s.h.deriveVerify(plain, encoded)
}
func (s *slot) VerifyDummy(plain string) { _, _, _ = s.h.deriveVerify(plain, s.h.dummy) }
func (s *slot) Release()                 { s.once.Do(s.h.release) }

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
