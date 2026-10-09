//go:build bench

package phase9bench_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const plain = "Benchmark-password-935!"

type prehashed struct {
	password.Hasher
	encoded string
}

func (h prehashed) Hash(context.Context, string) (string, error) { return h.encoded, nil }

func logMeasurement(t *testing.T, id string, data any) {
	t.Helper()
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("BENCH measurement %s %s", id, encoded)
}

func summarizeSamples(t *testing.T, samples []*sample) map[string]distribution {
	t.Helper()
	values, err := collect(samples)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]distribution{}
	for label, values := range values {
		result[label] = summarize(values)
	}
	return result
}

func (e *environment) root() http.Handler {
	return app.NewRootHandler(app.RootDeps{API: app.BuildAPIRouter(e.users, e.real, clock.Real{}, e.logger), Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: http.NotFoundHandler()}, app.NewCommonMiddleware(e.logger, true, nil))
}
func (e *environment) request(ctx context.Context, root http.Handler, method, path string, input any, etag string, index int) (time.Duration, int) {
	var body []byte
	if input != nil {
		var err error
		body, err = json.Marshal(input)
		if err != nil {
			panic(err)
		}
	}
	req := httptest.NewRequestWithContext(ctx, method, "https://crm.example.test"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://crm.example.test")
	req.RemoteAddr = fmt.Sprintf("198.18.%d.%d:4567", (index/250)%250, (index%250)+1)
	if method == http.MethodGet {
		req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: e.first.Session.RawToken})
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	rec := httptest.NewRecorder()
	started := time.Now()
	root.ServeHTTP(rec, req)
	return time.Since(started), rec.Code
}

