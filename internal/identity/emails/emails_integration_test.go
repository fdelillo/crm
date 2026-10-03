//go:build integration

package emails

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/mailer"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/testsupport/containers"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestTemplatesReachMailpit(t *testing.T) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        containers.Mailpit,
			ExposedPorts: []string{"1025/tcp", "8025/tcp"},
			WaitingFor:   wait.ForListeningPort("8025/tcp"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	smtpPort, err := container.MappedPort(ctx, "1025/tcp")
	if err != nil {
		t.Fatal(err)
	}
	apiPort, err := container.MappedPort(ctx, "8025/tcp")
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(smtpPort.Port(), "%d", &port); err != nil {
		t.Fatal(err)
	}
	smtp, err := mailer.NewSMTP(mailer.SMTPConfig{Host: host, Port: port, From: "no-reply@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	api := "http://" + net.JoinHostPort(host, apiPort.Port())
	for _, tc := range []struct{ template, base, path, subject, token, role string }{
		{"password_reset", "https://crm.example", "/reset-password", "contraseña", "reset123", ""},
		{"email_verification", "https://crm.example", "/verify-email", "email", "verify123", ""},
		{"invitation", "https://crm.example", "/accept-invitation", "invitación", "invite123", "admin"},
		{"invitation", "https://crm.example", "/accept-invitation", "invitación", "invite456", "operator"},
		{"password_reset", "https://localhost:5173", "/reset-password", "contraseña", "local123", ""},
	} {
		t.Run(tc.template+"_"+tc.token, func(t *testing.T) {
			before := mailpitCount(t, api)
			handler, err := NewHandler(smtp, tc.base, LinkPaths{})
			if err != nil {
				t.Fatal(err)
			}
			err = handler.Handle(ctx, outbox.Message{Kind: "email", Template: tc.template,
				Recipient: "user@example.com", Payload: map[string]string{
					"token": tc.token, "company_name": "Empresa Ejemplo", "role": tc.role,
				}})
			if err != nil {
				t.Fatalf("send: %v (cause: %v)", err, errors.Unwrap(err))
			}
			msg := waitMailpitMessage(t, api, before+1)
			if !strings.Contains(strings.ToLower(msg.Subject), tc.subject) {
				t.Errorf("subject=%q", msg.Subject)
			}
			detail := mailpitMessage(t, api, msg.ID)
			link := tc.base + tc.path + "#token=" + tc.token
			if !strings.Contains(detail.Text, link) || !strings.Contains(detail.HTML, link) {
				t.Fatalf("link absent: text=%q html=%q", detail.Text, detail.HTML)
			}
			if tc.template == "invitation" {
				role := "Administrador"
				if tc.role == "operator" {
					role = "Operador"
				}
				if !strings.Contains(detail.Text, "Empresa Ejemplo") || !strings.Contains(detail.Text, role) {
					t.Fatalf("invitation=%q", detail.Text)
				}
			}
			if strings.Contains(detail.Text, "<no value>") || strings.Contains(detail.HTML, "<no value>") {
				t.Fatal("empty template field")
			}
		})
	}
}

type mailpitSummary struct{ ID, Subject string }
type mailpitList struct {
	Total    int              `json:"total"`
	Messages []mailpitSummary `json:"messages"`
}
type mailpitDetail struct{ Text, HTML string }

func mailpitCount(t *testing.T, api string) int {
	t.Helper()
	list := getMailpit[mailpitList](t, api+"/api/v1/messages")
	return list.Total
}
func waitMailpitMessage(t *testing.T, api string, count int) mailpitSummary {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		list := getMailpit[mailpitList](t, api+"/api/v1/messages")
		if list.Total >= count && len(list.Messages) > 0 {
			return list.Messages[0]
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("message did not reach Mailpit")
	return mailpitSummary{}
}
func mailpitMessage(t *testing.T, api, id string) mailpitDetail {
	t.Helper()
	return getMailpit[mailpitDetail](t, api+"/api/v1/message/"+id)
}
func getMailpit[T any](t *testing.T, url string) T {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("Mailpit %s: %d %s", url, response.StatusCode, body)
	}
	var result T
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}
