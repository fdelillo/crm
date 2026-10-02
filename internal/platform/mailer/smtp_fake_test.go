//go:build integration

package mailer

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/outbox"
)

// fakeSMTPScript is the scripted behaviour of one fake SMTP conversation (T-B213): each field is
// the exact response line for that stage, or a "hang" flag that makes the server stop responding
// (and keep the connection open) to exercise go-mail's per-stage timeout.
type fakeSMTPScript struct {
	greeting       string
	greetingHang   bool
	heloCode       string   // overrides the EHLO/HELO response entirely (e.g. a 550)
	extensions     []string // EHLO extension lines, without the leading "250-"/"250 " (e.g. "AUTH CRAM-MD5")
	authResponse   string
	mailFrom       string
	rcptTo         string
	dataPrompt     string
	dataPromptHang bool
	endOfData      string
	endOfDataHang  bool
	closeAfterData bool // close right after endOfData instead of answering QUIT
}

// fakeSMTPServer accepts exactly the connections the test dials, each played against script. The
// commands of every connection are recorded, in order, for assertions on what the client sent.
type fakeSMTPServer struct {
	listener net.Listener
	script   fakeSMTPScript

	mu       sync.Mutex
	commands []string
}

func newFakeSMTPServer(t *testing.T, script fakeSMTPScript) *fakeSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSMTPServer{listener: listener, script: script}
	go s.acceptLoop()
	t.Cleanup(func() { _ = listener.Close() })
	return s
}

func (s *fakeSMTPServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.serve(conn)
	}
}

func (s *fakeSMTPServer) record(line string) {
	s.mu.Lock()
	s.commands = append(s.commands, line)
	s.mu.Unlock()
}

// Commands returns the SMTP commands received so far, in order.
func (s *fakeSMTPServer) Commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}

// Host and Port split the listener's address the way mailer.SMTPConfig needs it.
func (s *fakeSMTPServer) Host() string {
	host, _, _ := net.SplitHostPort(s.listener.Addr().String())
	return host
}
func (s *fakeSMTPServer) Port() int {
	_, port, _ := net.SplitHostPort(s.listener.Addr().String())
	n, _ := strconv.Atoi(port)
	return n
}

func isSMTPErrorCode(line string) bool {
	return len(line) > 0 && (line[0] == '4' || line[0] == '5')
}

// serve plays script against one connection. A stage left to "hang" never writes its response: the
// client is left reading, and the stage timeout of go-mail (or the test's own context) is what
// eventually ends the attempt. A 10s read deadline bounds the goroutine's own lifetime regardless,
// so a client that never closes its end does not leak the goroutine past the test.
func (s *fakeSMTPServer) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	w := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	r := bufio.NewReader(conn)
	readLine := func() (string, bool) {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", false
		}
		return strings.TrimRight(line, "\r\n"), true
	}

	if s.script.greetingHang {
		readLine() // blocks until the client gives up and closes, or the deadline above fires
		return
	}
	w(s.script.greeting)
	if isSMTPErrorCode(s.script.greeting) {
		readLine() // the client has nothing to say after a rejected greeting besides closing
		return
	}

	for {
		line, ok := readLine()
		if !ok {
			return
		}
		s.record(line)
		verb := strings.ToUpper(strings.Fields(line)[0])
		switch verb {
		case "EHLO", "HELO":
			if s.script.heloCode != "" {
				w(s.script.heloCode)
				continue
			}
			// go-mail's EHLO parser treats the response's first line as greeting text (discarded)
			// and only the following lines as capabilities (smtp_ehlo.go ehlo()): a script with a
			// single extension and no banner line would otherwise register no capability at all.
			lines := append([]string{"fake.example greets you"}, s.script.extensions...)
			for i, ext := range lines {
				sep := "-"
				if i == len(lines)-1 {
					sep = " "
				}
				w("250" + sep + ext)
			}
		case "AUTH":
			w("334 " + base64.StdEncoding.EncodeToString([]byte("<fake.challenge>")))
			if _, ok := readLine(); !ok {
				return
			}
			w(s.script.authResponse)
		case "MAIL":
			w(s.script.mailFrom)
		case "RCPT":
			w(s.script.rcptTo)
		case "DATA":
			if s.script.dataPromptHang {
				readLine()
				return
			}
			w(s.script.dataPrompt)
			if isSMTPErrorCode(s.script.dataPrompt) {
				continue
			}
			for {
				l, ok := readLine()
				if !ok || l == "." {
					break
				}
			}
			if s.script.endOfDataHang {
				readLine()
				return
			}
			w(s.script.endOfData)
			if s.script.closeAfterData {
				return
			}
		case "QUIT":
			w("221 2.0.0 bye")
			return
		case "NOOP":
			w("250 2.0.0 ok")
		case "RSET":
			w("250 2.0.0 ok")
		default:
			w("500 5.5.1 unrecognized command")
		}
	}
}

