package mailer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/outbox"
)

func TestSMTPCodeClassification(t *testing.T) {
	for _, tc := range []struct {
		code      int
		permanent bool
	}{{421, false}, {450, false}, {550, true}, {554, true}} {
		err := classifySMTPCode(tc.code, errors.New("SMTP response"))
		var permanent *outbox.PermanentError
		if errors.As(err, &permanent) != tc.permanent {
			t.Errorf("code %d permanent=%v", tc.code, !tc.permanent)
		}
	}
}

func TestRecipientRejectedBeforeDial(t *testing.T) {
	m, err := NewSMTP(SMTPConfig{Host: "127.0.0.1", Port: 1, From: "no-reply@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), Email{To: "user@example.com\r\nBcc: attacker@example.com", Subject: "Hola", TextBody: "texto", HTMLBody: "<p>texto</p>"})
	var permanent *outbox.PermanentError
	if !errors.As(err, &permanent) {
		t.Fatalf("unsafe recipient error=%v", err)
	}
}

func TestUnreachableSMTPIsRecoverable(t *testing.T) {
	m, err := NewSMTP(SMTPConfig{Host: "127.0.0.1", Port: 1, From: "no-reply@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err = m.Send(ctx, Email{To: "user@example.com", Subject: "Hola", TextBody: "texto", HTMLBody: "<p>texto</p>"})
	if err == nil {
		t.Fatal("unreachable SMTP accepted")
	}
	var permanent *outbox.PermanentError
	if errors.As(err, &permanent) {
		t.Fatalf("unreachable classified permanent: %v", err)
	}
}
