package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox/store"
	"github.com/google/uuid"
)

// Message is one queued delivery: Enqueue writes it in the caller's transaction, the Dispatcher
// reads it back under the role of its company (ADR-010).
type Message struct {
	TenantID  uuid.UUID
	Kind      string
	Template  string
	Recipient string
	Payload   map[string]string
}

// Enqueuer writes a Message in the caller's transaction (ADR-010): it commits or rolls back with
// whatever operation caused it, so a message is never visible unless its triggering change is.
type Enqueuer interface {
	Enqueue(ctx context.Context, tx db.Tx, m Message) error
}

// enqueuer fixes created_at and next_attempt_at with its own clock (DD-18) instead of the column
// defaults: deferDelay (ADR-024 §6) needs created_at to agree with the clock the rest of the
// outbox uses, in production and in tests alike (M6).
type enqueuer struct{ clock clock.Clock }

// NewEnqueuer returns the Enqueuer. c defaults to clock.Real{} when nil.
func NewEnqueuer(c clock.Clock) Enqueuer {
	if c == nil {
		c = clock.Real{}
	}
	return enqueuer{clock: c}
}

func (e enqueuer) Enqueue(ctx context.Context, tx db.Tx, m Message) error {
	if tx == nil {
		return errors.New("outbox: transaction required")
	}
	tenant, bound := tx.TenantID()
	if !bound || tenant != m.TenantID {
		return errors.New("outbox: transaction tenant mismatch")
	}
	payload, err := json.Marshal(m.Payload)
	if err != nil {
		return fmt.Errorf("outbox: encode payload: %w", err)
	}
	now := e.clock.Now()
	if err := store.New(tx).InsertMessage(ctx, store.InsertMessageParams{
		TenantID: m.TenantID, Kind: m.Kind, Template: m.Template, Recipient: m.Recipient, Payload: payload,
		CreatedAt: now, NextAttemptAt: now,
	}); err != nil {
		return fmt.Errorf("outbox: enqueue: %w", db.MapError(err))
	}
	return nil
}

// Handler delivers one Message; identity/emails implements it (templates + mailer.Mailer).
type Handler interface {
	// Handle returns nil on success, a *DeliveryError (ADR-024) classifying the failure, or
	// context.Canceled if ctx's deadline or cancellation ended the attempt. ctx carries the
	// SendBudget deadline the Dispatcher sets around each call (DD-35): a Handler that blocks
	// past it should return ctx.Err() rather than its own timeout error, so the Dispatcher can
	// tell "the budget ran out" (recoverable, cause network) from "the server rejected it".
	Handle(ctx context.Context, m Message) error
}

// PeriodicTask is a system task the Dispatcher runs every Every() alongside polling (T-B902).
type PeriodicTask interface {
	Name() string
	Every() time.Duration
	Run(ctx context.Context) error
}

// Cause is why a delivery failed (ADR-024 §1). It alone decides whether the Dispatcher may retry
// the message and at what log level, so every DeliveryError carries exactly one.
type Cause string

const (
	// CauseNetwork: a timeout (of a stage or of SendBudget), a rejected, reset or closed
	// connection, or a DNS failure. Always recoverable, logged at WARN.
	CauseNetwork Cause = "network"
	// CauseTransient: any SMTP 4xx response, in any phase. Always recoverable, logged at WARN.
	CauseTransient Cause = "transient"
	// CauseConfig: our configuration or the provider account, affecting every message (any 5xx of
	// the connection phase or of MAIL FROM, a 5xx of RCPT TO/DATA that does not identify the
	// recipient, TLS/AUTH unavailable, or anything the adapter cannot classify). Recoverable, but
	// logged at ERROR: it needs an operator, not a retry (ADR-024 §4).
	CauseConfig Cause = "config"
	// CauseRecipient: the recipient is invalid or does not exist (ADR-024 §1's "recipient rule").
	// Permanent, logged at WARN: it is routine, not an incident.
	CauseRecipient Cause = "recipient"
	// CauseBug: an unknown template, an unreadable or incomplete payload, or an invalid subject —
	// a programming error that a retry cannot fix. Permanent, logged at ERROR.
	CauseBug Cause = "bug"
)

// Permanent reports whether c makes a delivery failure definitive (CauseRecipient, CauseBug):
// every other cause is retried with backoff until MarkFailed's attempt limit (ADR-024 §1).
func (c Cause) Permanent() bool {
	return c == CauseRecipient || c == CauseBug
}

