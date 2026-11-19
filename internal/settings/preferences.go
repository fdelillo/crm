package settings

import (
	"context"
	"fmt"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings/store"
	"github.com/jackc/pgx/v5/pgtype"
)

// Gender is the grammatical gender of a custom name: "masculine" or "feminine" (P-3).
type Gender string

const (
	genderMasculine Gender = "masculine"
	genderFeminine  Gender = "feminine"
)

var genders = []Gender{genderMasculine, genderFeminine}

// EntityLabel is the singular, plural and gender of a custom name.
type EntityLabel struct {
	Singular string
	Plural   string
	Gender   Gender
}

// Labels holds the three custom names of the company.
type Labels struct{ Project, Customer, Quote EntityLabel }

// QuoteDefaults are the default commercial conditions of a quote.
type QuoteDefaults struct {
	ValidityDays int     // 1..365
	Terms        *string // nil = no terms; free text
}

func validateGender(g Gender) string { return validateEnum(g, genders) }

// normalizeLabels validates and normalizes the three names, returning every field error at once. The
// keys are the contract paths ("project.singular").
// T-002-B302 (002/DD-10, P-3): texts up to 40 characters, gender masculine or feminine.
func normalizeLabels(l Labels) (Labels, FieldErrors) {
	errs := FieldErrors{}
	normalize := func(prefix string, in EntityLabel) EntityLabel {
		out := EntityLabel{Gender: in.Gender}
		var code string
		out.Singular, code = normalizeRequiredText(in.Singular, maxEntityLabelLength)
		errs.add(prefix+".singular", code)
		out.Plural, code = normalizeRequiredText(in.Plural, maxEntityLabelLength)
		errs.add(prefix+".plural", code)
		if in.Gender == "" {
			errs.add(prefix+".gender", codeRequired)
		} else {
			errs.add(prefix+".gender", validateGender(in.Gender))
		}
		return out
	}
	out := Labels{
		Project:  normalize("project", l.Project),
		Customer: normalize("customer", l.Customer),
		Quote:    normalize("quote", l.Quote),
	}
	if len(errs) > 0 {
		return Labels{}, errs
	}
	return out, nil
}

// normalizeQuoteDefaults validates validity_days (1..365) and terms (optional, up to 2000 characters).
// T-002-B302 (002/DD-10, P-10): empty or only spaces terms become nil.
func normalizeQuoteDefaults(q QuoteDefaults) (QuoteDefaults, FieldErrors) {
	errs := FieldErrors{}
	if q.ValidityDays < minQuoteValidityDays || q.ValidityDays > maxQuoteValidityDays {
		errs.add("validity_days", codeInvalidValue)
	}
	out := QuoteDefaults{ValidityDays: q.ValidityDays}
	if q.Terms != nil {
		terms, code := normalizeOptionalText(*q.Terms, maxQuoteTermsLength)
		errs.add("terms", code)
		out.Terms = terms
	}
	if len(errs) > 0 {
		return QuoteDefaults{}, errs
	}
	return out, nil
}

// GetLabels returns the custom names of the company. Any role may read (002/DD-11). A company without its settings
// row is ErrTenantSettingsMissing.
// T-002-B702 (002/DD-10, DD-19)
func (s *Service) GetLabels(ctx context.Context, p authz.Principal) (Labels, error) {
	var out Labels
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		row, err := readTenantSettings(ctx, store.New(tx), p.TenantID)
		out = labelsFromRow(row)
		return err
	})
	if err != nil {
		return Labels{}, err
	}
	return out, nil
}

// ReplaceLabels saves the three custom names and returns them normalized. Invalid fields are FieldErrors with the
// contract paths ("project.singular"). The audit lists the entities that changed; the current values write and audit
// nothing.
// T-002-B702 (002/DD-10, DD-15, DD-19, INV-13)
func (s *Service) ReplaceLabels(ctx context.Context, p authz.Principal, in Labels, meta RequestMeta) (Labels, error) {
	if err := requireSettingsManage(p); err != nil {
		return Labels{}, err
	}
	labels, errs := normalizeLabels(in)
	if errs != nil {
		return Labels{}, errs
	}
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		if err := lockTenantSettings(ctx, q, p.TenantID); err != nil {
			return err
		}
		// The instant is read under the lock, as in writeStages (002/DD-5).
		now := s.clock.Now()
		row, err := readTenantSettings(ctx, q, p.TenantID)
		if err != nil {
			return err
		}
		current := labelsFromRow(row)
		var changed []string
		for _, e := range []struct {
			name      string
			cur, next EntityLabel
		}{{"project", current.Project, labels.Project}, {"customer", current.Customer, labels.Customer}, {"quote", current.Quote, labels.Quote}} {
			if e.cur != e.next {
				changed = append(changed, e.name)
			}
		}
		if len(changed) == 0 {
			return nil
		}
		err = q.UpdateLabels(ctx, store.UpdateLabelsParams{TenantID: p.TenantID, Now: now,
			ProjectSingular: labels.Project.Singular, ProjectPlural: labels.Project.Plural, ProjectGender: string(labels.Project.Gender),
			CustomerSingular: labels.Customer.Singular, CustomerPlural: labels.Customer.Plural, CustomerGender: string(labels.Customer.Gender),
			QuoteSingular: labels.Quote.Singular, QuotePlural: labels.Quote.Plural, QuoteGender: string(labels.Quote.Gender)})
		if err != nil {
			return fmt.Errorf("settings: update labels: %w", db.MapError(err))
		}
		return s.recordTenantSettings(ctx, tx, p, meta, "settings.labels_updated", map[string]any{"entities": changed})
	})
	if err != nil {
		return Labels{}, err
	}
	return labels, nil
}

