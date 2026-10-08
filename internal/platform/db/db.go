package db

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is the interface sqlc generates for pgx/v5; *pgxpool.Pool, pgx.Tx and Tx satisfy it.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// SystemRole is a role for the flows that run before the company is known (ADR-005, plan §4.4).
type SystemRole string

const (
	RoleAuth   SystemRole = "crm_auth"
	RoleWorker SystemRole = "crm_worker"
	RoleSignup SystemRole = "crm_signup"
)

// Tx is a database transaction bound, as much as one, to a company. Only this package changes the
// role of a transaction, and always with SET LOCAL, so it is undone at COMMIT or ROLLBACK and a
// connection returns to the pool as crm_app with no privileges (INV-02, INV-03).
type Tx interface {
	DBTX
	// TenantID reports the company the transaction is bound to, if any.
	TenantID() (uuid.UUID, bool)
	// AsTenant switches to the role of company tenantID. It returns ErrTenantAlreadyBound if the
	// transaction was already bound to another company; nothing runs as the other one.
	AsTenant(ctx context.Context, tenantID uuid.UUID) error
	// AsSystem switches to a system role (crm_auth, crm_worker, crm_signup). The binding to a
	// company, if any, is kept: the transaction may go back to that company later.
	AsSystem(ctx context.Context, role SystemRole) error
}

// TxRunner opens the transactions of the application. Business code never touches the pool.
type TxRunner interface {
	// InTenantTx runs fn as the role of the company tenantID. It commits if fn returns nil and rolls
	// back if fn returns an error or panics (the panic continues). fn runs at most once: if PostgreSQL
	// refuses the first SET LOCAL ROLE (DD-34) the whole transaction is started again, once, before fn
	// has done anything; after fn started, nothing is retried.
	InTenantTx(ctx context.Context, tenantID uuid.UUID, fn func(ctx context.Context, tx Tx) error) error
	// InSystemTx runs fn as a system role; fn may then move to one company with AsTenant.
	InSystemTx(ctx context.Context, role SystemRole, fn func(ctx context.Context, tx Tx) error) error
}

// TenantRoleName is the PostgreSQL role of a company: "crm_t_" + the 32 hex digits of its UUID.
// It is the only place in Go that derives it (the SQL side is app.current_tenant_id()).
func TenantRoleName(tenantID uuid.UUID) string {
	return "crm_t_" + hex.EncodeToString(tenantID[:])
}

// rollbackTimeout bounds the ROLLBACK issued after the caller's context is gone.
const rollbackTimeout = 5 * time.Second

type runner struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
	// fault, when set (tests only, through export_test.go), runs before every SET LOCAL ROLE and may
	// return an error as if PostgreSQL had refused the switch.
	fault func(role string) error
}

// Option configures a TxRunner.
type Option func(*runner)

// WithLogger sets the logger for the runner's own events (default: slog.Default()).
func WithLogger(l *slog.Logger) Option { return func(r *runner) { r.logger = l } }

// NewTxRunner returns the TxRunner over a pool connected as crm_app.
func NewTxRunner(pool *pgxpool.Pool, opts ...Option) TxRunner {
	metricsPool.Store(pool)
	r := &runner{pool: pool, logger: slog.Default()}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Outcomes of the retry of DD-34, the values of the label `outcome` of set_role_retry_total and of the log.
const (
	// RetryRecovered: the second SET LOCAL ROLE was accepted; the stale role list was the cause.
	RetryRecovered = "recovered"
	// RetryFailed: the second attempt was refused too, or the company could not be entered for another reason.
	RetryFailed = "failed"
)

var (
	retriesRecovered atomic.Int64
	retriesFailed    atomic.Int64
)

// SetRoleRetryCount is the number of entries into a company that were retried after PostgreSQL refused the
// first SET LOCAL ROLE (see InTenantTx), by outcome (RetryRecovered or RetryFailed), since the process
// started. It is the value of set_role_retry_total{outcome}, exported with expvar in T-B904: a value that
// grows in production means the stale role-list race of PostgreSQL is happening and the first defence
// (the pg_auth_members read in setRole) is not enough. An unknown outcome counts 0.
func SetRoleRetryCount(outcome string) int64 {
	switch outcome {
	case RetryRecovered:
		return retriesRecovered.Load()
	case RetryFailed:
		return retriesFailed.Load()
	}
	return 0
}

func (r *runner) InTenantTx(ctx context.Context, tenantID uuid.UUID, fn func(ctx context.Context, tx Tx) error) error {
	enter := func(ctx context.Context, t *tx) error { return t.AsTenant(ctx, tenantID) }
	fnStarted, err := r.run(ctx, enter, fn)
	if err == nil || fnStarted || !isSetRoleDenied(err) {
		return err
	}
	// Second line of defence against the stale role list (see setRole): PostgreSQL refused the very first
	// SET LOCAL ROLE of the transaction, so fn has not done anything and the whole transaction, with its
	// BEGIN, is started again, once. Never after fn ran, never for system roles, never for AsTenant in the
	// middle of a transaction (there fn may already have written).
	fnStarted, err = r.run(ctx, enter, fn)
	if !fnStarted && errors.Is(err, ErrCanceled) {
		return err // the client left between the attempts: the outcome of the retry is unknown, nothing to report
	}
	outcome := RetryRecovered
	counter := &retriesRecovered
	if !fnStarted && err != nil { // the second entry was refused too (or failed otherwise): fn never ran
		outcome, counter = RetryFailed, &retriesFailed
	}
	counter.Add(1)
	r.logger.WarnContext(ctx, "retried the entry into a company after SET ROLE was refused",
		"event", "set_role_retry", "tenant_id", tenantID.String(), "outcome", outcome)
	return err
}

func (r *runner) InSystemTx(ctx context.Context, role SystemRole, fn func(ctx context.Context, tx Tx) error) error {
	_, err := r.run(ctx, func(ctx context.Context, t *tx) error { return t.AsSystem(ctx, role) }, fn)
	return err
}

// isSetRoleDenied recognises PostgreSQL's refusal of SET ROLE: SQLSTATE 42501 "permission denied to set role".
func isSetRoleDenied(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501" && strings.HasPrefix(pgErr.Message, "permission denied to set role")
}

// run executes one transaction. fnStarted reports whether fn was called, so callers know whether
// starting over is safe.
func (r *runner) run(ctx context.Context, enter func(context.Context, *tx) error, fn func(context.Context, Tx) error) (fnStarted bool, err error) {
	pt, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("db: begin: %w", MapError(err))
	}
	t := &tx{inner: pt, fault: r.fault, logger: r.logger}
	committed := false
	defer func() {
		if committed {
			return
		}
		// Also runs while a panic unwinds: roll back, then let the panic continue. The caller's
		// context may be the reason we are here, so the ROLLBACK gets its own.
		rbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		_ = pt.Rollback(rbCtx)
	}()

	if err := enter(ctx, t); err != nil {
		return false, err
	}
	if err := fn(ctx, t); err != nil {
		return true, err
	}
	if err := pt.Commit(ctx); err != nil {
		return true, fmt.Errorf("db: commit: %w", MapError(err))
	}
	committed = true
	return true, nil
}