func TestPhase9Measurements(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()
	e := newEnvironment(t, ctx)
	// M-1. No extra activity while creating roles. All ten blocks retain every trace/sample.
	var blocks []map[string]distribution
	for block := 0; block < 10; block++ {
		started := time.Now()
		samples := make([]*sample, 0, 1000)
		for i := 0; i < 1000; i++ {
			registration, s := e.register(ctx, fmt.Sprintf("M1/%05d", block*1000+i+1))
			requireSample(t, s)
			if block == 0 && i == 0 {
				e.first = registration
			}
			samples = append(samples, s)
		}
		durations := summarizeSamples(t, samples)
		blocks = append(blocks, durations)
		logMeasurement(t, "M-1", map[string]any{"first": block*1000 + 1, "last": (block + 1) * 1000, "block_ms": float64(time.Since(started).Nanoseconds()) / 1e6, "intervals": pick(durations, "hold", "provision", "statement:set_role:tenant", "pre_callback", "callback")})
	}
	var count int
	if err := e.super.QueryRow(ctx, "SELECT count(*) FROM app.tenants").Scan(&count); err != nil || count != 10000 {
		t.Fatalf("tenants count=%d err=%v", count, err)
	}
	// M-2. Sequential/no other database work; the self-control also proves non-overlap.
	var isolated []*sample
	for i := 0; i < 100; i++ {
		_, s := e.register(ctx, fmt.Sprintf("M2/%03d", i+1))
		requireSample(t, s)
		isolated = append(isolated, s)
	}
	if err := noOverlap(isolated); err != nil {
		t.Fatal(err)
	}
	baseline := summarizeSamples(t, isolated)
	logMeasurement(t, "M-2", baseline)
	// M-3. Same role/attributes/GRANT options, measured separately; every experiment rolls back.
	ddl := map[string][]time.Duration{}
	for i := 0; i < 50; i++ {
		func() {
			tx, err := e.super.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err := tx.Exec(ctx, "SET LOCAL ROLE crm_provisioner"); err != nil {
				t.Fatal(err)
			}
			role := pgx.Identifier{db.TenantRoleName(uuid.New())}.Sanitize()
			for _, statement := range []struct{ label, sql string }{{"CREATE_ROLE", "CREATE ROLE " + role + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"}, {"GRANT_crm_tenant", "GRANT crm_tenant TO " + role + " WITH INHERIT TRUE, SET FALSE"}, {"GRANT_crm_app", "GRANT " + role + " TO crm_app WITH INHERIT FALSE, SET TRUE"}} {
				start := time.Now()
				_, err := tx.Exec(ctx, statement.sql)
				duration := time.Since(start)
				if err != nil {
					t.Fatal(err)
				}
				ddl[statement.label] = append(ddl[statement.label], duration)
			}
		}()
	}
	ddlResult := map[string]distribution{}
	for label, values := range ddl {
		ddlResult[label] = summarize(values)
	}
	logMeasurement(t, "M-3", ddlResult)
	// M-4. Seven borrowed connections force Register onto an eighth distinct backend.
	var first, hot, afterMe []time.Duration
	root := e.root()
	for i := 0; i < 20; i++ {
		func() {
			var held []*pgxpool.Conn
			defer func() {
				for _, conn := range held {
					conn.Release()
				}
			}()
			for j := 0; j < 7; j++ {
				conn, err := e.pool.Acquire(ctx)
				if err != nil {
					t.Fatal(err)
				}
				held = append(held, conn)
			}
			_, s := e.register(ctx, fmt.Sprintf("M4/connections/%02d", i+1))
			requireSample(t, s)
			registrationPID := s.Events[0].PID
			for _, conn := range held {
				if conn.Conn().PgConn().PID() == registrationPID {
					t.Fatal("M-4 reused registration backend")
				}
				tx, err := conn.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for _, values := range []*[]time.Duration{&first, &hot} {
					start := time.Now()
					_, err := tx.Exec(ctx, roleSQL("crm_auth"))
					elapsed := time.Since(start)
					if err != nil {
						_ = tx.Rollback(ctx)
						t.Fatal(err)
					}
					*values = append(*values, elapsed)
				}
				if err := tx.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
			}
		}()
		_, s := e.register(ctx, fmt.Sprintf("M4/me/%02d", i+1))
		requireSample(t, s)
		duration, code := e.request(ctx, root, "GET", "/api/v1/me", nil, "", i)
		if code != 200 {
			t.Fatalf("M-4 GET me=%d", code)
		}
		afterMe = append(afterMe, duration)
	}
	m4 := map[string]distribution{"first": summarize(first), "hot": summarize(hot), "me_after_registration": summarize(afterMe)}
	logMeasurement(t, "M-4", m4)
	// M-5. Five repetitions per burst size, including pool acquisition in end_to_end.
	unavailable := map[int]int{}
	var duringMe []time.Duration
	for _, size := range []int{10, 50} {
		var all []*sample
		for repetition := 0; repetition < 5; repetition++ {
			e.waitPoolIdle(t, ctx)
			start, finished := make(chan struct{}), make(chan struct{})
			var ready, done sync.WaitGroup
			ready.Add(size)
			done.Add(size)
			outcomes := make(chan *sample, size)
			for i := 0; i < size; i++ {
				go func(i int) {
					defer done.Done()
					ready.Done()
					<-start
					_, s := e.register(ctx, fmt.Sprintf("M5/%d/%d/%02d", size, repetition+1, i+1))
					outcomes <- s
				}(i)
			}
			meDone := make(chan struct{})
			var me []time.Duration
			var meCodes []int
			if size == 50 {
				go func() {
					defer close(meDone)
					<-start
					for {
						select {
						case <-finished:
							return
						default:
						}
						duration, code := e.request(ctx, root, "GET", "/api/v1/me", nil, "", 0)
						me = append(me, duration)
						meCodes = append(meCodes, code)
					}
				}()
			} else {
				close(meDone)
			}
			ready.Wait()
			close(start)
			done.Wait()
			close(finished)
			<-meDone
			close(outcomes)
			for s := range outcomes {
				if errors.Is(s.Err, db.ErrUnavailable) {
					unavailable[size]++
				} else if s.Err != nil {
					t.Fatalf("M-5 registration: %v", s.Err)
				}
				all = append(all, s)
			}
			for _, code := range meCodes {
				if code != 200 {
					t.Errorf("GET me during burst status=%d", code)
				}
			}
			duringMe = append(duringMe, me...)
		}
		summary := summarizeSamples(t, all)
		var waits []time.Duration
		values, err := collect(all)
		if err != nil {
			t.Fatal(err)
		}
		for _, duration := range values["provision"] {
			waits = append(waits, duration-time.Duration(baseline["provision"].P50MS*1e6))
		}
		logMeasurement(t, "M-5", map[string]any{"size": size, "samples": len(all), "503": unavailable[size], "intervals": pick(summary, "end_to_end", "pool_wait", "provision", "hold", "callback"), "estimated_lock_wait": summarize(waits)})
	}
	meBurst := summarize(duringMe)
	logMeasurement(t, "M-5-me", meBurst)
	// M-6. Preserve all eight measurements of the original run, without the old mixed target.
	m6 := e.measureOriginal(t, ctx, root)
	recovered, failed := db.SetRoleRetryCount(db.RetryRecovered), db.SetRoleRetryCount(db.RetryFailed)
	logMeasurement(t, "retry", map[string]int64{"recovered": recovered, "failed": failed, "total": recovered + failed})
	for _, target := range []struct {
		id    string
		pass  bool
		value any
	}{{"T-1", baseline["hold"].P95MS < 140, baseline["hold"]}, {"T-2", unavailable[10] == 0, map[string]int{"samples": 50, "503": unavailable[10]}}, {"T-3", m6["GET_me"].P95MS < 50 && m6["GET_logo_304"].P95MS < 50 && m6["POST_login"].P95MS < 400 && m6["POST_signup"].P95MS < 1000, m6}} {
		logMeasurement(t, "target", map[string]any{"id": target.id, "passed": target.pass, "values": target.value})
		if !target.pass {
			t.Errorf("BENCH target %s failed; report and complete the phase checkpoint", target.id)
		}
	}
	if recovered+failed != 0 {
		t.Error("set_role_retry_total != 0: analyze with plan 12.3 runbook")
	}
	ratio := blocks[9]["hold"].P95MS / blocks[4]["hold"].P95MS
	for _, rule := range []struct {
		id        string
		triggered bool
		value     float64
	}{{"D-a", unavailable[50] > 0, float64(unavailable[50])}, {"D-b", meBurst.P95MS > 1000, meBurst.P95MS}, {"D-c", m4["first"].P95MS > 250, m4["first"].P95MS}, {"D-d", ratio > 2.5, ratio}} {
		t.Logf("BENCH rule %s triggered=%t value=%.6f", rule.id, rule.triggered, rule.value)
	}
	t.Log("BENCH self-controls: all registration statements have sample/PID/known label; exactly one BEGIN/provision/COMMIT per success; M-2 non-overlap")
}

func pick(values map[string]distribution, labels ...string) map[string]distribution {
	picked := map[string]distribution{}
	for _, label := range labels {
		picked[label] = values[label]
	}
	return picked
}
func (e *environment) waitPoolIdle(t *testing.T, ctx context.Context) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for e.pool.Stat().AcquiredConns() != 0 {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-deadline.C:
			t.Fatal("burst leaked a pool connection")
		case <-ticker.C:
		}
	}
}

