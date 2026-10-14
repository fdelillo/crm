//go:build integration

package app_test

import (
	"bytes"
	"context"
	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/config"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type shutdownDelivery struct {
	entered, release chan struct{}
	success          bool
}

func (h *shutdownDelivery) Handle(ctx context.Context, _ outbox.Message) error {
	close(h.entered)
	<-ctx.Done()
	if h.success {
		<-h.release
		return nil
	}
	return ctx.Err()
}
func TestPhase9Shutdown(t *testing.T) {
	if app.ShutdownTimeout < outbox.SendBudget+5*time.Second {
		t.Fatalf("shutdown budget=%s send=%s", app.ShutdownTimeout, outbox.SendBudget)
	}
	for _, success := range []bool{false, true} {
		t.Run(map[bool]string{false: "canceled", true: "sent"}[success], func(t *testing.T) {
			f := e2e.New(t, pgtest.AppPool(t))
			ctx := context.Background()
			// The state probe is dedicated. Only the worker occupies a runtime connection during its send.
			probe, err := pgx.ConnectConfig(ctx, pgtest.SuperuserPool(t).Config().ConnConfig.Copy())
			if err != nil {
				t.Fatal(err)
			}
			defer probe.Close(ctx)
			if _, err := probe.Exec(ctx, "UPDATE app.outbox_messages SET status='sent',payload=NULL,sent_at=now() WHERE status='pending'"); err != nil {
				t.Fatal(err)
			}
			if err := f.Runner.InTenantTx(ctx, f.A.ID, func(ctx context.Context, tx db.Tx) error {
				return outbox.NewEnqueuer(f.Clock).Enqueue(ctx, tx, outbox.Message{TenantID: f.A.ID, Kind: "email", Template: "password_reset", Recipient: "shutdown@example.test", Payload: map[string]string{"link": "test"}})
			}); err != nil {
				t.Fatal(err)
			}
			type messageState struct {
				status   string
				attempts int
				next     time.Time
				last     string
				payload  bool
			}
			state := func() messageState {
				t.Helper()
				var s messageState
				if err := probe.QueryRow(ctx, "SELECT status,attempts,next_attempt_at,coalesce(last_error,''),payload IS NOT NULL FROM app.outbox_messages WHERE tenant_id=$1 AND status='pending' ORDER BY created_at DESC LIMIT 1", f.A.ID).Scan(&s.status, &s.attempts, &s.next, &s.last, &s.payload); err != nil {
					t.Fatal(err)
				}
				return s
			}
			before := state()
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			delivery := &shutdownDelivery{entered: make(chan struct{}), release: make(chan struct{}), success: success}
			releaseSend := sync.OnceFunc(func() { close(delivery.release) })
			defer releaseSend()
			dispatcher := outbox.NewDispatcher(f.Runner, delivery, f.Clock, logger)
			processCtx, signal := context.WithCancel(ctx)
			defer signal()
			workerDone := make(chan error, 1)
			go func() { workerDone <- dispatcher.Run(processCtx) }()
			requestEntered := make(chan struct{})
			requestRelease := make(chan struct{})
			releaseRequest := sync.OnceFunc(func() { close(requestRelease) })
			defer releaseRequest()
			shutdownEntered := make(chan struct{})
			slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(requestEntered)
				<-requestRelease
				if success {
					_, _ = io.WriteString(w, "done")
				} else {
					httpx.WriteDBError(w, r, db.ErrCanceled, logger)
				}
			})
			root := app.NewRootHandler(app.RootDeps{API: slow, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: http.NotFoundHandler()}, app.NewCommonMiddleware(logger, true, nil))
			srv, err := app.NewServer(config.Config{}, root)
			if err != nil {
				t.Fatal(err)
			}
			srv.RegisterOnShutdown(func() { close(shutdownEntered) })
			metrics := app.NewMetricsServer(config.Config{})
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			metricsLn, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			apiDone := make(chan error, 1)
			metricsDone := make(chan error, 1)
			go func() { apiDone <- app.Serve(processCtx, srv, ln, logger) }()
			go func() { metricsDone <- app.Serve(processCtx, metrics, metricsLn, logger) }()
			type response struct {
				code int
				body string
				err  error
			}
			requestDone := make(chan response, 1)
			go func() {
				client := http.Client{Timeout: 10 * time.Second}
				resp, err := client.Get("http://" + ln.Addr().String() + "/api/v1/shutdown-probe")
				if err != nil {
					requestDone <- response{err: err}
					return
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				requestDone <- response{resp.StatusCode, string(body), err}
			}()
			// A timeout only bounds a broken test; all ordering is established by channels and server hooks.
			bound := time.NewTimer(10 * time.Second)
			defer bound.Stop()
			await := func(ch <-chan struct{}) {
				t.Helper()
				select {
				case <-ch:
				case <-bound.C:
					t.Fatal("shutdown test did not reach barrier")
				}
			}
			await(requestEntered)
			await(delivery.entered)
			signal()
			await(shutdownEntered)
			select {
			case err := <-metricsDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-bound.C:
				t.Fatal("metrics waited for the worker")
			}
			connection, err := net.Dial("tcp", metricsLn.Addr().String())
			if err == nil {
				connection.Close()
				t.Error("metrics still accepting after signal")
			}
			select {
			case err := <-apiDone:
				t.Fatalf("API returned before request completed: %v", err)
			default:
			}
			if success {
				select {
				case err := <-workerDone:
					t.Fatalf("worker returned before successful send finished: %v", err)
				default:
				}
				releaseSend()
			}
			releaseRequest()
			got := <-requestDone
			if got.err != nil {
				t.Fatal(got.err)
			}
			if success && (got.code != 200 || got.body != "done") {
				t.Errorf("request=%+v", got)
			}
			if !success && (got.code != 503 || !strings.Contains(got.body, "service_unavailable")) {
				t.Errorf("internal cancellation=%+v", got)
			}
			if err := <-apiDone; err != nil {
				t.Fatal(err)
			}
			if err := <-workerDone; err != nil {
				t.Fatal(err)
			}
			if success {
				var sent bool
				if err := probe.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM app.outbox_messages WHERE tenant_id=$1 AND template='password_reset' AND status='sent' AND payload IS NULL)", f.A.ID).Scan(&sent); err != nil || !sent {
					t.Fatalf("success not persisted %v/%v", sent, err)
				}
			} else {
				after := state()
				if after != before {
					t.Errorf("canceled message changed before=%+v after=%+v", before, after)
				}
				if !strings.Contains(logs.String(), `"outcome":"canceled"`) {
					t.Error("cancellation not logged")
				}
			}
			if strings.Contains(logs.String(), `"level":"ERROR"`) {
				t.Errorf("shutdown ERROR: %s", &logs)
			}
		})
	}
	// An internal cancellation is observable as 503 with the live request context.
	rec := httptest.NewRecorder()
	httpx.WriteDBError(rec, httptest.NewRequest("GET", "/", nil), db.ErrCanceled, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if rec.Code != 503 {
		t.Errorf("internal cancellation=%d", rec.Code)
	}
}
