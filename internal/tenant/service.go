package tenant

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant/store"
	"github.com/google/uuid"
)

const (
	SignupLockTimeout = 2 * time.Second
	defaultTimezone   = "America/Argentina/Buenos_Aires"
)

type Signup struct {
	Name                 string `json:"name"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	CompanyName          string `json:"company_name"`
	BaseCurrency         string `json:"base_currency"`
	IndustryTemplateCode string `json:"industry_template_code"`
	Timezone             string `json:"timezone"`
}

type Summary struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	BaseCurrency string    `json:"base_currency"`
	Timezone     string    `json:"timezone"`
	HasLogo      bool      `json:"has_logo"`
}

type Registration struct {
	Tenant  Summary
	User    identity.CurrentUser
	Session identity.SessionResult
}

// FieldErrors are stable contract codes. They never include submitted values.
type FieldErrors map[string]string

func (e FieldErrors) Error() string { return "tenant: invalid registration fields" }

type AdminOnboarding interface {
	CreateFirstAdmin(ctx context.Context, tx db.Tx, in identity.NewAdmin) (uuid.UUID, error)
	CreateSession(ctx context.Context, tx db.Tx, p authz.Principal, meta identity.RequestMeta) (identity.SessionResult, error)
	IssueEmailVerification(ctx context.Context, tx db.Tx, tenantID, userID uuid.UUID) error
}

type Service struct {
	runner db.TxRunner
	admins AdminOnboarding
	seeder industrytemplate.Seeder
	hasher password.Hasher
	audit  audit.Recorder
	logger *slog.Logger
}

func NewService(runner db.TxRunner, admins AdminOnboarding, seeder industrytemplate.Seeder,
	hasher password.Hasher, recorder audit.Recorder, logger *slog.Logger) *Service {
	if runner == nil || admins == nil || seeder == nil || hasher == nil || recorder == nil || logger == nil {
		panic("tenant: invalid service dependencies")
	}
	return &Service{runner: runner, admins: admins, seeder: seeder, hasher: hasher, audit: recorder, logger: logger}
}

func (s *Service) Register(ctx context.Context, in Signup, meta identity.RequestMeta) (Registration, error) {
	name := strings.TrimSpace(in.Name)
	company := strings.TrimSpace(in.CompanyName)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	fields := FieldErrors{}
	if name == "" || utf8.RuneCountInString(name) > 120 || strings.ContainsRune(name, '\x00') {
		fields["name"] = "invalid_value"
	}
	if company == "" || utf8.RuneCountInString(company) > 120 || strings.ContainsRune(company, '\x00') {
		fields["company_name"] = "invalid_value"
	}
	if len(email) > 254 || email == "" || !validEmail(email) {
		fields["email"] = "invalid_format"
	}
	if code := password.Validate(in.Password, email); code != "" {
		fields["password"] = code
	}
	if in.BaseCurrency != "ARS" && in.BaseCurrency != "USD" {
		fields["base_currency"] = "invalid_value"
	}
	template, templateOK := industrytemplate.Lookup(in.IndustryTemplateCode)
	if !templateOK {
		fields["industry_template_code"] = "unknown_template"
	}
	if len(fields) > 0 {
		return Registration{}, fields
	}
	timezone := in.Timezone
	if len(timezone) > 64 {
		timezone = ""
	}
	if timezone == "" {
		timezone = defaultTimezone
		s.logger.InfoContext(ctx, "signup timezone defaulted", "event", "signup_timezone_defaulted")
	} else if _, err := time.LoadLocation(timezone); err != nil {
		timezone = defaultTimezone
		s.logger.InfoContext(ctx, "signup timezone defaulted", "event", "signup_timezone_defaulted")
	}
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return Registration{}, fmt.Errorf("tenant: hash admin password: %w", err)
	}
	tenantID, err := uuid.NewV7()
	if err != nil {
		return Registration{}, fmt.Errorf("tenant: generate id: %w", err)
	}
	result := Registration{Tenant: Summary{ID: tenantID, Name: company, BaseCurrency: in.BaseCurrency, Timezone: timezone}}
	err = s.runner.InSystemTx(ctx, db.RoleSignup, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		if _, err := q.SetSignupLockTimeout(ctx, SignupLockTimeout.String()); err != nil {
			return fmt.Errorf("tenant: set signup lock timeout: %w", db.MapError(err))
		}
		if _, err := q.ProvisionTenantRole(ctx, tenantID); err != nil {
			return fmt.Errorf("tenant: provision role: %w", db.MapError(err))
		}
		if err := tx.AsTenant(ctx, tenantID); err != nil {
			return err
		}
		if err := q.InsertTenant(ctx, store.InsertTenantParams{TenantID: tenantID, Name: company,
			BaseCurrency: in.BaseCurrency, Timezone: timezone, IndustryTemplateCode: template.Code,
			IndustryTemplateVersion: int32(template.Version)}); err != nil {
			return fmt.Errorf("tenant: insert company: %w", db.MapError(err))
		}
		userID, err := s.admins.CreateFirstAdmin(ctx, tx, identity.NewAdmin{TenantID: tenantID,
			Name: name, Email: email, PasswordHash: hash})
		if err != nil {
			return err
		}
		if err := s.seeder.Seed(ctx, tx, tenantID, template); err != nil {
			return fmt.Errorf("tenant: seed template: %w", err)
		}
		if err := s.admins.IssueEmailVerification(ctx, tx, tenantID, userID); err != nil {
			return err
		}
		result.Session, err = s.admins.CreateSession(ctx, tx,
			authz.Principal{TenantID: tenantID, UserID: userID, Role: authz.RoleAdmin}, meta)
		if err != nil {
			return err
		}
		if err := s.audit.Record(ctx, tx, audit.Entry{TenantID: tenantID, ActorUserID: &userID,
			Action: "tenant.registered", TargetType: "tenant", TargetID: &tenantID,
			Data: map[string]any{"industry_template_code": template.Code}, IP: meta.IP,
			UserAgent: meta.UserAgent}); err != nil {
			return fmt.Errorf("tenant: audit registration: %w", err)
		}
		result.User = identity.CurrentUser{ID: userID, Name: name, Email: email, Role: authz.RoleAdmin,
			Status: "active", EmailVerified: false}
		return nil
	})
	return result, err
}

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && strings.Contains(value, "@")
}

// SummaryFor reads the company under its own role for SessionInfo responses.
func (s *Service) SummaryFor(ctx context.Context, tenantID uuid.UUID) (Summary, error) {
	var out Summary
	err := s.runner.InTenantTx(ctx, tenantID, func(ctx context.Context, tx db.Tx) error {
		row, err := store.New(tx).GetTenantSummary(ctx, tenantID)
		if err != nil {
			return db.MapError(err)
		}
		out = Summary{ID: row.ID, Name: row.Name, BaseCurrency: row.BaseCurrency,
			Timezone: row.Timezone, HasLogo: row.LogoObjectKey.Valid}
		return nil
	})
	return out, err
}

func IsValidation(err error) (FieldErrors, bool) {
	var fields FieldErrors
	return fields, errors.As(err, &fields)
}
