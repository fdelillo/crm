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

type LinkPaths struct{ Reset, Verify, Invitation string }
type handler struct {
	mailer mailer.Mailer
	base   string
	paths  LinkPaths
}
type templateData struct{ Link, CompanyName, Role string }

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
func (h *handler) Handle(ctx context.Context, message outbox.Message) error {
	email, err := h.render(message)
	if err != nil {
		return &outbox.PermanentError{Err: err}
	}
	return h.mailer.Send(ctx, email)
}
func (h *handler) render(message outbox.Message) (mailer.Email, error) {
	if message.Kind != "email" {
		return mailer.Email{}, errors.New("unknown message kind")
	}
	token := message.Payload["token"]
	if token == "" {
		return mailer.Email{}, errors.New("missing token")
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
		return mailer.Email{}, errors.New("unknown email template")
	}
	data := templateData{Link: h.base + path + "#token=" + url.QueryEscape(token),
		CompanyName: message.Payload["company_name"]}
	if message.Template == "invitation" {
		if data.CompanyName == "" {
			return mailer.Email{}, errors.New("missing company name")
		}
		switch message.Payload["role"] {
		case "admin":
			data.Role = "Administrador"
		case "operator":
			data.Role = "Operador"
		default:
			return mailer.Email{}, errors.New("invalid role")
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