// LogLevel is the level each delivery attempt of cause c is logged at (ADR-024 §1): ERROR for
// CauseConfig and CauseBug, the two that need an operator's attention; WARN for the rest, which
// are either routine (recipient) or expected to clear on retry (network, transient).
func (c Cause) LogLevel() slog.Level {
	switch c {
	case CauseConfig, CauseBug:
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// Phase is the stage of the SMTP conversation a delivery failed in (ADR-024 §2). The connection
// phase is special: any failure in it is attributable to us or the provider, never to the
// message, so the Dispatcher ends the cycle after handling it (ADR-024 §4, §6).
type Phase string

const (
	PhaseCompose    Phase = "compose"    // before connecting: template, payload, addresses, subject
	PhaseConnection Phase = "connection" // TCP, TLS, greeting, EHLO, STARTTLS, AUTH
	PhaseMailFrom   Phase = "mail_from"
	PhaseRcptTo     Phase = "rcpt_to"
	PhaseData       Phase = "data" // DATA, content and the end-of-data response
	PhaseUnknown    Phase = "unknown"
)

// maxLastErrorRunes is the CHECK (char_length(last_error) <= 1000) of migration 00006
// (data-model.md §2.5): LastError truncates to it, counting runes so a multi-byte provider
// message is not cut mid-character.
const maxLastErrorRunes = 1000

// DeliveryError describes one failed delivery attempt (ADR-024): the Mailer and the template
// Handler return it instead of a bare error so the Dispatcher knows the Cause, the Phase and,
// when there was an SMTP response, its code. It is a struct, not a sentinel: errors.Is only says
// *that* something failed, and the Dispatcher needs to know *how* to classify and log it.
type DeliveryError struct {
	Cause    Cause
	Phase    Phase
	SMTPCode int    // the SMTP reply code, or 0 when there was none
	Enhanced string // the RFC 3463 extended code ("d.d.d"), or "" when the server did not report one
	Detail   string // the provider's text or a local description; LastError sanitizes it
	// Err is the original error. It is never logged nor persisted (ADR-024 §3, INV-30): go-mail's
	// own error text includes the recipient address.
	Err error
}

// Error returns LastError(): even a caller that logs the bare error by mistake never leaks an
// address, because this is already the sanitized text.
func (e *DeliveryError) Error() string { return e.LastError() }

// Unwrap exposes Err for tests (errors.Is/As); production code must never log or persist it.
func (e *DeliveryError) Unwrap() error { return e.Err }

// responseTextOmitted is the fixed detail of a data-phase failure that has an SMTP response
// (ADR-025 §1).
const responseTextOmitted = "response text omitted"

// longSecretRun matches 20 or more consecutive characters of the base64, base64url and hex
// alphabets: a securetoken (43), a JWT segment, a key or a long identifier (ADR-025 §2).
var longSecretRun = regexp.MustCompile(`[A-Za-z0-9+/=_-]{20,}`)

// LastError renders "<cause> <phase>[ <SMTP code>[ <extended code>]]: <detail>" (ADR-024 §3),
// with Detail sanitized (every space-separated field containing '@', "://" or a run of 20+
// characters of [A-Za-z0-9+/=_-] replaced by "[redacted]", control characters turned to spaces,
// repeated spaces collapsed, trimmed; ADR-025 §2) and the whole result truncated to
// maxLastErrorRunes runes, ending in "…" if it was cut. In the data phase with an SMTP code the
// detail is the fixed "response text omitted" (ADR-025 §1): that is the only phase in which the
// server already holds the message content, hence the link and its token, so omitting the text
// is a structural guarantee and not a guess about how a provider quotes a URL. It is the only
// place that builds this text: outbox_messages.last_error and every delivery log use it, so
// neither can leak a recipient, a sender, a message id or a token by forgetting to sanitize
// (ADR-024 §3, ADR-025, INV-30).
func (e *DeliveryError) LastError() string {
	head := string(e.Cause) + " " + string(e.Phase)
	if e.SMTPCode != 0 {
		head += " " + strconv.Itoa(e.SMTPCode)
		if e.Enhanced != "" {
			head += " " + e.Enhanced
		}
	}
	detail := responseTextOmitted
	if e.Phase != PhaseData || e.SMTPCode == 0 {
		detail = sanitizeDetail(e.Detail)
	}
	return truncateRunes(head+": "+detail, maxLastErrorRunes)
}

// sanitizeDetail is the redaction (ADR-024 §3 (1), widened by ADR-025 §2) and whitespace
// normalization step of LastError. Redacting one field too many costs nothing; keeping a secret
// in a column that outlives the payload does.
func sanitizeDetail(raw string) string {
	noControl := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, raw)
	fields := strings.Fields(noControl)
	for i, f := range fields {
		if strings.Contains(f, "@") || strings.Contains(f, "://") || longSecretRun.MatchString(f) {
			fields[i] = "[redacted]"
		}
	}
	return strings.Join(fields, " ")
}

// truncateRunes returns s unchanged if it has at most max runes, or its first max-1 runes plus
// "…" (itself one rune) otherwise, so the result never exceeds max runes.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// SendBudget bounds one Handler.Handle call (DD-35, INV-29): the Dispatcher wraps ctx with it
// before calling Handle. SendBudget + 10s must stay <= idle_in_transaction_session_timeout of
// crm_app (30s, db/bootstrap/001_roles_and_database.sql): PostgreSQL must never cut the worker's
// transaction while a send is in flight. A test on each side of that inequality (here and in the
// bootstrap) keeps the two from drifting apart (ADR-024 §5).
const SendBudget = 20 * time.Second
