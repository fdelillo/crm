package outbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox/store"
	"github.com/google/uuid"
)

// fakeRunner is a db.TxRunner double for unit tests of Dispatcher.deferMessage that only need to
// know whether the transaction it runs succeeds or fails: it never calls fn, so it needs no real
// database or db.Tx (the integration tests exercise the real queries).
type fakeRunner struct{ err error }

func (f fakeRunner) InTenantTx(context.Context, uuid.UUID, func(context.Context, db.Tx) error) error {
	return f.err
}
func (f fakeRunner) InSystemTx(context.Context, db.SystemRole, func(context.Context, db.Tx) error) error {
	return f.err
}

func TestRetryDelay(t *testing.T) {
	want := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 6 * time.Hour, 6 * time.Hour}
	for i, expected := range want {
		if got := retryDelay(i + 1); got != expected {
			t.Errorf("attempt %d: %s != %s", i+1, got, expected)
		}
	}
}

func TestDeferDelay(t *testing.T) {
	for _, tc := range []struct {
		age  time.Duration
		want time.Duration
	}{
		{-5 * time.Second, 10 * time.Second},
		{0, 10 * time.Second},
		{9 * time.Second, 10 * time.Second},
		{12 * time.Second, 12 * time.Second},
		{14 * time.Minute, 14 * time.Minute},
		{20 * time.Minute, 15 * time.Minute},
	} {
		if got := deferDelay(tc.age); got != tc.want {
			t.Errorf("age %s: got %s want %s", tc.age, got, tc.want)
		}
	}
}

func TestCausePermanentAndLogLevel(t *testing.T) {
	for _, tc := range []struct {
		cause     Cause
		permanent bool
		level     slog.Level
	}{
		{CauseNetwork, false, slog.LevelWarn},
		{CauseTransient, false, slog.LevelWarn},
		{CauseConfig, false, slog.LevelError},
		{CauseRecipient, true, slog.LevelWarn},
		{CauseBug, true, slog.LevelError},
	} {
		if got := tc.cause.Permanent(); got != tc.permanent {
			t.Errorf("%s.Permanent()=%v want %v", tc.cause, got, tc.permanent)
		}
		if got := tc.cause.LogLevel(); got != tc.level {
			t.Errorf("%s.LogLevel()=%v want %v", tc.cause, got, tc.level)
		}
	}
}

