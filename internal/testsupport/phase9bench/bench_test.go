//go:build bench

// Package phase9bench measures T-B905 independently of make check, with production hashing and roles.
package phase9bench_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/containers"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

const plain = "Benchmark-password-935!"

type prehashed struct {
	password.Hasher
	encoded string
}

func (h prehashed) Hash(context.Context, string) (string, error) { return h.encoded, nil }

type transactionMeter struct {
	db.TxRunner
	mu        sync.Mutex
	durations []time.Duration
}

func (m *transactionMeter) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	var started time.Time
	err := m.TxRunner.InSystemTx(ctx, role, func(ctx context.Context, tx db.Tx) error {
		if role == db.RoleSignup {
			started = time.Now()
		}
		return fn(ctx, tx)
	})
	if !started.IsZero() {
		duration := time.Since(started)
		m.mu.Lock()
		m.durations = append(m.durations, duration)
		m.mu.Unlock()
	}
	return err
}
func p95(values []time.Duration) time.Duration {
	copy := append([]time.Duration(nil), values...)
	sort.Slice(copy, func(i, j int) bool { return copy[i] < copy[j] })
	return copy[(95*len(copy)+99)/100-1]
}

type result struct {
	Operation   string  `json:"operation"`
	Samples     int     `json:"samples"`
	P95MS       float64 `json:"p95_ms"`
	TargetMS    float64 `json:"target_ms,omitempty"`
	Unavailable int     `json:"503"`
}

