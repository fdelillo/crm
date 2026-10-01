package mailer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/textproto"
	"syscall"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/outbox"
)

func TestStageTimeoutsFitInsideSendBudget(t *testing.T) {
	// Four deadline windows without RSET (ADR-024 §5, DD-35): dial; greeting; EHLO+STARTTLS+AUTH+
	// NOOP; MAIL+RCPT+DATA+end of data+QUIT. Raising either value without checking this is exactly
	// the mistake the review found (M7): the worst case must stay within SendBudget.
	if want, got := outbox.SendBudget, 4*stageTimeout; got > want {
		t.Fatalf("4 * stageTimeout = %s exceeds SendBudget = %s", got, want)
	}
}

func TestRecipientRejectedBeforeDial(t *testing.T) {
	m, err := NewSMTP(SMTPConfig{Host: "127.0.0.1", Port: 1, From: "no-reply@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), Email{To: "user@example.com\r\nBcc: attacker@example.com", Subject: "Hola", TextBody: "texto", HTMLBody: "<p>texto</p>"})
	var de *outbox.DeliveryError
	if !errors.As(err, &de) || de.Cause != outbox.CauseRecipient || de.Phase != outbox.PhaseCompose {
		t.Fatalf("unsafe recipient error=%v", err)
	}
}

func TestInvalidSubjectIsABugBeforeDial(t *testing.T) {
	m, err := NewSMTP(SMTPConfig{Host: "127.0.0.1", Port: 1, From: "no-reply@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), Email{To: "user@example.com", Subject: "Hola\r\nX-Injected: 1", TextBody: "t", HTMLBody: "<p>t</p>"})
	var de *outbox.DeliveryError
	if !errors.As(err, &de) || de.Cause != outbox.CauseBug || de.Phase != outbox.PhaseCompose {
		t.Fatalf("expected a bug error, got %v", err)
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
	var de *outbox.DeliveryError
	if !errors.As(err, &de) || de.Cause.Permanent() {
		t.Fatalf("unreachable classified as permanent: %v", err)
	}
	if de.Phase != outbox.PhaseConnection {
		t.Fatalf("phase=%v want connection", de.Phase)
	}
}

func TestRecipientRejectionRule(t *testing.T) {
	for _, tc := range []struct {
		phase    outbox.Phase
		code     int
		enhanced string
		want     outbox.Cause
	}{
		{outbox.PhaseRcptTo, 550, "", outbox.CauseRecipient},
		{outbox.PhaseRcptTo, 551, "", outbox.CauseRecipient},
		{outbox.PhaseRcptTo, 553, "", outbox.CauseRecipient},
		{outbox.PhaseRcptTo, 554, "", outbox.CauseConfig},
		{outbox.PhaseRcptTo, 550, "5.1.1", outbox.CauseRecipient},
		{outbox.PhaseRcptTo, 550, "5.2.2", outbox.CauseRecipient},
		{outbox.PhaseRcptTo, 550, "5.7.1", outbox.CauseConfig},
		{outbox.PhaseData, 550, "5.1.1", outbox.CauseRecipient},
		{outbox.PhaseData, 550, "", outbox.CauseConfig},
		{outbox.PhaseMailFrom, 550, "5.1.1", outbox.CauseConfig},
		{outbox.PhaseConnection, 550, "5.1.1", outbox.CauseConfig},
	} {
		t.Run(string(tc.phase)+"_"+tc.enhanced, func(t *testing.T) {
			if got := classifyCodeInPhase(tc.phase, tc.code, tc.enhanced); got != tc.want {
				t.Errorf("phase=%s code=%d enhanced=%q got=%s want=%s", tc.phase, tc.code, tc.enhanced, got, tc.want)
			}
		})
	}
}

func TestClassifyCodeInPhaseGenericRules(t *testing.T) {
	if got := classifyCodeInPhase(outbox.PhaseMailFrom, 0, ""); got != outbox.CauseNetwork {
		t.Errorf("no code = %s want network", got)
	}
	if got := classifyCodeInPhase(outbox.PhaseRcptTo, 452, "4.2.2"); got != outbox.CauseTransient {
		t.Errorf("4xx = %s want transient", got)
	}
	if got := classifyCodeInPhase(outbox.PhaseConnection, 250, ""); got != outbox.CauseConfig {
		t.Errorf("unexpected 2xx = %s want config", got)
	}
}

type fakeTimeoutError struct{ error }

func (fakeTimeoutError) Timeout() bool   { return true }
func (fakeTimeoutError) Temporary() bool { return true }

func TestClassifyConnectionErrorNetworkCauses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		detail string
	}{
		{"deadline exceeded", context.DeadlineExceeded, "timeout"},
		{"net timeout", fakeTimeoutError{errors.New("i/o timeout")}, "timeout"},
		{"dns error", &net.DNSError{Err: "no such host", Name: "example.invalid"}, "dns lookup failed"},
		{"connection refused", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, "connection refused"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			de := classifyConnectionError(tc.err)
			if de.Cause != outbox.CauseNetwork || de.Phase != outbox.PhaseConnection || de.Detail != tc.detail {
				t.Fatalf("got cause=%s phase=%s detail=%q", de.Cause, de.Phase, de.Detail)
			}
		})
	}
}

func TestClassifyConnectionErrorTLSIsConfig(t *testing.T) {
	err := &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}
	de := classifyConnectionError(err)
	if de.Cause != outbox.CauseConfig || de.Phase != outbox.PhaseConnection || de.Detail != "tls handshake failed" {
		t.Fatalf("got cause=%s phase=%s detail=%q", de.Cause, de.Phase, de.Detail)
	}
}

func TestClassifyConnectionErrorSMTPResponse(t *testing.T) {
	err := &textproto.Error{Code: 554, Msg: "5.7.1 client host blocked"}
	de := classifyConnectionError(err)
	if de.Cause != outbox.CauseConfig || de.SMTPCode != 554 || de.Enhanced != "5.7.1" || de.Detail != "client host blocked" {
		t.Fatalf("got %+v", de)
	}
}

func TestClassifyConnectionErrorUnclassifiedUsesGoMailText(t *testing.T) {
	err := errors.New("smtp: server does not support SMTP AUTH")
	de := classifyConnectionError(err)
	if de.Cause != outbox.CauseConfig || de.Phase != outbox.PhaseConnection || de.Detail != err.Error() {
		t.Fatalf("got %+v", de)
	}
}
