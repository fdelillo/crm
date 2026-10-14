//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/identity/emails"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/mailer"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/testsupport/containers"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// T-B211, T-B213: an outbox message is delivered through the real SMTP adapter and arrives in Mailpit.
func TestOutboxDeliversEmailToMailpit(t *testing.T) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        containers.Mailpit,
			ExposedPorts: []string{"1025/tcp", "8025/tcp"},
			WaitingFor:   wait.ForListeningPort("8025/tcp"),
		}, Started: true,
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
	smtpPortNumber, err := net.LookupPort("tcp", smtpPort.Port())
	if err != nil {
		t.Fatal(err)
	}
	smtp, err := mailer.NewSMTP(mailer.SMTPConfig{Host: host, Port: smtpPortNumber, From: "no-reply@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	emailHandler, err := emails.NewHandler(smtp, "https://crm.example", emails.LinkPaths{})
	if err != nil {
		t.Fatal(err)
	}

	company := fixture.NewCompany(t, pgtest.AppPool(t))
	runner := db.NewTxRunner(pgtest.AppPool(t))
	// The generic fixture inserts its own pending message; keep this test scoped to the new one.
	err = runner.InTenantTx(ctx, company.ID, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.outbox_messages SET status = 'sent', payload = NULL, sent_at = now()
   WHERE tenant_id = $1`, company.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// The outbox is shared with the other tests of the package (the Dispatcher reads every company):
	// a recipient of its own, and only its message is looked up in Mailpit.
	recipient := "mailpit-" + uuid.NewString() + "@example.com"
	err = runner.InTenantTx(ctx, company.ID, func(ctx context.Context, tx db.Tx) error {
		return outbox.NewEnqueuer(clock.Real{}).Enqueue(ctx, tx, outbox.Message{
			TenantID: company.ID, Kind: "email", Template: "password_reset", Recipient: recipient,
			Payload: map[string]string{"token": "mailpit-token"},
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	// LockDueMessage takes due messages ordered by next_attempt_at, and RunOnce handles a limited batch.
	// Other tests of the package leave due messages behind (hundreds, from the isolation matrices), so
	// this message is made the oldest one: it is the first the Dispatcher picks, whatever those tests
	// left in the shared outbox and whatever order the test files run in.
	if _, err := pgtest.SuperuserPool(t).Exec(ctx, `UPDATE app.outbox_messages SET next_attempt_at = '1970-01-01T00:00:00Z'
   WHERE tenant_id = $1 AND recipient = $2`, company.ID, recipient); err != nil {
		t.Fatal(err)
	}
	dispatcher := outbox.NewDispatcher(runner, emailHandler, clock.Real{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// RunOnce also takes messages of other companies: repeat until ours is sent (with a cap).
	for i := 0; ; i++ {
		if err := dispatcher.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		var sent bool
		if err := runner.InTenantTx(ctx, company.ID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT status = 'sent' FROM app.outbox_messages WHERE tenant_id = $1 AND recipient = $2`, company.ID, recipient).Scan(&sent)
		}); err != nil {
			t.Fatal(err)
		}
		if sent {
			break
		}
		if i >= 50 {
			t.Fatal("the message of the test was not sent after 50 runs")
		}
	}
	err = runner.InTenantTx(ctx, company.ID, func(ctx context.Context, tx db.Tx) error {
		var status string
		var payloadNull bool
		if err := tx.QueryRow(ctx, `SELECT status, payload IS NULL FROM app.outbox_messages
   WHERE tenant_id = $1 AND recipient = $2 ORDER BY created_at DESC LIMIT 1`, company.ID, recipient).
			Scan(&status, &payloadNull); err != nil {
			return err
		}
		if status != "sent" || !payloadNull {
			t.Errorf("outbox status=%s scrubbed=%v", status, payloadNull)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	api := "http://" + net.JoinHostPort(host, apiPort.Port())
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(api + "/api/v1/search?query=" + url.QueryEscape("to:"+recipient))
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Total int `json:"messages_count"`
		}
		err = json.NewDecoder(response.Body).Decode(&result)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if result.Total == 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("transactional outbox email did not reach Mailpit")
}