// send builds the adapter against server, without TLS (the fake server speaks plain text) and
// without a username: most scripts are about MAIL/RCPT/DATA, not AUTH.
func send(t *testing.T, server *fakeSMTPServer, ctx context.Context) error {
	t.Helper()
	m, err := newSMTPForTest(SMTPConfig{Host: server.Host(), Port: server.Port(), From: "no-reply@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return m.Send(ctx, Email{To: "user@example.com", Subject: "Hola", TextBody: "texto", HTMLBody: "<p>texto</p>"})
}

// sendAuthenticated is send but with a username, so go-mail auto-discovers a mechanism from the
// server's EHLO extensions and attempts AUTH (T-B213: "el test arma el adaptador con usuario y sin
// TLS con un constructor no exportado para tests"; production's TLS policy does not change).
func sendAuthenticated(t *testing.T, server *fakeSMTPServer, ctx context.Context) error {
	t.Helper()
	m, err := newSMTPForTest(SMTPConfig{Host: server.Host(), Port: server.Port(), From: "no-reply@example.com",
		Username: "crm", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return m.Send(ctx, Email{To: "user@example.com", Subject: "Hola", TextBody: "texto", HTMLBody: "<p>texto</p>"})
}

func assertDeliveryError(t *testing.T, err error, wantLastError string) *outbox.DeliveryError {
	t.Helper()
	var de *outbox.DeliveryError
	if !errors.As(err, &de) {
		t.Fatalf("not a *outbox.DeliveryError: %v", err)
	}
	if got := de.LastError(); got != wantLastError {
		t.Fatalf("LastError()=%q want %q", got, wantLastError)
	}
	return de
}

func TestFakeSMTPGreetingRejectsAsConfig(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greeting: "554 5.7.1 client host blocked"})
	err := send(t, server, context.Background())
	de := assertDeliveryError(t, err, "config connection 554 5.7.1: client host blocked")
	if de.Cause.Permanent() {
		t.Fatal("connection-phase failure must not be permanent")
	}
}

func TestFakeSMTPGreetingTransient(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greeting: "421 4.3.2 Service not available"})
	err := send(t, server, context.Background())
	assertDeliveryError(t, err, "transient connection 421 4.3.2: Service not available")
}

func TestFakeSMTPEhloAndHeloRejected(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greeting: "220 fake.example", heloCode: "550 not allowed"})
	err := send(t, server, context.Background())
	var de *outbox.DeliveryError
	if !errors.As(err, &de) || de.Cause != outbox.CauseConfig || de.Phase != outbox.PhaseConnection || de.SMTPCode != 550 {
		t.Fatalf("got %+v err=%v", de, err)
	}
}

func TestFakeSMTPAuthCredentialsInvalid(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greeting: "220 fake.example", extensions: []string{"AUTH CRAM-MD5"},
		authResponse: "535 5.7.8 Authentication credentials invalid"})
	err := sendAuthenticated(t, server, context.Background())
	assertDeliveryError(t, err, "config connection 535 5.7.8: Authentication credentials invalid")
}