func (e *environment) measureOriginal(t *testing.T, ctx context.Context, root http.Handler) map[string]distribution {
	t.Helper()
	result := map[string]distribution{}
	for _, size := range []int{10, 50} {
		start := make(chan struct{})
		var ready, done sync.WaitGroup
		ready.Add(size)
		done.Add(size)
		samples := make(chan *sample, size)
		for i := 0; i < size; i++ {
			go func(i int) {
				defer done.Done()
				ready.Done()
				<-start
				_, s := e.register(ctx, fmt.Sprintf("M6/tx/%d/%02d", size, i+1))
				samples <- s
			}(i)
		}
		ready.Wait()
		close(start)
		done.Wait()
		close(samples)
		var all []*sample
		count503 := 0
		for s := range samples {
			if errors.Is(s.Err, db.ErrUnavailable) {
				count503++
			} else if s.Err != nil {
				t.Fatal(s.Err)
			}
			all = append(all, s)
		}
		result[fmt.Sprintf("registration_tx_%d", size)] = summarizeSamples(t, all)["callback"]
		logMeasurement(t, "M-6-503", map[string]int{"size": size, "503": count503})
	}
	var warm, switches, connections []time.Duration
	for i := 0; i < 200; i++ {
		err := e.runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
			start := time.Now()
			err := tx.AsTenant(ctx, e.first.Tenant.ID)
			duration := time.Since(start)
			if i < 10 {
				warm = append(warm, duration)
			} else {
				switches = append(switches, duration)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	result["set_local_role_with_catalog"] = summarize(switches)
	logMeasurement(t, "M-6-role-warm", summarize(warm))
	for i := 0; i < 30; i++ {
		start := time.Now()
		conn, err := pgx.Connect(ctx, e.appURL)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.Close(ctx); err != nil {
			t.Fatal(err)
		}
		connections = append(connections, elapsed)
	}
	result["crm_app_connect"] = summarize(connections)
	logoID := uuid.New()
	key := "tenants/" + e.first.Tenant.ID.String() + "/logo/" + logoID.String() + ".png"
	if _, err := e.super.Exec(ctx, "UPDATE app.tenants SET logo_object_key=$2,logo_content_type='image/png' WHERE id=$1", e.first.Tenant.ID, key); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []struct {
		name, method, path string
		samples, status    int
	}{{"GET_me", "GET", "/api/v1/me", 200, 200}, {"GET_logo_304", "GET", "/api/v1/tenant/logo", 200, 304}, {"POST_login", "POST", "/api/v1/auth/login", 80, 200}, {"POST_signup", "POST", "/api/v1/auth/signup", 60, 201}} {
		var durations []time.Duration
		for i := 0; i < operation.samples+10; i++ {
			var input any
			etag := ""
			if operation.name == "POST_login" {
				input = map[string]string{"email": e.first.User.Email, "password": plain}
			}
			if operation.name == "POST_signup" {
				input = signupInput()
			}
			if operation.name == "GET_logo_304" {
				etag = `"` + logoID.String() + `"`
			}
			duration, status := e.request(ctx, root, operation.method, operation.path, input, etag, i)
			if status != operation.status {
				t.Fatalf("M-6 %s status=%d", operation.name, status)
			}
			if i >= 10 {
				durations = append(durations, duration)
			}
		}
		result[operation.name] = summarize(durations)
	}
	logMeasurement(t, "M-6", result)
	// Export structured summaries independently of Go's text prefix, for the report generator.
	dir := os.Getenv("PHASE9_BENCH_ARTIFACT_DIR")
	if dir != "" {
		path := filepath.Join(dir, "m6.json")
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return result
}

// Percentiles are the ceil rank, including ten-sample batches where p95 is the maximum.
func TestBenchPercentiles(t *testing.T) {
	values := []time.Duration{10, 1, 4, 6, 8, 2, 5, 7, 3, 9}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	got := summarize(values)
	if got.P95MS != float64(10)/1e6 || got.P50MS != float64(5)/1e6 {
		t.Fatal(got)
	}
}
