package password

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHashVerifyPHC(t *testing.T) {
	h := NewHasher(4)
	a, err := h.Hash(context.Background(), "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	b, err := h.Hash(context.Background(), "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, "$argon2id$v=19$m=19456,t=2,p=1$") || a == b {
		t.Fatalf("bad PHC or salt: %q %q", a, b)
	}
	ok, rehash, err := h.Verify(context.Background(), "correct horse", a)
	if err != nil || !ok || rehash {
		t.Fatalf("correct: %v %v %v", ok, rehash, err)
	}
	ok, _, err = h.Verify(context.Background(), "wrong", a)
	if err != nil || ok {
		t.Fatalf("wrong: %v %v", ok, err)
	}
	if _, _, err := h.Verify(context.Background(), "pass", "broken"); err == nil {
		t.Fatal("corrupt PHC accepted")
	}
	// A changed parameter invalidates the hash. Generate an actual weaker hash for this check.
	weakHasher := newHasher(4, parameters{memory: 8192, iterations: 2, parallelism: 1}, nil)
	weaker, err := weakHasher.Hash(context.Background(), "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	ok, rehash, err = h.Verify(context.Background(), "correct horse", weaker)
	if err != nil || !ok || !rehash {
		t.Fatalf("weak: %v %v %v", ok, rehash, err)
	}
}

func TestValidation(t *testing.T) {
	for _, tc := range []struct{ plain, email, want string }{
		{"123456789", "a@example.com", "too_short"},
		{"1234567890", "a@example.com", ""},
		{strings.Repeat("a", 128), "a@example.com", ""},
		{strings.Repeat("a", 129), "a@example.com", "too_long"},
		{"A@Example.Com", "a@example.com", "same_as_email"},
	} {
		if got := Validate(tc.plain, tc.email); got != tc.want {
			t.Fatalf("Validate(%q)=%q want %q", tc.plain, got, tc.want)
		}
	}
}

func TestSemaphore(t *testing.T) {
	var active, peak atomic.Int32
	derive := func(_ []byte, _ []byte, _ parameters) []byte {
		n := active.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return make([]byte, 32)
	}
	h := newHasher(4, parameters{memory: 19456, iterations: 2, parallelism: 1}, derive)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h.Hash(context.Background(), "password123"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := peak.Load(); got != 4 {
		t.Fatalf("peak=%d want 4", got)
	}
}

func TestVerifyDummyCost(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	h := NewHasher(4)
	encoded, err := h.Hash(context.Background(), "password123")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, _, err = h.Verify(context.Background(), "password123", encoded)
	if err != nil {
		t.Fatal(err)
	}
	verifyTime := time.Since(start)
	start = time.Now()
	h.VerifyDummy(context.Background(), "password123")
	dummyTime := time.Since(start)
	if dummyTime < verifyTime/10 || dummyTime > verifyTime*10 {
		t.Fatalf("verify=%s dummy=%s", verifyTime, dummyTime)
	}
}

// Parameters small enough for a test, with the real argon2id derivation and one derivation at a time.
func slotHasher() *hasher {
	return newHasher(1, parameters{memory: 64, iterations: 1, parallelism: 1}, nil)
}

func acquireSlot(t *testing.T, h Hasher) Slot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s, err := h.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	return s
}

func TestSlotVerifyDoesNotWaitForTheSemaphore(t *testing.T) { // P1
	h := slotHasher()
	encoded, err := h.Hash(context.Background(), "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	s := acquireSlot(t, h)
	defer s.Release()
	done := make(chan bool, 1)
	go func() { ok, _, _ := s.Verify("correct horse", encoded); done <- ok }()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("Verify rejected the right password")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Slot.Verify waited for the semaphore its own Slot already holds")
	}
}

func TestSlotHashAndVerifyDummyDoNotWaitForTheSemaphore(t *testing.T) { // P2
	h := slotHasher()
	s := acquireSlot(t, h)
	done := make(chan string, 1)
	go func() {
		hash, err := s.Hash("correct horse")
		if err != nil {
			hash = ""
		}
		s.VerifyDummy("whatever")
		done <- hash
	}()
	var hash string
	select {
	case hash = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Slot.Hash or Slot.VerifyDummy waited for the semaphore")
	}
	s.Release()
	ok, _, err := h.Verify(context.Background(), "correct horse", hash)
	if err != nil || !ok {
		t.Fatalf("the hash of the Slot does not verify: ok=%v err=%v", ok, err)
	}
}

func TestSlotReleaseIsIdempotent(t *testing.T) { // P3
	h := slotHasher()
	s := acquireSlot(t, h)
	s.Release()
	_ = acquireSlot(t, h) // keeps the only seat until the end of the test
	// A second Release that freed a seat it no longer owns would let two Slots in at once.
	s.Release()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if second, err := h.Acquire(ctx); err == nil {
		second.Release()
		t.Fatal("a second Acquire got in while the only seat is taken")
	}
}

func TestAcquireWithCanceledContextDoesNotBlock(t *testing.T) { // P4
	h := slotHasher()
	s := acquireSlot(t, h)
	defer s.Release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { _, err := h.Acquire(ctx); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire with a canceled context blocked")
	}
}

func TestHashWaitsForAHeldSlot(t *testing.T) { // P5
	h := slotHasher()
	s := acquireSlot(t, h)
	done := make(chan struct{})
	go func() { _, _ = h.Hash(context.Background(), "correct horse"); close(done) }()
	select {
	case <-done:
		t.Fatal("Hasher.Hash ran while a Slot held the only seat")
	case <-time.After(150 * time.Millisecond):
	}
	s.Release()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Hasher.Hash did not resume after Release")
	}
}

func TestSlotFailsClosedAfterRelease(t *testing.T) {
	var derivations atomic.Int32
	h := newHasher(1, parameters{memory: 64, iterations: 1, parallelism: 1}, func(plain, salt []byte, p parameters) []byte {
		derivations.Add(1)
		return make([]byte, 32)
	})
	encoded, err := h.Hash(context.Background(), "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	s := acquireSlot(t, h)
	s.Release()
	derivations.Store(0) // the constructor and the setup Hash above derive; only the calls after Release count
	if _, err := s.Hash("x"); !errors.Is(err, ErrSlotReleased) {
		t.Fatalf("Hash after Release: %v", err)
	}
	if ok, _, err := s.Verify("correct horse", encoded); ok || !errors.Is(err, ErrSlotReleased) {
		t.Fatalf("Verify after Release: ok=%v err=%v", ok, err)
	}
	s.VerifyDummy("x")
	if n := derivations.Load(); n != 0 {
		t.Fatalf("a released slot derived %d times", n)
	}
}