// GetQuoteDefaults returns the default validity and terms of a quote. Any role may read (002/DD-11).
// T-002-B702 (002/DD-10, DD-19)
func (s *Service) GetQuoteDefaults(ctx context.Context, p authz.Principal) (QuoteDefaults, error) {
	var out QuoteDefaults
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		row, err := readTenantSettings(ctx, store.New(tx), p.TenantID)
		out = quoteDefaultsFromRow(row)
		return err
	})
	if err != nil {
		return QuoteDefaults{}, err
	}
	return out, nil
}

// ReplaceQuoteDefaults saves the default validity and terms and returns them normalized. validity_days outside
// 1..365 is FieldErrors{validity_days}; terms with only spaces become nil (the text is never interpreted). The audit
// lists the fields that changed; the current values write and audit nothing.
// T-002-B702 (002/DD-10, DD-15, DD-19, P-10, INV-13)
func (s *Service) ReplaceQuoteDefaults(ctx context.Context, p authz.Principal, in QuoteDefaults, meta RequestMeta) (QuoteDefaults, error) {
	if err := requireSettingsManage(p); err != nil {
		return QuoteDefaults{}, err
	}
	defaults, errs := normalizeQuoteDefaults(in)
	if errs != nil {
		return QuoteDefaults{}, errs
	}
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		if err := lockTenantSettings(ctx, q, p.TenantID); err != nil {
			return err
		}
		now := s.clock.Now()
		row, err := readTenantSettings(ctx, q, p.TenantID)
		if err != nil {
			return err
		}
		current := quoteDefaultsFromRow(row)
		var changed []string
		if current.ValidityDays != defaults.ValidityDays {
			changed = append(changed, "validity_days")
		}
		if !sameTerms(current.Terms, defaults.Terms) {
			changed = append(changed, "terms")
		}
		if len(changed) == 0 {
			return nil
		}
		var terms pgtype.Text
		if defaults.Terms != nil {
			terms = pgtype.Text{String: *defaults.Terms, Valid: true}
		}
		err = q.UpdateQuoteDefaults(ctx, store.UpdateQuoteDefaultsParams{TenantID: p.TenantID, ValidityDays: int32(defaults.ValidityDays), Terms: terms, Now: now})
		if err != nil {
			return fmt.Errorf("settings: update quote defaults: %w", db.MapError(err))
		}
		return s.recordTenantSettings(ctx, tx, p, meta, "settings.quote_defaults_updated", map[string]any{"fields": changed})
	})
	if err != nil {
		return QuoteDefaults{}, err
	}
	return defaults, nil
}

func (s *Service) recordTenantSettings(ctx context.Context, tx db.Tx, p authz.Principal, meta RequestMeta, action string, data map[string]any) error {
	return s.audit.Record(ctx, tx, audit.Entry{TenantID: p.TenantID, ActorUserID: &p.UserID, Action: action,
		TargetType: "tenant", TargetID: &p.TenantID, Data: data, IP: meta.IP, UserAgent: meta.UserAgent})
}

func labelsFromRow(r store.AppTenantSetting) Labels {
	return Labels{
		Project:  EntityLabel{Singular: r.ProjectLabelSingular, Plural: r.ProjectLabelPlural, Gender: Gender(r.ProjectLabelGender)},
		Customer: EntityLabel{Singular: r.CustomerLabelSingular, Plural: r.CustomerLabelPlural, Gender: Gender(r.CustomerLabelGender)},
		Quote:    EntityLabel{Singular: r.QuoteLabelSingular, Plural: r.QuoteLabelPlural, Gender: Gender(r.QuoteLabelGender)},
	}
}

func quoteDefaultsFromRow(r store.AppTenantSetting) QuoteDefaults {
	out := QuoteDefaults{ValidityDays: int(r.QuoteValidityDays)}
	if r.QuoteTerms.Valid {
		terms := r.QuoteTerms.String
		out.Terms = &terms
	}
	return out
}

func sameTerms(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