func TestFakeSMTPAuthTemporaryFailure(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greeting: "220 fake.example", extensions: []string{"AUTH CRAM-MD5"},
		authResponse: "454 4.7.0 Temporary authentication failure"})
	err := sendAuthenticated(t, server, context.Background())
	assertDeliveryError(t, err, "transient connection 454 4.7.0: Temporary authentication failure")
}

func TestFakeSMTPNoCompatibleAuthMechanism(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greeting: "220 fake.example"}) // no AUTH line
	err := sendAuthenticated(t, server, context.Background())
	var de *outbox.DeliveryError
	if !errors.As(err, &de) || de.Cause != outbox.CauseConfig || de.Phase != outbox.PhaseConnection || de.SMTPCode != 0 {
		t.Fatalf("got %+v err=%v", de, err)
	}
	if strings.Contains(de.LastError(), "@") {
		t.Fatalf("leaked an address: %q", de.LastError())
	}
}

func readyScript(overrides fakeSMTPScript) fakeSMTPScript {
	base := fakeSMTPScript{greeting: "220 fake.example", extensions: []string{"ENHANCEDSTATUSCODES"},
		mailFrom: "250 2.1.0 Sender ok", rcptTo: "250 2.1.5 Recipient ok", dataPrompt: "354 Start input",
		endOfData: "250 2.0.0 Message accepted"}
	if overrides.mailFrom != "" {
		base.mailFrom = overrides.mailFrom
	}
	if overrides.rcptTo != "" {
		base.rcptTo = overrides.rcptTo
	}
	if overrides.dataPrompt != "" {
		base.dataPrompt = overrides.dataPrompt
	}
	if overrides.endOfData != "" {
		base.endOfData = overrides.endOfData
	}
	base.dataPromptHang = overrides.dataPromptHang
	base.endOfDataHang = overrides.endOfDataHang
	base.closeAfterData = overrides.closeAfterData
	return base
}

func TestFakeSMTPMailFromRejected(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{mailFrom: "550 5.7.1 Sender not authorized"}))
	err := send(t, server, context.Background())
	assertDeliveryError(t, err, "config mail_from 550 5.7.1: Sender not authorized")
}

func TestFakeSMTPMailFromTransient(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{mailFrom: "451 4.3.0 Try again later"}))
	err := send(t, server, context.Background())
	assertDeliveryError(t, err, "transient mail_from 451 4.3.0: Try again later")
}

func TestFakeSMTPRcptToRecipientRejectedWithEnhanced(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{rcptTo: "550 5.1.1 user@example.com: User unknown"}))
	err := send(t, server, context.Background())
	de := assertDeliveryError(t, err, "recipient rcpt_to 550 5.1.1: [redacted] User unknown")
	if !de.Cause.Permanent() {
		t.Fatal("recipient rejection must be permanent")
	}
	if strings.Contains(de.LastError(), "@") {
		t.Fatalf("leaked an address: %q", de.LastError())
	}
}

func TestFakeSMTPRcptToRecipientRejectedWithoutEnhanced(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greeting: "220 fake.example", // no ENHANCEDSTATUSCODES
		mailFrom: "250 Sender ok", rcptTo: "550 User unknown", dataPrompt: "354 Start input", endOfData: "250 ok"})
	err := send(t, server, context.Background())
	assertDeliveryError(t, err, "recipient rcpt_to 550: User unknown")
}

func TestFakeSMTPRcptToRelayDeniedIsConfig(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{rcptTo: "554 5.7.1 Relay access denied"}))
	err := send(t, server, context.Background())
	de := assertDeliveryError(t, err, "config rcpt_to 554 5.7.1: Relay access denied")
	if de.Cause.Permanent() {
		t.Fatal("relay denial must be recoverable")
	}
}

func TestFakeSMTPRcptToMailboxFullIsTransient(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{rcptTo: "452 4.2.2 Mailbox full"}))
	err := send(t, server, context.Background())
	assertDeliveryError(t, err, "transient rcpt_to 452 4.2.2: Mailbox full")
}

