package db

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel errors. Services and handlers compare them with errors.Is; the original pgx error stays
// in the chain so logs keep the SQLSTATE and the constraint.
var (
	ErrNotFound           = errors.New("db: not found")
	ErrUniqueViolation    = errors.New("db: unique violation") // wrapped in *ConstraintError
	ErrPrivilege          = errors.New("db: insufficient privilege or RLS violation")
	ErrUnavailable        = errors.New("db: unavailable")
	ErrTenantAlreadyBound = errors.New("db: transaction already bound to another tenant")
)

// ConstraintError is a violated constraint, by name, so callers can tell which one (for example
// users_email_key → 409 email_already_registered). Err is the classification (ErrUniqueViolation).
type ConstraintError struct {
	Constraint string
	Err        error
}

func (e *ConstraintError) Error() string {
	return fmt.Sprintf("%v (constraint %q)", e.Err, e.Constraint)
}

func (e *ConstraintError) Unwrap() error { return e.Err }

// MapError classifies an error coming from pgx or PostgreSQL (plan §9.2):
//
//	pgx.ErrNoRows                          → ErrNotFound
//	23505 unique_violation                 → *ConstraintError{Err: ErrUniqueViolation}
//	42501 insufficient_privilege (RLS)     → ErrPrivilege (a bug: never a 403 or 404, INV-19)
//	connection errors, 08xxx, 57014 (statement timeout), 57P01..03, 53300,
//	deadline exceeded, network errors      → ErrUnavailable
//
// Anything else is wrapped without classification. It is idempotent and MapError(nil) is nil.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	var ce *ConstraintError
	for _, s := range []error{ErrNotFound, ErrPrivilege, ErrUnavailable, ErrTenantAlreadyBound} {
		if errors.Is(err, s) {
			return err
		}
	}
	if errors.As(err, &ce) {
		return err
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505":
			return &ConstraintError{Constraint: pgErr.ConstraintName, Err: ErrUniqueViolation}
		case pgErr.Code == "42501":
			return fmt.Errorf("%w: %w", ErrPrivilege, err)
		case unavailableCode(pgErr.Code):
			return fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		return fmt.Errorf("db: %w", err)
	}
	if isConnectionProblem(err) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return fmt.Errorf("db: %w", err)
}

func unavailableCode(code string) bool {
	switch code {
	case "57014", "57P01", "57P02", "57P03", "53300":
		return true
	}
	return strings.HasPrefix(code, "08") // connection_exception class
}

func isConnectionProblem(err error) bool {
	var connectErr *pgconn.ConnectError
	var netErr net.Error
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.As(err, &connectErr) || errors.As(err, &netErr) ||
		pgconn.Timeout(err)
}