func TestPhase9Benchmark(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool := pgtest.AppPool(t)
	var version string
	if err := pgtest.OwnerPool(t).QueryRow(ctx, "SELECT current_setting('server_version')").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if strings.Fields(version)[0] != "18.6" {
		t.Fatalf("production benchmark baseline PostgreSQL 18.6, got %s", version)
	}
	t.Logf("BENCH environment image=%s server_version=%s go=%s os=%s arch=%s cpus=%d pool_max_conns=%d", containers.Postgres, version, runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), pool.Config().MaxConns)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	c := clock.Real{}
	base := db.NewTxRunner(pool, db.WithLogger(logger))
	hasher := password.NewHasher(4)
	recorder := audit.NewRecorder()
	encoded, err := hasher.Hash(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	users := identity.NewService(base, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour, identity.WithAuthentication(hasher, recorder, bytes.Repeat([]byte{1}, 32), logger))
	seed := tenant.NewService(base, users, industrytemplate.NoopSeeder{}, prehashed{hasher, encoded}, recorder, logger)
	input := func(i int) tenant.Signup {
		return tenant.Signup{Name: "Benchmark", Email: fmt.Sprintf("bench-%d-%s@example.test", i, uuid.NewString()), Password: plain, CompanyName: "Benchmark SA", BaseCurrency: "ARS", IndustryTemplateCode: "generic"}
	}
	var first tenant.Registration
	for i := 0; i < 10000; i++ {
		registration, err := seed.Register(ctx, input(i), identity.RequestMeta{})
		if err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
		if i == 0 {
			first = registration
		}
		if (i+1)%1000 == 0 {
			t.Logf("BENCH seeded=%d", i+1)
		}
	}
	var count int
	if err := pgtest.SuperuserPool(t).QueryRow(ctx, "SELECT count(*) FROM app.tenants").Scan(&count); err != nil || count != 10000 {
		t.Fatalf("seed count=%d err=%v", count, err)
	}
	var results []result
	record := func(name string, values []time.Duration, target time.Duration, unavailable int) {
		t.Helper()
		if len(values) == 0 {
			t.Fatal("empty samples: " + name)
		}
		results = append(results, result{name, len(values), float64(p95(values).Microseconds()) / 1000, float64(target.Microseconds()) / 1000, unavailable})
	}
	// Same pool configuration as runtime. Start the timer after BEGIN/acquisition and include the
	// entire callback through COMMIT. This upper-bounds provision_tenant_role → COMMIT; hashing is precomputed.
	for _, concurrent := range []int{10, 50} {
		meter := &transactionMeter{TxRunner: base}
		svc := tenant.NewService(meter, users, industrytemplate.NoopSeeder{}, prehashed{hasher, encoded}, recorder, logger)
		start := make(chan struct{})
		var ready, done sync.WaitGroup
		ready.Add(concurrent)
		done.Add(concurrent)
		outcomes := make(chan error, concurrent)
		for i := 0; i < concurrent; i++ {
			go func(i int) {
				defer done.Done()
				ready.Done()
				<-start
				_, err := svc.Register(ctx, input(10000+i), identity.RequestMeta{})
				outcomes <- err
			}(i)
		}
		ready.Wait()
		close(start)
		done.Wait()
		close(outcomes)
		unavailable := 0
		for err := range outcomes {
			if errors.Is(err, db.ErrUnavailable) {
				unavailable++
			} else if err != nil {
				t.Fatal(err)
			}
		}
		target := time.Duration(0)
		if concurrent == 10 {
			target = 250 * time.Millisecond
		}
		record(fmt.Sprintf("registration_tx_%d", concurrent), meter.durations, target, unavailable)
	}
	var switches []time.Duration
	for i := 0; i < 200; i++ {
		err := base.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
			start := time.Now()
			err := tx.AsTenant(ctx, first.Tenant.ID)
			duration := time.Since(start)
			if i >= 10 {
				switches = append(switches, duration)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	record("set_local_role_with_catalog", switches, 0, 0)
	var connections []time.Duration
	for i := 0; i < 30; i++ {
		start := time.Now()
		conn, err := pgx.Connect(ctx, pgtest.AppURL(t))
		duration := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		conn.Close(ctx)
		connections = append(connections, duration)
	}
	record("crm_app_connect", connections, 0, 0)
	// A validator hit must never read storage. Its key and ETag use the same UUID derivation as production.
	logoID := uuid.New()
	key := "tenants/" + first.Tenant.ID.String() + "/logo/" + logoID.String() + ".png"
	if _, err := pgtest.SuperuserPool(t).Exec(ctx, "UPDATE app.tenants SET logo_object_key=$2,logo_content_type='image/png' WHERE id=$1", first.Tenant.ID, key); err != nil {
		t.Fatal(err)
	}
	companies := tenant.NewService(base, users, industrytemplate.NoopSeeder{}, hasher, recorder, logger)
	root := app.NewRootHandler(app.RootDeps{API: app.BuildAPIRouter(users, companies, c, logger), Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: http.NotFoundHandler()}, app.NewCommonMiddleware(logger, true, nil))
	measure := func(name, method, path string, n, code int, target time.Duration, body func(int) any) {
		t.Helper()
		var values []time.Duration
		for i := 0; i < n+10; i++ {
			var data []byte
			if body != nil {
				var err error
				data, err = json.Marshal(body(i))
				if err != nil {
					t.Fatal(err)
				}
			}
			req := httptest.NewRequest(method, "https://crm.example.test"+path, bytes.NewReader(data))
			req.Header.Set("Origin", "https://crm.example.test")
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = fmt.Sprintf("198.18.%d.%d:4567", i/250, (i%250)+1)
			if method == http.MethodGet {
				req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: first.Session.RawToken})
			}
			if path == "/api/v1/tenant/logo" {
				req.Header.Set("If-None-Match", `"`+logoID.String()+`"`)
			}
			rec := httptest.NewRecorder()
			start := time.Now()
			root.ServeHTTP(rec, req)
			duration := time.Since(start)
			if rec.Code != code {
				t.Fatalf("%s sample=%d status=%d body=%s", name, i, rec.Code, rec.Body)
			}
			if i >= 10 {
				values = append(values, duration)
			}
		}
		record(name, values, target, 0)
	}
	measure("GET_me", "GET", "/api/v1/me", 200, 200, 50*time.Millisecond, nil)
	measure("GET_logo_304", "GET", "/api/v1/tenant/logo", 200, 304, 50*time.Millisecond, nil)
	measure("POST_login", "POST", "/api/v1/auth/login", 80, 200, 400*time.Millisecond, func(int) any { return map[string]string{"email": first.User.Email, "password": plain} })
	measure("POST_signup", "POST", "/api/v1/auth/signup", 60, 201, time.Second, func(i int) any { return input(20000 + i) })
	recovered, failed := db.SetRoleRetryCount(db.RetryRecovered), db.SetRoleRetryCount(db.RetryFailed)
	t.Logf("BENCH set_role_retry_total recovered=%d failed=%d total=%d", recovered, failed, recovered+failed)
	bad := false
	for _, value := range results {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("BENCH result " + string(data))
		if value.TargetMS > 0 && (value.P95MS >= value.TargetMS || (value.Operation == "registration_tx_10" && value.Unavailable != 0)) {
			bad = true
		}
	}
	if bad {
		t.Fatal("T-B905 target failed: stop and return to architect under ADR-005; targets and design unchanged")
	}
	if recovered+failed != 0 {
		t.Fatal("T-B905 SET ROLE retry requires runbook analysis")
	}
}