// tx is what business code receives. It holds the pgx transaction in a private field and delegates
// only queries: embedding pgx.Tx would let fn COMMIT, ROLLBACK, open nested transactions or take the
// connection, escaping the runner (INV-03).
type tx struct {
	inner  pgx.Tx
	logger *slog.Logger
	tenant uuid.UUID
	bound  bool
	fault  func(role string) error // tests only
}

func (t *tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.inner.Exec(ctx, sql, args...)
}

func (t *tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.inner.Query(ctx, sql, args...)
}

func (t *tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.inner.QueryRow(ctx, sql, args...)
}

func (t *tx) TenantID() (uuid.UUID, bool) { return t.tenant, t.bound }

func (t *tx) AsTenant(ctx context.Context, tenantID uuid.UUID) error {
	if t.bound && t.tenant != tenantID {
		return ErrTenantAlreadyBound
	}
	role := TenantRoleName(tenantID)
	if err := t.setRole(ctx, role); err != nil {
		// MapError so that "permission denied to set role" is ErrPrivilege (INV-19); the message keeps the step
		// and the role (INV-27: it tells a refused role switch from an RLS violation inside a query). The hint
		// only fits a role that does not exist (22023 in PostgreSQL 18); with 42501 the role exists.
		hint := ""
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22023" {
			hint = " (was the company provisioned?)"
			if t.logger != nil {
				t.logger.ErrorContext(ctx, "company role missing", "security_event", "privilege_error", "step", "set_role", "role", role, "sqlstate", pgErr.Code)
			}
		}
		return MapError(fmt.Errorf("set role %s%s: %w", role, hint, err))
	}
	t.tenant, t.bound = tenantID, true
	return nil
}

func (t *tx) AsSystem(ctx context.Context, role SystemRole) error {
	switch role {
	case RoleAuth, RoleWorker, RoleSignup:
	default:
		return fmt.Errorf("db: %q is not a system role", role)
	}
	if err := t.setRole(ctx, string(role)); err != nil {
		return MapError(fmt.Errorf("set role %s: %w", role, err))
	}
	return nil
}

// setRole is the single place that changes a transaction's role (INV-03). SET LOCAL lasts until the
// end of the transaction; the identifier is quoted, and comes from a UUID or a closed set anyway.
//
// The SET is preceded by a read of pg_auth_members, in the same round trip. Without it, a backend that
// has not used SET ROLE since another connection created and granted a new company role can answer
// "permission denied to set role" for a role that exists and was granted: the per-backend list of
// SET-able roles (roles_is_member_of in PostgreSQL's acl.c) stays stale, and is rebuilt only when the
// next role invalidation arrives. The read makes the backend refresh its role membership state first.
// Measured on PostgreSQL 18.6 with 2000 roles and 16 concurrent registrations: about 1.4% of first
// uses failed without it and none in 4800 attempts with it
// (TestTxRunner_NewCompanyIsUsableOnAnyConnectionRightAfterProvisioning reproduces it in seconds).
// pg_auth_members is readable by everybody; LIMIT 0 keeps it free.
func (t *tx) setRole(ctx context.Context, role string) error {
	if t.fault != nil {
		if err := t.fault(role); err != nil {
			return err
		}
	}
	_, err := t.Exec(ctx, "SELECT 1 FROM pg_catalog.pg_auth_members LIMIT 0; SET LOCAL ROLE "+pgx.Identifier{role}.Sanitize())
	return err
}
