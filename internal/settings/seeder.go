package settings

import (
	"context"
	"fmt"

	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// TemplateSeeder satisfies industrytemplate.Seeder: it gives a company registering with a template its initial
// configuration (002/DD-3, INV-10). It receives the transaction of the registration, already switched to the role
// of the new company; it opens no transaction, audits nothing and does no network I/O.
type TemplateSeeder struct{ clock clock.Clock }

// NewTemplateSeeder returns the seeder that stamps every row it creates with c.Now().
func NewTemplateSeeder(c clock.Clock) TemplateSeeder { return TemplateSeeder{clock: c} }

var _ industrytemplate.Seeder = TemplateSeeder{}

// Seed inserts the configuration of t for the company tenantID with exactly four statements, whatever the size of
// the template: the settings row, the stages, the categories (the system ones first, then the template's) and the
// sample catalog items. It does not validate t: the catalog is build-time data checked by a test (002/INV-9).
// Any statement error is returned wrapped, so the registration rolls back whole.
func (s TemplateSeeder) Seed(ctx context.Context, tx db.Tx, tenantID uuid.UUID, t industrytemplate.Template) error {
	q := store.New(tx)
	now := s.clock.Now()

	terms := pgtype.Text{}
	if t.QuoteDefaults.Terms != "" {
		terms = pgtype.Text{String: t.QuoteDefaults.Terms, Valid: true}
	}
	if err := q.SeedTenantSettings(ctx, store.SeedTenantSettingsParams{
		TenantID:              tenantID,
		ProjectLabelSingular:  t.Labels.Project.Singular,
		ProjectLabelPlural:    t.Labels.Project.Plural,
		ProjectLabelGender:    t.Labels.Project.Gender,
		CustomerLabelSingular: t.Labels.Customer.Singular,
		CustomerLabelPlural:   t.Labels.Customer.Plural,
		CustomerLabelGender:   t.Labels.Customer.Gender,
		QuoteLabelSingular:    t.Labels.Quote.Singular,
		QuoteLabelPlural:      t.Labels.Quote.Plural,
		QuoteLabelGender:      t.Labels.Quote.Gender,
		QuoteValidityDays:     int32(t.QuoteDefaults.ValidityDays),
		QuoteTerms:            terms,
		Now:                   now,
	}); err != nil {
		return fmt.Errorf("settings: seed tenant settings: %w", db.MapError(err))
	}

	stages := store.SeedStagesParams{TenantID: tenantID, Now: now}
	for _, st := range t.Stages {
		stages.Names = append(stages.Names, st.Name)
		stages.Types = append(stages.Types, st.Type)
		stages.Colors = append(stages.Colors, st.Color)
	}
	if err := q.SeedStages(ctx, stages); err != nil {
		return fmt.Errorf("settings: seed stages: %w", db.MapError(err))
	}

	categories := store.SeedCategoriesParams{TenantID: tenantID, Now: now}
	for _, c := range systemCategories {
		categories.Names = append(categories.Names, c.Name)
		categories.Types = append(categories.Types, string(c.Type))
		categories.SystemKeys = append(categories.SystemKeys, string(c.Key))
	}
	for _, c := range t.Categories {
		categories.Names = append(categories.Names, c.Name)
		categories.Types = append(categories.Types, c.Type)
		categories.SystemKeys = append(categories.SystemKeys, "")
	}
	if err := q.SeedCategories(ctx, categories); err != nil {
		return fmt.Errorf("settings: seed categories: %w", db.MapError(err))
	}

	items := store.SeedCatalogItemsParams{TenantID: tenantID, Now: now}
	for _, it := range t.CatalogItems {
		items.Names = append(items.Names, it.Name)
		items.Descriptions = append(items.Descriptions, it.Description)
		items.Units = append(items.Units, it.Unit)
		items.UnitLabels = append(items.UnitLabels, it.UnitLabel)
	}
	if err := q.SeedCatalogItems(ctx, items); err != nil {
		return fmt.Errorf("settings: seed catalog items: %w", db.MapError(err))
	}
	return nil
}
