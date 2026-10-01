package password

import (
	"context"
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