func TestDeliveryErrorLastErrorFormat(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  *DeliveryError
		want string
	}{
		{"code and extended", &DeliveryError{Cause: CauseConfig, Phase: PhaseConnection, SMTPCode: 535, Enhanced: "5.7.8", Detail: "Authentication credentials invalid"},
			"config connection 535 5.7.8: Authentication credentials invalid"},
		{"code without extended", &DeliveryError{Cause: CauseRecipient, Phase: PhaseRcptTo, SMTPCode: 550, Detail: "User unknown"},
			"recipient rcpt_to 550: User unknown"},
		{"no code", &DeliveryError{Cause: CauseNetwork, Phase: PhaseConnection, Detail: "timeout"},
			"network connection: timeout"},
		{"phase unknown", &DeliveryError{Cause: CauseConfig, Phase: PhaseUnknown, Detail: "unclassified handler error"},
			"config unknown: unclassified handler error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.LastError(); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestDeliveryErrorSanitizesControlCharactersAndSpaces(t *testing.T) {
	de := &DeliveryError{Cause: CauseConfig, Phase: PhaseConnection, Detail: "line one\r\nline\ttwo   three  "}
	got := de.LastError()
	if strings.ContainsAny(got, "\r\n\t") {
		t.Fatalf("control characters survived: %q", got)
	}
	if got != "config connection: line one line two three" {
		t.Fatalf("got %q", got)
	}
}

func TestDeliveryErrorRedactsAddresses(t *testing.T) {
	de := &DeliveryError{Cause: CauseRecipient, Phase: PhaseRcptTo, SMTPCode: 550,
		Detail: `<a@b.example> a@b.example, "a@b.example"`}
	got := de.LastError()
	if strings.Contains(got, "@") {
		t.Fatalf("address survived: %q", got)
	}
	want := "recipient rcpt_to 550: [redacted] [redacted] [redacted]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDeliveryErrorTruncatesToRunes(t *testing.T) {
	de := &DeliveryError{Cause: CauseConfig, Phase: PhaseData, Detail: strings.Repeat("á", 2000)}
	got := []rune(de.LastError())
	if len(got) != maxLastErrorRunes {
		t.Fatalf("length=%d want %d", len(got), maxLastErrorRunes)
	}
	if got[len(got)-1] != '…' {
		t.Fatalf("did not end in an ellipsis: %q", string(got[len(got)-5:]))
	}
}

func TestDeliveryErrorUnwrapNeverExposedByLastError(t *testing.T) {
	cause := errors.New("SMTP failed, affected recipient(s): user@example.com")
	de := &DeliveryError{Cause: CauseNetwork, Phase: PhaseConnection, Detail: "connection refused", Err: cause}
	if !errors.Is(de, cause) {
		t.Fatal("Unwrap does not expose Err to errors.Is")
	}
	if strings.Contains(de.LastError(), "user@example.com") {
		t.Fatalf("LastError leaked the wrapped error: %q", de.LastError())
	}
}

func TestErrorsAsRecoversDeliveryErrorThroughWrapping(t *testing.T) {
	de := &DeliveryError{Cause: CauseBug, Phase: PhaseCompose, Detail: "unknown email template"}
	wrapped := fmt.Errorf("handler: %w", de)
	var got *DeliveryError
	if !errors.As(wrapped, &got) || got != de {
		t.Fatalf("errors.As did not recover the DeliveryError: %v", got)
	}
}

func TestClassifyHandleResult(t *testing.T) {
	budgetTimeout := &DeliveryError{Cause: CauseNetwork, Phase: PhaseUnknown, Detail: "timeout"}
	for _, tc := range []struct {
		name       string
		err        error
		cycleAlive bool
		wantCause  Cause
		canceled   bool
		unclass    bool
	}{
		{"delivery error passthrough", &DeliveryError{Cause: CauseTransient, Phase: PhaseRcptTo}, true, CauseTransient, false, false},
		{"budget timeout, cycle alive", context.DeadlineExceeded, true, budgetTimeout.Cause, false, false},
		{"cycle canceled", context.Canceled, true, "", true, false},
		{"cycle deadline exceeded, not alive", context.DeadlineExceeded, false, "", true, false},
		{"bare error is unclassified", errors.New("boom"), true, CauseConfig, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			de, canceled, unclassified := classifyHandleResult(tc.err, tc.cycleAlive)
			if canceled != tc.canceled || unclassified != tc.unclass {
				t.Fatalf("canceled=%v unclassified=%v", canceled, unclassified)
			}
			if !tc.canceled && de.Cause != tc.wantCause {
				t.Fatalf("cause=%v want %v", de.Cause, tc.wantCause)
			}
		})
	}
}

// N1 of the second PR #8 review: a dial/TLS timeout from go-mail wraps context.DeadlineExceeded
// inside a connection-phase *DeliveryError (classifyConnectionError in platform/mailer); errors.Is
// traverses Unwrap, so checking the bare context.DeadlineExceeded branch before errors.As would
// replace the connection phase with "unknown" and the cycle would not stop (ADR-024 §4, INV-31).
func TestClassifyHandleResultPreservesConnectionPhaseOverWrappedDeadlineExceeded(t *testing.T) {
	original := &DeliveryError{Phase: PhaseConnection, Err: fmt.Errorf("dial failed: %w", context.DeadlineExceeded)}
	de, canceled, unclassified := classifyHandleResult(original, true)
	if canceled || unclassified {
		t.Fatalf("canceled=%v unclassified=%v", canceled, unclassified)
	}
	if de != original {
		t.Fatalf("got %v want the original *DeliveryError", de)
	}
	if de.Phase != PhaseConnection {
		t.Fatalf("phase=%v want %v", de.Phase, PhaseConnection)
	}
}

