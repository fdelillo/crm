package mailer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fdelillo/crm/internal/platform/outbox"
	gomail "github.com/wneessen/go-mail"
)

// Email is the rendered message identity/emails hands to the Mailer: a subject and two bodies, in
// Spanish, with no attachments (001 only sends transactional links).
type Email struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// Mailer sends one Email. The SMTP adapter is the only implementation; identity/emails depends on
// this interface, not on go-mail, so a future provider needs no change above this package.
type Mailer interface {
	// Send returns nil, a *outbox.DeliveryError classified per ADR-024 §1-§2 and ADR-025 §3, or
	// ctx.Err() verbatim (context.Canceled or context.DeadlineExceeded) if ctx ended before the
	// attempt finished. ctx carries the SendBudget deadline the Dispatcher sets; what the context
	// error means is decided one layer up, in outbox.classifyHandleResult (DD-35).
	Send(ctx context.Context, e Email) error
}

// SMTPConfig is everything the adapter needs to reach the provider. Username and Password are
// either both set or both empty (NewSMTP rejects the mixed case).
type SMTPConfig struct {
	Host                     string
	Port                     int
	Username, Password, From string
}

// stageTimeout bounds every stage of the SMTP conversation (DD-35, INV-29): with WithoutRset there
// are 4 such windows (dial; greeting; EHLO+STARTTLS+AUTH+NOOP; MAIL+RCPT+DATA+end of
// data+QUIT), so the worst case is 4*stageTimeout, which must stay within outbox.SendBudget
// (ADR-024 §5). A unit test ties the two together so raising either one is a deliberate change.
const stageTimeout = 5 * time.Second

type smtpMailer struct {
	cfg SMTPConfig
	// forceNoTLS is only set by newSMTPForTest (T-B213): it lets a test exercise AUTH against an
	// in-process fake server, which cannot do TLS. Production always goes through NewSMTP's own
	// rule (DD-14-adjacent: only skip TLS for a local, unauthenticated host).
	forceNoTLS bool
}

// NewSMTP validates cfg and returns the adapter. It never dials: Send does, on every call, because
// go-mail's Client is built around one connection per DialAndSendWithContext (ADR-010).
func NewSMTP(cfg SMTPConfig) (Mailer, error) { return newSMTP(cfg, false) }

// newSMTPForTest is NewSMTP without the "no TLS only when there is no username" rule, so a unit
// test can authenticate against the fake SMTP server of smtp_fake_test.go without TLS (T-B213).
func newSMTPForTest(cfg SMTPConfig) (Mailer, error) { return newSMTP(cfg, true) }

func newSMTP(cfg SMTPConfig, forceNoTLS bool) (Mailer, error) {
	if cfg.Host == "" || cfg.Port < 1 || cfg.Port > 65535 {
		return nil, errors.New("mailer: SMTP_HOST and SMTP_PORT are required")
	}
	if err := validateAddress(cfg.From); err != nil {
		return nil, errors.New("mailer: SMTP_FROM is invalid")
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		return nil, errors.New("mailer: SMTP_USERNAME and SMTP_PASSWORD must both be set")
	}
	return &smtpMailer{cfg: cfg, forceNoTLS: forceNoTLS}, nil
}

