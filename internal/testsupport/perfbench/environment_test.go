//go:build bench

package phase9bench_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/db/migrations"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const postgresDigest = "postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722"

type environment struct {
	pool, super *pgxpool.Pool
	appURL      string
	tracer      *benchTracer
	runner      measuredRunner
	users       *identity.Service
	seed, real  *tenant.Service
	hasher      password.Hasher
	logger      *slog.Logger
	first       tenant.Registration
}

func newEnvironment(t *testing.T, ctx context.Context) *environment {
	t.Helper()
	ownerPassword, appPassword, superPassword := uuid.NewString(), uuid.NewString(), uuid.NewString()
	bootstrap, err := filepath.Abs("../../../db/bootstrap/001_roles_and_database.sql")
	if err != nil {
		t.Fatal(err)
	}
	ctr, err := postgres.Run(ctx, postgresDigest, postgres.WithUsername("postgres"), postgres.WithPassword(superPassword), testcontainers.WithEnv(map[string]string{"CRM_OWNER_PASSWORD": ownerPassword, "CRM_APP_PASSWORD": appPassword}), postgres.WithInitScripts(bootstrap), postgres.BasicWaitStrategies())
	if err != nil {
		if ctr != nil {
			_ = testcontainers.TerminateContainer(ctr)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Error(err)
		}
	})
	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	dsn := func(user, pass string) string {
		u := url.URL{Scheme: "postgres", User: url.UserPassword(user, pass), Host: net.JoinHostPort(host, port.Port()), Path: "/crm", RawQuery: "sslmode=disable"}
		return u.String()
	}
	owner, err := sql.Open("pgx", dsn("crm_owner", ownerPassword))
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, owner, migrations.FS)
	if err == nil {
		_, err = provider.Up(ctx)
	}
	_ = owner.Close()
	if err != nil {
		t.Fatal(err)
	}
	e := &environment{logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	e.super, err = pgxpool.New(ctx, dsn("postgres", superPassword))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.super.Close)
	var version string
	if err := e.super.QueryRow(ctx, "SELECT current_setting('server_version')").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if strings.Fields(version)[0] != "18.6" {
		t.Fatalf("production baseline 18.6, got %s", version)
	}
	e.appURL = dsn("crm_app", appPassword)
	config, err := pgxpool.ParseConfig(e.appURL + "&pool_max_conns=8")
	if err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv("PHASE9_BENCH_ARTIFACT_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "trace-"+ctr.GetContainerID()[:12]+".csv")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	e.tracer = &benchTracer{origin: time.Now(), file: file, csv: csv.NewWriter(file)}
	if err := e.tracer.csv.Write([]string{"sample_id", "backend_pid", "label", "start_monotonic_ns", "end_monotonic_ns", "error"}); err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Tracer = e.tracer
	e.pool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		e.pool.Close()
		if err := e.tracer.close(); err != nil {
			t.Error(err)
		}
	})
	if e.pool.Config().MaxConns != 8 {
		t.Fatal("bench pool must have exactly 8 connections")
	}
	mem, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{.MemTotal}}").Output()
	if err != nil {
		t.Fatalf("Docker memory: %v", err)
	}
	t.Logf("BENCH environment run=%s digest=%s container=%s server_version=%s go=%s os=%s arch=%s cpus=%d docker_memory_bytes=%s pool_max_conns=%d trace=%s", os.Getenv("PHASE9_BENCH_RUN"), postgresDigest, ctr.GetContainerID()[:12], version, runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), strings.TrimSpace(string(mem)), e.pool.Config().MaxConns, path)
	c := clock.Real{}
	recorder := audit.NewRecorder()
	e.hasher = password.NewHasher(4)
	e.runner = measuredRunner{db.NewTxRunner(e.pool, db.WithLogger(e.logger))}
	e.users = identity.NewService(e.runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour, identity.WithAuthentication(e.hasher, recorder, bytes.Repeat([]byte{1}, 32), e.logger))
	encoded, err := e.hasher.Hash(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	e.seed = tenant.NewService(e.runner, e.users, industrytemplate.NoopSeeder{}, prehashed{e.hasher, encoded}, recorder, e.logger)
	e.real = tenant.NewService(e.runner, e.users, industrytemplate.NoopSeeder{}, e.hasher, recorder, e.logger)
	return e
}

func signupInput() tenant.Signup {
	return tenant.Signup{Name: "Benchmark", Email: "bench-" + uuid.NewString() + "@example.test", Password: plain, CompanyName: "Benchmark SA", BaseCurrency: "ARS", IndustryTemplateCode: "generic"}
}

func (e *environment) register(ctx context.Context, id string) (tenant.Registration, *sample) {
	s := &sample{ID: id, Start: time.Now()}
	registration, err := e.seed.Register(context.WithValue(ctx, sampleKey{}, s), signupInput(), identity.RequestMeta{})
	s.End = time.Now()
	s.Err = err
	return registration, s
}

func requireSample(t *testing.T, s *sample) {
	t.Helper()
	if s.Err != nil {
		t.Fatalf("registration %s: %v", s.ID, s.Err)
	}
	if _, err := sampleIntervals(s); err != nil {
		t.Fatal(err)
	}
}

func TestBenchTracerDatabaseControls(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	e := newEnvironment(t, ctx)
	_, a := e.register(ctx, "control/1")
	requireSample(t, a)
	_, b := e.register(ctx, "control/2")
	requireSample(t, b)
	if err := noOverlap([]*sample{a, b}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []*sample{a, b} {
		for _, event := range s.Events {
			t.Logf("CONTROL id=%s pid=%d label=%s duration=%s", event.ID, event.PID, event.Label, event.End.Sub(event.Start))
		}
	}
	t.Log("CONTROL query/acquire/BEGIN/provision/COMMIT IDs, known labels and non-overlap passed")
}
