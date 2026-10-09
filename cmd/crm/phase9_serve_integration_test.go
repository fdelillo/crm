//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type faultListener struct {
	net.Listener
	fault  error
	failed chan struct{}
}

func (ln *faultListener) Accept() (net.Conn, error) {
	conn, err := ln.Listener.Accept()
	if err != nil {
		select {
		case <-ln.failed:
			return nil, ln.fault
		default:
		}
	}
	return conn, err
}

func (ln *faultListener) fail() { close(ln.failed); _ = ln.Close() }

// Pool.Close already waits for a borrowed connection. This second barrier, after rollback has
// released that connection, proves runServe waits for the Dispatcher itself to finish as well.
type workerExitLog struct {
	safeBuffer
	entered, release chan struct{}
	block            bool
	once             sync.Once
}

func (w *workerExitLog) Write(p []byte) (int, error) {
	if w.block && bytes.Contains(p, []byte("outbox cycle canceled")) {
		w.once.Do(func() { close(w.entered); <-w.release })
	}
	return w.safeBuffer.Write(p)
}

func TestPhase9RunServe(t *testing.T) {
	for _, scenario := range []string{"api-fails", "metrics-fails", "cancel-sending", "cancel-idle"} {
		t.Run(scenario, func(t *testing.T) {
			pool := pgtest.AppPool(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			probe, err := pgx.ConnectConfig(ctx, pgtest.SuperuserPool(t).Config().ConnConfig.Copy())
			if err != nil {
				t.Fatal(err)
			}
			defer probe.Close(context.Background())
			if _, err := probe.Exec(ctx, "UPDATE app.outbox_messages SET status='sent',payload=NULL,sent_at=now() WHERE status='pending'"); err != nil {
				t.Fatal(err)
			}
			var company fixture.Company
			if scenario != "cancel-idle" {
				company = fixture.NewCompany(t, pool)
				if _, err := probe.Exec(ctx, `UPDATE app.outbox_messages SET payload='{"token":"serve-test-token"}' WHERE id=$1`, company.Rows["outbox_messages"]); err != nil {
					t.Fatal(err)
				}
			}
			smtp, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer smtp.Close()
			smtpEntered, smtpRelease, smtpDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
			releaseSMTP := sync.OnceFunc(func() { close(smtpRelease) })
			defer releaseSMTP()
			go func() {
				defer close(smtpDone)
				conn, err := smtp.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				close(smtpEntered)
				<-smtpRelease
				_, _ = io.WriteString(conn, "421 4.3.2 stopping\r\n")
			}()
			values := validEnv()
			u, err := url.Parse(pgtest.AppURL(t))
			if err != nil {
				t.Fatal(err)
			}
			query := u.Query()
			query.Set("pool_max_conns", "4")
			application := "phase9-serve-" + uuid.NewString()
			query.Set("application_name", application)
			u.RawQuery = query.Encode()
			values["DATABASE_URL"] = u.String()
			values["HTTP_ADDR"], values["METRICS_ADDR"] = "127.0.0.1:0", "127.0.0.1:0"
			values["SMTP_HOST"], values["SMTP_PORT"], _ = net.SplitHostPort(smtp.Addr().String())
			values["SMTP_USERNAME"], values["SMTP_PASSWORD"] = "", ""
			listeners := make(chan *faultListener, 2)
			fault := errors.New("permanent injected Accept failure")
			logs := &workerExitLog{entered: make(chan struct{}), release: make(chan struct{}), block: scenario == "cancel-sending"}
			releaseLog := sync.OnceFunc(func() { close(logs.release) })
			defer releaseLog()
			e := env{getenv: mapEnv(values), stdout: logs, stderr: io.Discard, listen: func(ctx context.Context, network, address string) (net.Listener, error) {
				ln, err := (&net.ListenConfig{}).Listen(ctx, network, address)
				if err != nil {
					return nil, err
				}
				controlled := &faultListener{Listener: ln, fault: fault, failed: make(chan struct{})}
				listeners <- controlled
				return controlled, nil
			}}
			done := make(chan error, 1)
			go func() { done <- runServe(ctx, nil, e) }()
			finished := false
			defer func() {
				cancel()
				releaseSMTP()
				releaseLog()
				_ = smtp.Close()
				if !finished {
					select {
					case <-done:
					case <-time.After(app.ShutdownTimeout):
						t.Error("cleanup: runServe did not stop")
					}
				}
				select {
				case <-smtpDone:
				case <-time.After(time.Second):
					t.Error("SMTP goroutine did not end")
				}
			}()
			getListener := func() *faultListener {
				t.Helper()
				select {
				case ln := <-listeners:
					return ln
				case err := <-done:
					finished = true
					t.Fatalf("startup: %v", err)
				case <-time.After(3 * time.Second):
					t.Fatal("listener startup timed out")
				}
				return nil
			}
			api, metrics := getListener(), getListener()
			client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
			for _, endpoint := range []struct {
				ln   *faultListener
				path string
			}{{api, "/healthz"}, {metrics, "/debug/vars"}} {
				response, err := client.Get("http://" + endpoint.ln.Addr().String() + endpoint.path)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = io.Copy(io.Discard, response.Body)
				response.Body.Close()
				if response.StatusCode != 200 {
					t.Fatalf("startup %s: %d", endpoint.path, response.StatusCode)
				}
			}
			if scenario != "cancel-idle" {
				select {
				case <-smtpEntered:
				case <-time.After(3 * time.Second):
					t.Fatal("worker never entered SMTP")
				}
			}
			switch scenario {
			case "api-fails":
				api.fail()
				releaseSMTP()
			case "metrics-fails":
				metrics.fail()
				releaseSMTP()
			case "cancel-sending":
				cancel()
				select {
				case err := <-done:
					finished = true
					t.Fatalf("returned while SMTP retained for <500 ms: %v", err)
				case <-time.After(500 * time.Millisecond):
				}
				releaseSMTP()
				select {
				case <-logs.entered:
				case err := <-done:
					finished = true
					t.Fatalf("returned before worker exit barrier: %v", err)
				case <-time.After(3 * time.Second):
					t.Fatal("worker never reached exit barrier")
				}
				select {
				case err := <-done:
					finished = true
					t.Fatalf("returned before Dispatcher finished: %v", err)
				case <-time.After(500 * time.Millisecond):
				}
				releaseLog()
			case "cancel-idle":
				cancel()
			}
			select {
			case err := <-done:
				finished = true
				if strings.HasSuffix(scenario, "fails") {
					if !errors.Is(err, fault) {
						t.Fatalf("Accept error lost: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("runServe did not cancel both servers/worker before ShutdownTimeout")
			}
			for _, ln := range []*faultListener{api, metrics} {
				conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
				if err == nil {
					conn.Close()
					t.Error("listener still accepts after runServe")
				}
			}
			var active int
			probeCtx, stopProbe := context.WithTimeout(context.Background(), 3*time.Second)
			defer stopProbe()
			poll := time.NewTicker(20 * time.Millisecond)
			defer poll.Stop()
			for {
				if err := probe.QueryRow(probeCtx, "SELECT count(*) FROM pg_stat_activity WHERE application_name=$1", application).Scan(&active); err != nil {
					t.Fatalf("runtime connection probe: %v", err)
				}
				if active == 0 {
					break
				}
				select {
				case <-probeCtx.Done():
					t.Fatalf("runtime connections remain after worker completion: %d", active)
				case <-poll.C:
				}
			}
			if scenario != "cancel-idle" {
				var status, last string
				var attempts int
				var hasPayload bool
				if err := probe.QueryRow(context.Background(), "SELECT status,attempts,coalesce(last_error,''),payload IS NOT NULL FROM app.outbox_messages WHERE id=$1", company.Rows["outbox_messages"]).Scan(&status, &attempts, &last, &hasPayload); err != nil {
					t.Fatal(err)
				}
				if status != "sent" && status != "pending" {
					t.Fatalf("partial state: %s", status)
				}
				if scenario == "cancel-sending" && status == "pending" && (attempts != 0 || last != "" || !hasPayload) {
					t.Fatalf("pending changed on cancellation: %s", fmt.Sprint(status, attempts, last, hasPayload))
				}
			}
		})
	}
}
