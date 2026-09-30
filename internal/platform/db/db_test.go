package db_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// T-B101 (Refactor): the role name is derived in one place.
func TestTenantRoleName(t *testing.T) {
	id := uuid.MustParse("0192f3a4-7b1c-7d2e-8f30-a1b2c3d4e5f6")
	if got, want := db.TenantRoleName(id), "crm_t_0192f3a47b1c7d2e8f30a1b2c3d4e5f6"; got != want {
		t.Errorf("TenantRoleName = %q, want %q", got, want)
	}
	for range 50 {
		got := db.TenantRoleName(uuid.New())
		if len(got) != len("crm_t_")+32 || got != lower(got) {
			t.Fatalf("TenantRoleName = %q, want crm_t_ + 32 lowercase hex digits", got)
		}
	}
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func pgErr(code, constraint string) error {
	return &pgconn.PgError{Code: code, ConstraintName: constraint, Message: "boom"}
}

// T-B105: mapping of PostgreSQL and pgx errors to the package's sentinels (plan §9.2).
func TestMapError(t *testing.T) {
	tests := []struct {
		name string
		in   error
		is   []error // every one must match with errors.Is
		not  []error // none may match
	}{
		{"no rows", pgx.ErrNoRows, []error{db.ErrNotFound}, []error{db.ErrPrivilege, db.ErrUnavailable}},
		{"wrapped no rows", fmt.Errorf("get user: %w", pgx.ErrNoRows), []error{db.ErrNotFound}, nil},
		{"unique violation", pgErr("23505", "users_email_key"), []error{db.ErrUniqueViolation}, []error{db.ErrNotFound}},
		{"insufficient privilege", pgErr("42501", ""), []error{db.ErrPrivilege}, []error{db.ErrNotFound, db.ErrUnavailable}},
		{"statement timeout", pgErr("57014", ""), []error{db.ErrUnavailable}, []error{db.ErrPrivilege}},
		{"admin shutdown", pgErr("57P01", ""), []error{db.ErrUnavailable}, nil},
		{"connection exception class", pgErr("08006", ""), []error{db.ErrUnavailable}, nil},
		{"too many connections", pgErr("53300", ""), []error{db.ErrUnavailable}, nil},
		{"deadline exceeded", context.DeadlineExceeded, []error{db.ErrUnavailable}, nil},
		{"wrapped deadline", fmt.Errorf("query: %w", context.DeadlineExceeded), []error{db.ErrUnavailable}, nil},
		{"connect error", &pgconn.ConnectError{Config: &pgconn.Config{}}, []error{db.ErrUnavailable}, nil},
		{"network error", &net.OpError{Op: "read", Err: errors.New("connection reset by peer")}, []error{db.ErrUnavailable}, nil},
		{"unexpected EOF", io.ErrUnexpectedEOF, []error{db.ErrUnavailable}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := db.MapError(tt.in)
			for _, want := range tt.is {
				if !errors.Is(got, want) {
					t.Errorf("MapError(%v) = %v; errors.Is(%v) = false", tt.in, got, want)
				}
			}
			for _, not := range tt.not {
				if errors.Is(got, not) {
					t.Errorf("MapError(%v) = %v; must not be %v", tt.in, got, not)
				}
			}
		})
	}
}

func TestMapError_UniqueViolationCarriesTheConstraint(t *testing.T) {
	var ce *db.ConstraintError
	if err := db.MapError(pgErr("23505", "users_email_key")); !errors.As(err, &ce) {
		t.Fatalf("MapError = %v, want a *db.ConstraintError", err)
	}
	if ce.Constraint != "users_email_key" || !errors.Is(ce.Err, db.ErrUniqueViolation) {
		t.Errorf("ConstraintError = %+v, want constraint users_email_key and ErrUniqueViolation", ce)
	}
}

// Anything else is returned wrapped with its cause and no classification; nil stays nil.
func TestMapError_Unclassified(t *testing.T) {
	if db.MapError(nil) != nil {
		t.Error("MapError(nil) != nil")
	}
	cause := pgErr("22012", "") // division by zero
	got := db.MapError(cause)
	if got == nil || !errors.Is(got, cause) {
		t.Fatalf("MapError = %v, want an error that wraps the cause", got)
	}
	for _, s := range []error{db.ErrNotFound, db.ErrUniqueViolation, db.ErrPrivilege, db.ErrUnavailable, db.ErrTenantAlreadyBound} {
		if errors.Is(got, s) {
			t.Errorf("unclassified error matches %v", s)
		}
	}
	plain := errors.New("something else")
	if got := db.MapError(plain); !errors.Is(got, plain) {
		t.Errorf("MapError(plain) = %v, want it to wrap the cause", got)
	}
}

// Mapping twice must not change the classification (a service may map what a helper already mapped).
func TestMapError_IsIdempotent(t *testing.T) {
	once := db.MapError(pgErr("23505", "users_email_key"))
	twice := db.MapError(once)
	var ce *db.ConstraintError
	if !errors.As(twice, &ce) || ce.Constraint != "users_email_key" {
		t.Errorf("second MapError lost the constraint: %v", twice)
	}
	if !errors.Is(db.MapError(db.MapError(pgx.ErrNoRows)), db.ErrNotFound) {
		t.Error("second MapError lost ErrNotFound")
	}
}