func (s *smtpMailer) Send(ctx context.Context, e Email) error {
	// Validated before dialing, so an invalid address never counts a connection attempt (ADR-024
	// §2, phase compose). Never include the address itself: these are DeliveryError.Detail, which
	// LastError logs and persists.
	if err := validateAddress(e.To); err != nil {
		return &outbox.DeliveryError{Cause: outbox.CauseRecipient, Phase: outbox.PhaseCompose, Detail: "invalid recipient address"}
	}
	if strings.ContainsAny(e.Subject, "\r\n") {
		return &outbox.DeliveryError{Cause: outbox.CauseBug, Phase: outbox.PhaseCompose, Detail: "invalid subject"}
	}

	opts := []gomail.Option{gomail.WithPort(s.cfg.Port), gomail.WithTimeout(stageTimeout), gomail.WithoutRset()}
	switch {
	case s.forceNoTLS:
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	case s.cfg.Port == 465:
		opts = append(opts, gomail.WithSSL())
	case s.cfg.Username == "" && localSMTPHost(s.cfg.Host):
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	default:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	}
	if s.cfg.Username != "" {
		opts = append(opts, gomail.WithUsername(s.cfg.Username), gomail.WithPassword(s.cfg.Password),
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover))
	}
	client, err := gomail.NewClient(s.cfg.Host, opts...)
	if err != nil {
		return fmt.Errorf("mailer: configure SMTP: %w", err)
	}
	message := gomail.NewMsg()
	if err := message.From(s.cfg.From); err != nil {
		return &outbox.DeliveryError{Cause: outbox.CauseConfig, Phase: outbox.PhaseCompose, Detail: "invalid sender"}
	}
	if err := message.To(e.To); err != nil {
		return &outbox.DeliveryError{Cause: outbox.CauseRecipient, Phase: outbox.PhaseCompose, Detail: "invalid recipient address"}
	}
	message.Subject(e.Subject)
	message.SetBodyString(gomail.TypeTextPlain, e.TextBody)
	message.AddAlternativeString(gomail.TypeTextHTML, e.HTMLBody)

	sendErr := client.DialAndSendWithContext(ctx, message)
	if sendErr == nil {
		return nil
	}
	// The server accepted the end of DATA before the failure (RSET/QUIT, or a cut connection): the
	// message was delivered, so a retry would duplicate it (ADR-024 §5, accepted by ADR-010).
	if message.IsDelivered() {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return classifyDeliveryError(sendErr)
}

// classifyDeliveryError turns whatever DialAndSendWithContext returned into a *outbox.DeliveryError
// (ADR-024 §1-§3). go-mail wraps every error of the dial (connection, TLS, greeting, EHLO,
// STARTTLS, AUTH) as "dial failed: %w" and never as *gomail.SendError (research R-29); everything
// from MAIL FROM onward is a *gomail.SendError.
func classifyDeliveryError(err error) *outbox.DeliveryError {
	var sendErr *gomail.SendError
	if errors.As(err, &sendErr) {
		return classifySendError(sendErr)
	}
	return classifyConnectionError(err)
}

// classifySendError applies ADR-024 §2's special cases by Reason, then the generic code-based rule
// of §1 for the three SMTP-command reasons.
func classifySendError(err *gomail.SendError) *outbox.DeliveryError {
	detail := sendErrorDetail(err)
	switch err.Reason {
	case gomail.ErrConnCheck:
		return &outbox.DeliveryError{Cause: outbox.CauseNetwork, Phase: outbox.PhaseConnection, Detail: networkDetailFromText(detail), Err: err}
	case gomail.ErrGetRcpts:
		return &outbox.DeliveryError{Cause: outbox.CauseRecipient, Phase: outbox.PhaseCompose, Detail: detail, Err: err}
	case gomail.ErrGetSender:
		return &outbox.DeliveryError{Cause: outbox.CauseConfig, Phase: outbox.PhaseCompose, Detail: detail, Err: err}
	case gomail.ErrNoUnencoded, gomail.ErrAmbiguous:
		return &outbox.DeliveryError{Cause: outbox.CauseConfig, Phase: outbox.PhaseUnknown, Detail: detail, Err: err}
	}
	phase := phaseForSendErrReason(err.Reason)
	code, enhanced := err.ErrorCode(), err.EnhancedStatusCode()
	if enhanced == "" && code != 0 {
		enhanced, detail = splitEnhancedCode(code, detail)
	}
	cause := classifyCodeInPhase(phase, code, enhanced)
	if cause == outbox.CauseNetwork {
		// SendError has no Unwrap (research R-29), so a write/close failure during MAIL, RCPT or
		// DATA (code == 0: go-mail never got a numbered response) only leaves this package the
		// error's own text to work with. The fixed vocabulary of ADR-024 §3 still applies: a
		// *reading* of the text, not the text itself, which could otherwise be an arbitrary
		// net.OpError message.
		detail = networkDetailFromText(detail)
	}
	return &outbox.DeliveryError{Cause: cause, Phase: phase, SMTPCode: code, Enhanced: enhanced, Detail: detail, Err: err}
}

