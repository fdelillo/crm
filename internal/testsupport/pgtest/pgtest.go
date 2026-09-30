package pgtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/db/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver for goose
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	image           = "postgres:18"
	bootstrapFile   = "001_roles_and_database.sql"
	startupDeadline = 3 * time.Minute
)

// cluster is the PostgreSQL 18 shared by every test of a package: roles are cluster-wide, so one
// container per package (not per test) with each test creating its own companies (ADR-012).
type cluster struct {
	container *postgres.PostgresContainer
	ownerURL  string
	appURL    string
	app       *pgxpool.Pool
	owner     *pgxpool.Pool
	super     *pgxpool.Pool
}

// provider starts the cluster once and remembers the outcome, so every test of the package sees the
// same result (a start failure fails all of them). start is a field so the failure path can be tested
// without breaking Docker.
type provider struct {
	once  sync.Once
	c     *cluster
	err   error
	start func(context.Context) (*cluster, error)
}

var pkg = &provider{start: startCluster}

// Main is for TestMain: it runs the package's tests and then removes the container.
//
//	func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }
//
// The container is started lazily by the first test that needs it, so a run that selects no
// database test never pays for it. If TestMain does not use Main the testcontainers reaper still
// removes the container when the process ends.
func Main(m *testing.M) int {
	code := m.Run()
	stop()
	return code
}

// Start makes sure the PostgreSQL 18 of this package is running, with the bootstrap applied as the
// container superuser and the migrations applied as crm_owner. It fails t with a clear message when
// Docker is unavailable: an integration test never skips silently (ADR-012).
func Start(t testing.TB) {
	t.Helper()
	get(t)
}

// AppPool returns a pool connected as crm_app, the runtime role (INV-18). Use it to test behaviour.
func AppPool(t testing.TB) *pgxpool.Pool { t.Helper(); return get(t).app }

// OwnerPool returns a pool connected as crm_owner. Only for catalog assertions and fixtures that
// need ownership; never to prove behaviour.
func OwnerPool(t testing.TB) *pgxpool.Pool { t.Helper(); return get(t).owner }

// SuperuserPool returns a pool connected as the container superuser. Only to prepare fixtures that
// need privileges (creating roles or helper functions); a superuser ignores RLS, so it must never
// be used to test behaviour (INV-18).
func SuperuserPool(t testing.TB) *pgxpool.Pool { t.Helper(); return get(t).super }

// OwnerURL is the connection URL of crm_owner, for tests that run the migrations themselves.
func OwnerURL(t testing.TB) string { t.Helper(); return get(t).ownerURL }

// AppURL is the connection URL of crm_app, for tests that need their own pool (for example a pool of
// a single connection to observe what a connection looks like after a transaction).
func AppURL(t testing.TB) string { t.Helper(); return get(t).appURL }

// ApplyBootstrap runs db/bootstrap/ again inside the container, as a DBA would, to check it is
// idempotent. It runs the same file the container executed at startup.
func ApplyBootstrap(t testing.TB) {
	t.Helper()
	c := get(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	code, out, err := c.container.Exec(ctx, []string{
		"psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres",
		"-f", "/docker-entrypoint-initdb.d/" + bootstrapFile,
	}, exec.Multiplexed())
	if err != nil {
		t.Fatalf("pgtest: running the bootstrap again: %v", err)
	}
	text, _ := io.ReadAll(out)
	if code != 0 {
		t.Fatalf("pgtest: bootstrap exited with %d:\n%s", code, text)
	}
}

func get(t testing.TB) *cluster {
	t.Helper()
	return pkg.get(t)
}

// get fails t (never skips) when the cluster cannot start: without Docker an integration test must
// not pass silently in CI (ADR-012).
func (p *provider) get(t testing.TB) *cluster {
	t.Helper()
	p.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), startupDeadline)
		defer cancel()
		p.c, p.err = p.start(ctx)
	})
	if p.err != nil {
		t.Fatalf("pgtest: Docker is required to run integration tests (make test-int) and PostgreSQL 18 could not start: %v", p.err)
	}
	return p.c
}

func startCluster(ctx context.Context) (*cluster, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	superPassword, ownerPassword, appPassword := randomPassword(), randomPassword(), randomPassword()

	ctr, err := postgres.Run(ctx, image,
		postgres.WithUsername("postgres"),
		postgres.WithPassword(superPassword),
		// The bootstrap reads these through psql \getenv (never stored in the repository).
		testcontainers.WithEnv(map[string]string{
			"CRM_OWNER_PASSWORD": ownerPassword,
			"CRM_APP_PASSWORD":   appPassword,
		}),
		postgres.WithInitScripts(filepath.Join(root, "db", "bootstrap", bootstrapFile)),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		if ctr != nil {
			_ = testcontainers.TerminateContainer(ctr)
		}
		return nil, fmt.Errorf("starting %s: %w", image, err)
	}

	c := &cluster{container: ctr}
	host, err := ctr.Host(ctx)
	if err != nil {
		c.close()
		return nil, err
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		c.close()
		return nil, err
	}
	dsn := func(user, password string) string {
		u := url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(user, password),
			Host:     net.JoinHostPort(host, port.Port()),
			Path:     "/crm",
			RawQuery: "sslmode=disable",
		}
		return u.String()
	}
	c.ownerURL = dsn("crm_owner", ownerPassword)
	c.appURL = dsn("crm_app", appPassword)

	if err := migrate(ctx, c.ownerURL); err != nil {
		c.close()
		return nil, err
	}
	for _, p := range []struct {
		dst  **pgxpool.Pool
		user string
		pass string
	}{{&c.app, "crm_app", appPassword}, {&c.owner, "crm_owner", ownerPassword}, {&c.super, "postgres", superPassword}} {
		pool, err := pgxpool.New(ctx, dsn(p.user, p.pass))
		if err != nil {
			c.close()
			return nil, fmt.Errorf("opening pool as %s: %w", p.user, err)
		}
		*p.dst = pool
	}
	return c, nil
}

// migrate applies the embedded migrations as crm_owner, exactly as `crm migrate up` does.
func migrate(ctx context.Context, ownerURL string) error {
	db, err := sql.Open("pgx", ownerURL)
	if err != nil {
		return fmt.Errorf("opening migration connection: %w", err)
	}
	defer func() { _ = db.Close() }() // the work is done by now: a close error changes nothing
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("creating goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("running migrations as crm_owner: %w", err)
	}
	return nil
}

func (c *cluster) close() {
	for _, p := range []*pgxpool.Pool{c.app, c.owner, c.super} {
		if p == nil {
			continue
		}
		// Close waits for every acquired connection to come back; a test that leaked one must not
		// hang the whole package (and leave the container running).
		done := make(chan struct{})
		go func() { p.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			fmt.Fprintln(os.Stderr, "pgtest: a pool did not close in 10 s: some test left a connection or transaction unreleased")
		}
	}
	if c.container != nil {
		_ = testcontainers.TerminateContainer(c.container)
	}
}

func stop() {
	if pkg.c != nil {
		pkg.c.close()
	}
}

// repoRoot finds the directory holding go.mod, walking up from the package under test.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found above the working directory")
		}
		dir = parent
	}
}

func randomPassword() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return hex.EncodeToString(b)
}