func TestFakeSMTPDataContentRejectedIsConfig(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{endOfData: "554 5.7.1 Message rejected"}))
	err := send(t, server, context.Background())
	assertDeliveryError(t, err, "config data 554 5.7.1: response text omitted")
}

func TestFakeSMTPDataRecipientRejectedAtEndOfData(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{endOfData: "550 5.1.1 Recipient rejected"}))
	err := send(t, server, context.Background())
	de := assertDeliveryError(t, err, "recipient data 550 5.1.1: response text omitted")
	if !de.Cause.Permanent() {
		t.Fatal("expected a permanent failure")
	}
}

func TestFakeSMTPDeliveredDespiteClosedConnectionAfterEndOfData(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{closeAfterData: true}))
	if err := send(t, server, context.Background()); err != nil {
		t.Fatalf("expected a nil error (delivered despite the missing QUIT reply), got %v", err)
	}
}

func TestFakeSMTPWithoutRsetSendsQuitNotReset(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{}))
	if err := send(t, server, context.Background()); err != nil {
		t.Fatal(err)
	}
	commands := server.Commands()
	last := commands[len(commands)-1]
	if !strings.HasPrefix(strings.ToUpper(last), "QUIT") {
		t.Fatalf("last command=%q want QUIT (RSET sent despite WithoutRset)", last)
	}
	for _, c := range commands {
		if strings.HasPrefix(strings.ToUpper(c), "RSET") {
			t.Fatalf("RSET was sent despite WithoutRset: %v", commands)
		}
	}
}

func TestFakeSMTPMultilineResponseIsSanitizedToOneLine(t *testing.T) {
	longText := strings.Repeat("filler text that pads the provider response out ", 40)
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{
		rcptTo: "550-5.1.1 first line with user@example.com inside\r\n550-" + longText + "\r\n550 5.1.1 last line",
	}))
	err := send(t, server, context.Background())
	var de *outbox.DeliveryError
	if !errors.As(err, &de) {
		t.Fatalf("not a DeliveryError: %v", err)
	}
	last := de.LastError()
	if strings.ContainsAny(last, "\r\n") || strings.Contains(last, "@") {
		t.Fatalf("not sanitized: %q", last)
	}
	if got := len([]rune(last)); got > 1000 {
		t.Fatalf("length=%d want <= 1000", got)
	}
}

func TestFakeSMTPGreetingTimeout(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greetingHang: true})
	start := time.Now()
	err := send(t, server, context.Background())
	if elapsed := time.Since(start); elapsed > 6*time.Second {
		t.Fatalf("took %s, want <= 6s", elapsed)
	}
	assertDeliveryError(t, err, "network connection: timeout")
}

func TestFakeSMTPEndOfDataTimeout(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{endOfDataHang: true}))
	start := time.Now()
	err := send(t, server, context.Background())
	if elapsed := time.Since(start); elapsed > 6*time.Second {
		t.Fatalf("took %s, want <= 6s", elapsed)
	}
	assertDeliveryError(t, err, "network data: timeout")
}

func TestFakeSMTPContextCanceledBeforeCallNeverConnects(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greeting: "220 fake.example"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := send(t, server, ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if len(server.Commands()) != 0 {
		t.Fatalf("connected despite the canceled context: %v", server.Commands())
	}
}

func TestFakeSMTPContextCanceledDuringGreetingDelay(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greetingHang: true})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	err := send(t, server, ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled, not a DeliveryError", err)
	}
	var de *outbox.DeliveryError
	if errors.As(err, &de) {
		t.Fatalf("got a DeliveryError instead of context.Canceled: %+v", de)
	}
}

// noEnhancedScript is a server that does not advertise ENHANCEDSTATUSCODES: go-mail then leaves
// SendError.EnhancedStatusCode() empty and the adapter must read the code from the text (ADR-025 §3).
func noEnhancedScript(rcptTo string) fakeSMTPScript {
	return fakeSMTPScript{greeting: "220 fake.example", mailFrom: "250 Sender ok", rcptTo: rcptTo,
		dataPrompt: "354 Start input", endOfData: "250 ok"}
}

