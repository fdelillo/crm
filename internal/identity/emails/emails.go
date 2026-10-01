package emails

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"strings"
	texttemplate "text/template"

	"github.com/fdelillo/crm/internal/platform/mailer"
	"github.com/fdelillo/crm/internal/platform/outbox"
)

//go:embed templates/*.txt templates/*.html
var templates embed.FS

// Sentinel render errors, each mapped by renderFailureDetail to a fixed DeliveryError.Detail text
// (ADR-024 §3): render's own fmt.Errorf wrapping (template parse/execute) never reaches a log or
// last_error, only these.
var (
	errUnknownKind        = errors.New("emails: unknown message kind")
	errMissingToken       = errors.New("emails: missing token")
	errUnknownTemplate    = errors.New("emails: unknown email template")
	errMissingCompanyName = errors.New("emails: missing company name")
	errInvalidRole        = errors.New("emails: invalid role")
)

// LinkPaths are the frontend routes each template links to (DD-14: the token travels in the URL
// fragment, never queried by the backend). Empty fields default in NewHandler.
type LinkPaths struct{ Reset, Verify, Invitation string }
type handler struct {
	mailer mailer.Mailer
	base   string
	paths  LinkPaths
}
type templateData struct{ Link, CompanyName, Role string }

// NewHandler returns the outbox.Handler that renders the three email templates (password_reset,
// email_verification, invitation) and sends them with m. base must be an https origin with no
// path, query or fragment (every link is base+path+"#token=..."); paths' empty fields default.
func NewHandler(m mailer.Mailer, base string, paths LinkPaths) (outbox.Handler, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("emails: base URL must be an https origin")
	}
	if m == nil {
		return nil, errors.New("emails: mailer required")
	}
	if paths.Reset == "" {
		paths.Reset = "/reset-password"
	}
	if paths.Verify == "" {
		paths.Verify = "/verify-email"
	}
	if paths.Invitation == "" {
		paths.Invitation = "/accept-invitation"
	}
	for _, path := range []string{paths.Reset, paths.Verify, paths.Invitation} {
		if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#") {
			return nil, errors.New("emails: invalid link path")
		}
	}
	return &handler{mailer: m, base: base, paths: paths}, nil
}

// Handle renders message's template and hands it to the Mailer. A rendering failure is always a
// bug (ADR-024 §1, CauseBug/PhaseCompose): an unknown template, a missing token or an invalid role
// is a defect in the caller, not something a retry fixes. err's own text is never exposed (it may
// come from the payload); the DeliveryError gets a fixed, safe description instead.
func (h *handler) Handle(ctx context.Context, message outbox.Message) error {
	email, err := h.render(message)
	if err != nil {
		return &outbox.DeliveryError{Cause: outbox.CauseBug, Phase: outbox.PhaseCompose, Detail: renderFailureDetail(err)}
	}
	return h.mailer.Send(ctx, email)
}

// renderFailureDetail maps a render error to one of the fixed texts ADR-024 §3 allows for CauseBug
// (never the payload or the template's own parse error, which could contain user-entered text).
func renderFailureDetail(err error) string {
	switch {
	case errors.Is(err, errUnknownKind):
		return "unknown message kind"
	case errors.Is(err, errMissingToken):
		return "missing token"
	case errors.Is(err, errUnknownTemplate):
		return "unknown email template"
	case errors.Is(err, errMissingCompanyName):
		return "missing company name"
	case errors.Is(err, errInvalidRole):
		return "invalid role"
	default:
		return "invalid payload"
	}
}
func (h *handler) render(message outbox.Message) (mailer.Email, error) {
	if message.Kind != "email" {
		return mailer.Email{}, errUnknownKind
	}
	token := message.Payload["token"]
	if token == "" {
		return mailer.Email{}, errMissingToken
	}
	var path, subject string
	switch message.Template {
	case "password_reset":
		path, subject = h.paths.Reset, "Recuperá tu contraseña"
	case "email_verification":
		path, subject = h.paths.Verify, "Verificá tu email"
	case "invitation":
		path, subject = h.paths.Invitation, "Invitación a tu empresa"
	default:
		return mailer.Email{}, errUnknownTemplate
	}
	data := templateData{Link: h.base + path + "#token=" + url.QueryEscape(token),
		CompanyName: message.Payload["company_name"]}
	if message.Template == "invitation" {
		if data.CompanyName == "" {
			return mailer.Email{}, errMissingCompanyName
		}
		switch message.Payload["role"] {
		case "admin":
			data.Role = "Administrador"
		case "operator":
			data.Role = "Operador"
		default:
			return mailer.Email{}, errInvalidRole
		}
	}
	txt, err := texttemplate.New(message.Template+".txt").Option("missingkey=error").ParseFS(templates, "templates/"+message.Template+".txt")
	if err != nil {
		return mailer.Email{}, fmt.Errorf("parse text template: %w", err)
	}
	html, err := htmltemplate.New(message.Template+".html").Option("missingkey=error").ParseFS(templates, "templates/"+message.Template+".html")
	if err != nil {
		return mailer.Email{}, fmt.Errorf("parse HTML template: %w", err)
	}
	var textBody, htmlBody bytes.Buffer
	if err := txt.Execute(&textBody, data); err != nil {
		return mailer.Email{}, fmt.Errorf("render text: %w", err)
	}
	if err := html.Execute(&htmlBody, data); err != nil {
		return mailer.Email{}, fmt.Errorf("render HTML: %w", err)
	}
	return mailer.Email{To: message.Recipient, Subject: subject,
		TextBody: textBody.String(), HTMLBody: htmlBody.String()}, nil
}
