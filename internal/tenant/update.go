package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/tenant/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Tenant struct {
	ID                   uuid.UUID `json:"id"`
	Name                 string    `json:"name"`
	LegalName            *string   `json:"legal_name"`
	TaxID                *string   `json:"tax_id"`
	Address              *string   `json:"address"`
	Phone                *string   `json:"phone"`
	Email                *string   `json:"email"`
	HasLogo              bool      `json:"has_logo"`
	BaseCurrency         string    `json:"base_currency"`
	Timezone             string    `json:"timezone"`
	IndustryTemplateCode string    `json:"industry_template_code"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Update preserves absence (no map entry) independently of an explicit JSON null.
// Its custom decoder keeps unknown fields malformed even when used outside HTTP.
type Update map[string]*string

var ErrMalformedUpdate = errors.New("tenant: malformed update")

var updateLimits = map[string]int{"name": 120, "legal_name": 200, "tax_id": 13, "address": 300, "phone": 50, "email": 254, "timezone": 64}

func (u *Update) UnmarshalJSON(data []byte) error {
	type plain Update
	var fields plain
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformedUpdate, err)
	}
	if err := Update(fields).checkFields(); err != nil {
		return err
	}
	*u = Update(fields)
	return nil
}
func (u Update) checkFields() error {
	if len(u) == 0 {
		return ErrMalformedUpdate
	}
	for key, value := range u {
		if value == nil && (key == "name" || key == "timezone") {
			return ErrMalformedUpdate
		}
		if _, ok := updateLimits[key]; !ok {
			return ErrMalformedUpdate
		}
	}
	return nil
}
func (u Update) validate() (Update, error) {
	if err := u.checkFields(); err != nil {
		return nil, err
	}
	normalized := make(Update, len(u))
	fields := FieldErrors{}
	for name, value := range u {
		normalized[name] = value
		if value == nil {
			continue
		}
		raw := *value
		if !validText(raw, updateLimits[name]) {
			switch name {
			case "timezone":
				fields[name] = "invalid_timezone"
			case "email", "tax_id":
				fields[name] = "invalid_format"
			case "name":
				fields[name] = validateName(raw)
			default:
				fields[name] = "invalid_value"
			}
			continue
		}
		v := strings.TrimSpace(raw)
		if v == "" && name != "name" && name != "timezone" {
			normalized[name] = nil
			continue
		}
		switch name {
		case "name":
			if code := validateName(raw); code != "" {
				fields[name] = code
			}
		case "timezone":
			if code := validateTimezone(v); code != "" {
				fields[name] = code
			}
		case "tax_id":
			var code string
			v, code = normalizeTaxID(v)
			if code != "" {
				fields[name] = code
			}
		case "email":
			if !validEmail(v) {
				fields[name] = "invalid_format"
			}
		}

		normalized[name] = &v
	}
	if len(fields) != 0 {
		return nil, fields
	}
	return normalized, nil
}
func textPointer(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
func nullableText(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}
func tenantFrom(row store.AppTenant) Tenant {
	return Tenant{ID: row.ID, Name: row.Name, LegalName: textPointer(row.LegalName), TaxID: textPointer(row.TaxID), Address: textPointer(row.Address), Phone: textPointer(row.Phone), Email: textPointer(row.Email), HasLogo: row.LogoObjectKey.Valid, BaseCurrency: row.BaseCurrency, Timezone: row.Timezone, IndustryTemplateCode: row.IndustryTemplateCode, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
func (s *Service) Get(ctx context.Context, p authz.Principal) (Tenant, error) {
	var out Tenant
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		row, err := store.New(tx).GetTenant(ctx, p.TenantID)
		if err != nil {
			return db.MapError(err)
		}
		out = tenantFrom(row)
		return nil
	})
	return out, err
}
func (s *Service) Update(ctx context.Context, p authz.Principal, input Update, meta identity.RequestMeta) (Tenant, error) {
	if !authz.Can(p.Role, authz.SettingsManage) {
		return Tenant{}, authz.ErrForbidden
	}
	input, err := input.validate()
	if err != nil {
		return Tenant{}, err
	}
	var out Tenant
	err = s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		row, err := q.LockTenant(ctx, p.TenantID)
		if err != nil {
			return db.MapError(err)
		}
		current := tenantFrom(row)
		values := map[string]*string{"name": &current.Name, "legal_name": current.LegalName, "tax_id": current.TaxID, "address": current.Address, "phone": current.Phone, "email": current.Email, "timezone": &current.Timezone}
		changed := make([]string, 0, len(input))
		// Audit order follows data-model §2.1, independently of JSON/map ordering.
		for _, key := range []string{"name", "legal_name", "tax_id", "address", "phone", "email", "timezone"} {
			v, present := input[key]
			if !present {
				continue
			}
			old := values[key]
			if (old == nil) != (v == nil) || old != nil && v != nil && *old != *v {
				changed = append(changed, key)
			}
			values[key] = v
		}
		if len(changed) == 0 {
			out = current
			return nil
		}

		row, err = q.UpdateTenantDetails(ctx, store.UpdateTenantDetailsParams{TenantID: p.TenantID, Name: *values["name"], LegalName: nullableText(values["legal_name"]), TaxID: nullableText(values["tax_id"]), Address: nullableText(values["address"]), Phone: nullableText(values["phone"]), Email: nullableText(values["email"]), Timezone: *values["timezone"], UpdatedAt: time.Now().UTC()})
		if err != nil {
			return db.MapError(err)
		}
		if err = s.recordChange(ctx, tx, p, "tenant.updated", map[string]any{"fields": changed}, meta); err != nil {
			return err
		}
		out = tenantFrom(row)
		return nil
	})
	return out, err
}
func (s *Service) recordChange(ctx context.Context, tx db.Tx, p authz.Principal, action string, data map[string]any, meta identity.RequestMeta) error {
	return s.audit.Record(ctx, tx, audit.Entry{TenantID: p.TenantID, ActorUserID: &p.UserID, Action: action, TargetType: "tenant", TargetID: &p.TenantID, Data: data, IP: meta.IP, UserAgent: meta.UserAgent})
}