// networkDetailFromText maps go-mail's own error text to ADR-024 §3's fixed vocabulary for network
// causes, for the paths where the structured error is hidden behind *gomail.SendError.
func networkDetailFromText(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "refused"):
		return "connection refused"
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "deadline exceeded"):
		return "timeout"
	case strings.Contains(lower, "reset"), strings.Contains(lower, "closed"), strings.Contains(lower, "eof"):
		return "connection closed"
	case strings.Contains(lower, "lookup"), strings.Contains(lower, "no such host"):
		return "dns lookup failed"
	default:
		return "network error"
	}
}

func phaseForSendErrReason(r gomail.SendErrReason) outbox.Phase {
	switch r {
	case gomail.ErrSMTPMailFrom:
		return outbox.PhaseMailFrom
	case gomail.ErrSMTPRcptTo:
		return outbox.PhaseRcptTo
	case gomail.ErrSMTPData, gomail.ErrWriteContent, gomail.ErrSMTPDataClose:
		return outbox.PhaseData
	default:
		return outbox.PhaseUnknown
	}
}

// classifyCodeInPhase is ADR-024 §1's rule once the special cases of §2 are out of the way: 4xx is
// always transient; 5xx is recipient only under the destination rule (mail_from never is: a 5xx
// there is about the sender); anything else, including no code at all, is config/network.
func classifyCodeInPhase(phase outbox.Phase, code int, enhanced string) outbox.Cause {
	switch {
	case code == 0:
		return outbox.CauseNetwork
	case code >= 400 && code < 500:
		return outbox.CauseTransient
	case code >= 500 && code < 600:
		if isRecipientRejection(phase, code, enhanced) {
			return outbox.CauseRecipient
		}
		return outbox.CauseConfig
	default:
		return outbox.CauseConfig
	}
}

// isRecipientRejection is ADR-024 §1's "regla del destinatario": the only way a response is
// definitive. It never applies to mail_from or the connection phase.
func isRecipientRejection(phase outbox.Phase, code int, enhanced string) bool {
	if phase != outbox.PhaseRcptTo && phase != outbox.PhaseData {
		return false
	}
	if enhanced != "" {
		return strings.HasPrefix(enhanced, "5.1.") || strings.HasPrefix(enhanced, "5.2.")
	}
	return phase == outbox.PhaseRcptTo && (code == 550 || code == 551 || code == 553)
}

// sendErrorDetail renders a *gomail.SendError's own text, without the reason prefix, the leading
// SMTP/extended code (already in the DeliveryError's own fields) or go-mail's recipient/message-id
// suffixes, which would otherwise repeat an address LastError must redact (ADR-024 §3).
func sendErrorDetail(err *gomail.SendError) string {
	text := err.Error()
	if _, rest, ok := strings.Cut(text, ": "); ok {
		text = rest
	}
	if code := err.ErrorCode(); code != 0 {
		if rest, ok := strings.CutPrefix(text, strconv.Itoa(code)); ok {
			text = strings.TrimPrefix(rest, " ")
		}
	}
	for _, suffix := range []string{", affected recipient(s):", ", affected message ID:"} {
		if idx := strings.Index(text, suffix); idx != -1 {
			text = text[:idx]
		}
	}
	text = strings.TrimSpace(text)
	// net/textproto's *Error.Error() quotes its Msg (fmt.Sprintf("%03d %q", Code, Msg)); SendError
	// has no Unwrap (research R-29), so unquoting its own Error() text is the only way back to the
	// server's plain response text.
	if unquoted, err := strconv.Unquote(text); err == nil {
		text = unquoted
	}
	if enhanced := err.EnhancedStatusCode(); enhanced != "" {
		if rest, ok := strings.CutPrefix(text, enhanced); ok {
			text = strings.TrimPrefix(rest, " ")
		}
	}
	return strings.TrimSpace(text)
}

// enhancedCodeRe matches an RFC 3463 extended code at the start of a response's text: a class of
// 2, 4 or 5 and two subject/detail numbers of up to 3 digits, followed by a space or the end.
var enhancedCodeRe = regexp.MustCompile(`^([245])\.\d{1,3}\.\d{1,3}(?: |$)`)