// ADR-025 §3: a 5.7.1 in RCPT TO is a policy problem (config, recoverable) even when the server
// does not announce ENHANCEDSTATUSCODES; before, the 550 made it a definitive "recipient".
func TestFakeSMTPRcptToRelayDeniedWithoutEnhancedAnnouncementIsConfig(t *testing.T) {
	server := newFakeSMTPServer(t, noEnhancedScript("550 5.7.1 Relay access denied"))
	err := send(t, server, context.Background())
	de := assertDeliveryError(t, err, "config rcpt_to 550 5.7.1: Relay access denied")
	if de.Cause.Permanent() {
		t.Fatal("relay denial must be recoverable")
	}
}

func TestFakeSMTPExtendedCodeReadFromTextWithoutAnnouncement(t *testing.T) {
	for _, tc := range []struct{ name, rcptTo, want string }{
		{"user unknown", "550 5.1.1 user@example.com: User unknown", "recipient rcpt_to 550 5.1.1: [redacted] User unknown"},
		{"554 with 5.1.1", "554 5.1.1 Unknown user", "recipient rcpt_to 554 5.1.1: Unknown user"},
		{"class mismatch is ignored", "550 4.2.2 Mailbox full", "recipient rcpt_to 550: 4.2.2 Mailbox full"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newFakeSMTPServer(t, noEnhancedScript(tc.rcptTo))
			assertDeliveryError(t, send(t, server, context.Background()), tc.want)
		})
	}
}

func TestFakeSMTPGreetingWithMismatchedExtendedClassKeepsItInTheText(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greeting: "554 4.7.1 blocked"})
	assertDeliveryError(t, send(t, server, context.Background()), "config connection 554: 4.7.1 blocked")
}

// ADR-025 §1 (INV-30): the provider quotes the message's link, with its token, at the end of data;
// neither the token nor "token=" may reach LastError.
func TestFakeSMTPDataRejectionQuotingTheLinkNeverLeaksTheToken(t *testing.T) {
	const token = "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCde"
	link := "https://crm.example/reset-password#token=" + token
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{endOfData: "554 5.7.1 Message rejected, URL " + link + " listed"}))
	m, err := newSMTPForTest(SMTPConfig{Host: server.Host(), Port: server.Port(), From: "no-reply@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), Email{To: "user@example.com", Subject: "Hola", TextBody: link, HTMLBody: "<a href=\"" + link + "\">x</a>"})
	de := assertDeliveryError(t, err, "config data 554 5.7.1: response text omitted")
	if strings.Contains(de.LastError(), token) || strings.Contains(de.LastError(), "token=") {
		t.Fatalf("leaked the token: %q", de.LastError())
	}
}

// ADR-025 §2: a URL in a phase where the text is kept is redacted.
func TestFakeSMTPMailFromHelpURLIsRedacted(t *testing.T) {
	server := newFakeSMTPServer(t, readyScript(fakeSMTPScript{
		mailFrom: "550 5.7.1 Sender not verified, see https://provider.example/help?id=1"}))
	assertDeliveryError(t, send(t, server, context.Background()),
		"config mail_from 550 5.7.1: Sender not verified, see [redacted]")
}

// DD-35: when the context's own deadline expires while the server delays the greeting, Send returns
// the context error as is (never a DeliveryError): the Dispatcher decides what it means.
func TestFakeSMTPContextDeadlineDuringGreetingDelay(t *testing.T) {
	server := newFakeSMTPServer(t, fakeSMTPScript{greetingHang: true})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := send(t, server, ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v want context.DeadlineExceeded", err)
	}
	var de *outbox.DeliveryError
	if errors.As(err, &de) {
		t.Fatalf("got a DeliveryError instead of the context error: %+v", de)
	}
}