// Nit of the second PR #8 review (dispatcher.go:256-257, 347): if the deferral itself fails, the
// original per-message failure (cause) must still be visible in the error RunOnce logs, not
// replaced by the deferral's own error.
func TestDeferMessageIncludesOriginalCauseWhenTheDeferralItselfFails(t *testing.T) {
	dispatcher := NewDispatcher(fakeRunner{err: db.ErrUnavailable}, nil, clock.Real{}, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))
	cause := errors.New("original as_tenant failure")
	item := store.LockDueMessageRow{ID: uuid.New(), TenantID: uuid.New(), CreatedAt: time.Now()}
	err := dispatcher.deferMessage(context.Background(), item, stepAsTenant, cause)
	if !errors.Is(err, db.ErrUnavailable) {
		t.Fatalf("lost the deferral failure: %v", err)
	}
	if !strings.Contains(err.Error(), cause.Error()) {
		t.Fatalf("lost the original cause: %v", err)
	}
}

// Nit of the second PR #8 review (dispatcher.go:341-354): DeferMessage affecting 0 rows means
// another worker already resolved the message (its own doc comment), so logging "outcome":
// "deferred" would describe something that never happened.
func TestDeferMessageLogsNothingWhenAnotherWorkerAlreadyResolvedIt(t *testing.T) {
	var logs bytes.Buffer
	dispatcher := NewDispatcher(fakeRunner{}, nil, clock.Real{}, slog.New(slog.NewJSONHandler(&logs, nil)))
	item := store.LockDueMessageRow{ID: uuid.New(), TenantID: uuid.New(), CreatedAt: time.Now()}
	if err := dispatcher.deferMessage(context.Background(), item, stepAsTenant, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	if logs.Len() != 0 {
		t.Fatalf("logged a deferral that did not happen: %s", logs.String())
	}
}

// ADR-025 §2 (INV-30): besides '@', a field with "://" or with a run of 20 or more characters of
// [A-Za-z0-9+/=_-] is redacted: URLs, 43-character tokens, JWTs and long identifiers.
func TestDeliveryErrorRedactsURLsAndTokenLikeFields(t *testing.T) {
	const token = "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCde" // 43 base64url characters
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.c2lnbmF0dXJlLXZhbHVlLXRlc3Q"
	cases := []struct {
		name, detail, want string
		mustNotContain     []string
	}{
		{"url with token fragment", "see https://crm.example/reset-password#token=" + token, "see [redacted]",
			[]string{"token=", "://", token}},
		{"fragment without scheme", "x #token=" + token, "x [redacted]", []string{"token="}},
		{"bare token followed by a dot", "x " + token + ".", "x [redacted]", []string{token}},
		{"jwt", "x " + jwt, "x [redacted]", []string{"eyJ"}},
		{"19 characters", "id 0123456789abcdefghi", "id 0123456789abcdefghi", nil},
		{"20 characters", "id 0123456789abcdefghij", "id [redacted]", nil},
		{"plain texts", "Authentication credentials invalid, client host blocked, Mailbox full",
			"Authentication credentials invalid, client host blocked, Mailbox full", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeDetail(tc.detail)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			for _, s := range tc.mustNotContain {
				if strings.Contains(got, s) {
					t.Fatalf("%q survived in %q", s, got)
				}
			}
		})
	}
}

// ADR-025 §1 (INV-30): in the data phase the server already holds the message content, so with an
// SMTP code the provider's text is never kept, whatever Detail says.
func TestDeliveryErrorOmitsProviderTextInDataPhase(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  *DeliveryError
		want string
	}{
		{"quoted url", &DeliveryError{Cause: CauseConfig, Phase: PhaseData, SMTPCode: 554, Enhanced: "5.7.1",
			Detail: "Message rejected, URL https://crm.example/x#token=abc listed"},
			"config data 554 5.7.1: response text omitted"},
		{"recipient", &DeliveryError{Cause: CauseRecipient, Phase: PhaseData, SMTPCode: 550, Enhanced: "5.1.1", Detail: "Recipient rejected"},
			"recipient data 550 5.1.1: response text omitted"},
		{"no SMTP code keeps the fixed vocabulary", &DeliveryError{Cause: CauseNetwork, Phase: PhaseData, Detail: "timeout"},
			"network data: timeout"},
		{"other phases keep the text", &DeliveryError{Cause: CauseConfig, Phase: PhaseMailFrom, SMTPCode: 550, Detail: "Sender not authorized"},
			"config mail_from 550: Sender not authorized"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.LastError(); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