// splitEnhancedCode is the only place that reads the extended code out of a response's text (ADR-025
// §3). The server may not announce ENHANCEDSTATUSCODES (go-mail then reports none) and the dial
// errors never carry it, so the phase does not matter: both the connection phase and
// classifySendError call it. The code is used only when its class (first digit) matches the basic
// code's, as RFC 3463 requires; otherwise it is left in the text and the basic code decides.
// text is the response text without its basic code.
func splitEnhancedCode(code int, text string) (enhanced, rest string) {
	m := enhancedCodeRe.FindStringSubmatch(text)
	if code < 100 || m == nil || m[1] != strconv.Itoa(code)[:1] {
		return "", text
	}
	enhanced = strings.TrimSuffix(m[0], " ")
	return enhanced, strings.TrimLeft(text[len(m[0]):], " ")
}

// classifyConnectionError handles everything that is not a *gomail.SendError: go-mail wraps the
// whole dial (TCP, TLS, greeting, EHLO, STARTTLS, AUTH) as "dial failed: %w" (research R-29),
// never as a SendError, so every case here is "connection" (ADR-024 §2).
func classifyConnectionError(err error) *outbox.DeliveryError {
	var textErr *textproto.Error
	if errors.As(err, &textErr) {
		enhanced, detail := splitEnhancedCode(textErr.Code, textErr.Msg)
		return &outbox.DeliveryError{Cause: classifyCodeInPhase(outbox.PhaseConnection, textErr.Code, enhanced),
			Phase: outbox.PhaseConnection, SMTPCode: textErr.Code, Enhanced: enhanced, Detail: detail, Err: err}
	}
	if isTimeoutError(err) {
		return &outbox.DeliveryError{Cause: outbox.CauseNetwork, Phase: outbox.PhaseConnection, Detail: "timeout", Err: err}
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return &outbox.DeliveryError{Cause: outbox.CauseNetwork, Phase: outbox.PhaseConnection, Detail: "dns lookup failed", Err: err}
	}
	if detail, ok := classifyConnectionReset(err); ok {
		return &outbox.DeliveryError{Cause: outbox.CauseNetwork, Phase: outbox.PhaseConnection, Detail: detail, Err: err}
	}
	if isTLSError(err) {
		return &outbox.DeliveryError{Cause: outbox.CauseConfig, Phase: outbox.PhaseConnection, Detail: "tls handshake failed", Err: err}
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return &outbox.DeliveryError{Cause: outbox.CauseNetwork, Phase: outbox.PhaseConnection, Detail: "network error", Err: err}
	}
	// Anything else is go-mail's own fixed text (no STARTTLS offered, no compatible AUTH
	// mechanism, and similar): never data from the message, safe to use as-is (ADR-024 §2).
	return &outbox.DeliveryError{Cause: outbox.CauseConfig, Phase: outbox.PhaseConnection, Detail: err.Error(), Err: err}
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// classifyConnectionReset recognises a rejected, reset or closed connection (ECONNREFUSED,
// ECONNRESET, EOF, a read on a closed connection): all three read as "the other side is not there
// anymore", with a detail that says which.
func classifyConnectionReset(err error) (detail string, ok bool) {
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused", true
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, net.ErrClosed):
		return "connection closed", true
	}
	return "", false
}

// isTLSError recognises a certificate or handshake failure: ours or the provider's TLS
// configuration, never the message, so it is a config cause (ADR-024 §2).
func isTLSError(err error) bool {
	var certErr *tls.CertificateVerificationError
	var hostErr x509.HostnameError
	var authErr x509.UnknownAuthorityError
	var invalidErr x509.CertificateInvalidError
	var alertErr tls.AlertError
	var recordErr tls.RecordHeaderError
	return errors.As(err, &certErr) || errors.As(err, &hostErr) || errors.As(err, &authErr) ||
		errors.As(err, &invalidErr) || errors.As(err, &alertErr) || errors.As(err, &recordErr)
}

func localSMTPHost(host string) bool {
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}

func validateAddress(raw string) error {
	if strings.ContainsAny(raw, "\r\n") {
		return errors.New("line break in address")
	}
	parsed, err := mail.ParseAddress(raw)
	if err != nil || parsed.Address != raw {
		return errors.New("invalid email address")
	}
	return nil
}
