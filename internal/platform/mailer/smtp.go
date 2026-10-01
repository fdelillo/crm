package mailer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/fdelillo/crm/internal/platform/outbox"
	gomail "github.com/wneessen/go-mail"
)

type Email struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}
type Mailer interface {
	Send(ctx context.Context, e Email) error
}
type SMTPConfig struct {
	Host                     string
	Port                     int
	Username, Password, From string
}
type smtpMailer struct{ cfg SMTPConfig }

func NewSMTP(cfg SMTPConfig) (Mailer, error) {
	if cfg.Host == "" || cfg.Port < 1 || cfg.Port > 65535 {
		return nil, errors.New("mailer: SMTP_HOST and SMTP_PORT are required")
	}
	if err := validateAddress(cfg.From); err != nil {
		return nil, errors.New("mailer: SMTP_FROM is invalid")
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		return nil, errors.New("mailer: SMTP_USERNAME and SMTP_PASSWORD must both be set")
	}
	return &smtpMailer{cfg: cfg}, nil
}

func (s *smtpMailer) Send(ctx context.Context, e Email) error {
	// Validate before dialing. Do not include the invalid address in the error.
	if err := validateAddress(e.To); err != nil {
		return &outbox.PermanentError{Err: errors.New("invalid recipient")}
	}
	if strings.ContainsAny(e.Subject, "\r\n") {
		return &outbox.PermanentError{Err: errors.New("invalid subject")}
	}
	opts := []gomail.Option{gomail.WithPort(s.cfg.Port), gomail.WithTimeout(10 * time.Second)}
	switch {
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
		return &outbox.PermanentError{Err: errors.New("invalid sender")}
	}
	if err := message.To(e.To); err != nil {
		return &outbox.PermanentError{Err: errors.New("invalid recipient")}
	}
	message.Subject(e.Subject)
	message.SetBodyString(gomail.TypeTextPlain, e.TextBody)
	message.AddAlternativeString(gomail.TypeTextHTML, e.HTMLBody)
	if err := client.DialAndSendWithContext(ctx, message); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var sendErr *gomail.SendError
		if errors.As(err, &sendErr) {
			return classifySMTPCode(sendErr.ErrorCode(), err)
		}
		var smtpErr *textproto.Error
		if errors.As(err, &smtpErr) {
			return classifySMTPCode(smtpErr.Code, err)
		}
		return fmt.Errorf("mailer: SMTP unavailable: %w", err)
	}
	return nil
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
func classifySMTPCode(code int, err error) error {
	if code >= 500 && code < 600 {
		return &outbox.PermanentError{Err: err}
	}
	return fmt.Errorf("mailer: SMTP delivery: %w", err)
}
