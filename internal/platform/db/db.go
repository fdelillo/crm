package db

import (
	"context"
	"encoding/hex"
	"fmt"
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
	// back if fn returns an error or panics (the panic continues).
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

type runner struct{ pool *pgxpool.Pool }

// NewTxRunner returns the TxRunner over a pool connected as crm_app.
func NewTxRunner(pool *pgxpool.Pool) TxRunner { return &runner{pool: pool} }

func (r *runner) InTenantTx(ctx context.Context, tenantID uuid.UUID, fn func(ctx context.Context, tx Tx) error) error {
	return r.run(ctx, func(ctx context.Context, t *tx) error { return t.AsTenant(ctx, tenantID) }, fn)
}

func (r *runner) InSystemTx(ctx context.Context, role SystemRole, fn func(ctx context.Context, tx Tx) error) error {
	return r.run(ctx, func(ctx context.Context, t *tx) error { return t.AsSystem(ctx, role) }, fn)
}

func (r *runner) run(ctx context.Context, enter func(context.Context, *tx) error, fn func(context.Context, Tx) error) error {
	pt, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin: %w", MapError(err))
	}
	t := &tx{Tx: pt}
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
		return err
	}
	if err := fn(ctx, t); err != nil {
		return err
	}
	if err := pt.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", MapError(err))
	}
	committed = true
	return nil
}

type tx struct {
	pgx.Tx
	tenant uuid.UUID
	bound  bool
}

func (t *tx) TenantID() (uuid.UUID, bool) { return t.tenant, t.bound }

func (t *tx) AsTenant(ctx context.Context, tenantID uuid.UUID) error {
	if t.bound && t.tenant != tenantID {
		return ErrTenantAlreadyBound
	}
	role := TenantRoleName(tenantID)
	if err := t.setRole(ctx, role); err != nil {
		return fmt.Errorf("db: switching to company role %s (was the company provisioned?): %w", role, err)
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
		return fmt.Errorf("db: switching to system role %s: %w", role, err)
	}
	return nil
}

// setRole is the single place that changes a transaction's role (INV-03). SET LOCAL lasts until the
// end of the transaction; the identifier is quoted, and comes from a UUID or a closed set anyway.
func (t *tx) setRole(ctx context.Context, role string) error {
	_, err := t.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{role}.Sanitize())
	return err
}
