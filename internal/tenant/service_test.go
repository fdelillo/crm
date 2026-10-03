package tenant_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/google/uuid"
)

type countingRunner struct{ calls int }

func (r *countingRunner) InTenantTx(context.Context, uuid.UUID, func(context.Context, db.Tx) error) error {
	r.calls++
	return errors.New("unexpected tenant transaction")
}
func (r *countingRunner) InSystemTx(context.Context, db.SystemRole, func(context.Context, db.Tx) error) error {
	r.calls++
	return errors.New("unexpected system transaction")
}

type countingHasher struct{ calls int }

func (h *countingHasher) Hash(context.Context, string) (string, error) {
	h.calls++
	return "", errors.New("unexpected hash")
}
func (*countingHasher) Verify(context.Context, string, string) (bool, bool, error) {
	return false, false, errors.New("unexpected verify")
}
func (*countingHasher) VerifyDummy(context.Context, string) {}

func TestRegisterRejectsInvalidInputBeforeHashOrTransaction(t *testing.T) {
	runner := &countingRunner{}
	hasher := &countingHasher{}
	c := clock.Real{}
	admins := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour)
	svc := tenant.NewService(runner, admins, industrytemplate.NoopSeeder{}, hasher,
		audit.NewRecorder(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	valid := tenant.Signup{Name: "Ana", Email: "ana@example.com", Password: "example-password",
		CompanyName: "Empresa", BaseCurrency: "ARS", IndustryTemplateCode: "generic"}
	for _, tc := range []struct {
		name, field, code string
		change            func(*tenant.Signup)
	}{
		{"short password", "password", "too_short", func(s *tenant.Signup) { s.Password = "shortpass" }},
		{"email as password", "password", "same_as_email", func(s *tenant.Signup) { s.Password = s.Email }},
		{"unknown template", "industry_template_code", "unknown_template", func(s *tenant.Signup) { s.IndustryTemplateCode = "unknown" }},
		{"currency", "base_currency", "invalid_value", func(s *tenant.Signup) { s.BaseCurrency = "EUR" }},
		{"NUL in company name", "company_name", "invalid_value", func(s *tenant.Signup) { s.CompanyName = "Empresa\x00 inválida" }},
		{"NUL in user name", "name", "invalid_value", func(s *tenant.Signup) { s.Name = "Ana\x00 inválida" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := valid
			tc.change(&input)
			_, err := svc.Register(context.Background(), input, identity.RequestMeta{})
			var fields tenant.FieldErrors
			if !errors.As(err, &fields) || fields[tc.field] != tc.code {
				t.Fatalf("validation error=%v; want %s=%s", err, tc.field, tc.code)
			}
		})
	}
	if hasher.calls != 0 || runner.calls != 0 {
		t.Fatalf("invalid requests reached hash=%d transactions=%d", hasher.calls, runner.calls)
	}
}
