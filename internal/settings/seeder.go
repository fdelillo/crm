package settings

import (
	"context"

	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/google/uuid"
)

// TemplateSeeder satisfies industrytemplate.Seeder (002/DD-3, INV-10).
type TemplateSeeder struct{ clock clock.Clock }

// NewTemplateSeeder returns the seeder that stamps every row it creates with c.Now().
func NewTemplateSeeder(c clock.Clock) TemplateSeeder { return TemplateSeeder{clock: c} }

var _ industrytemplate.Seeder = TemplateSeeder{}

// Seed is a stub until T-002-B306.
func (TemplateSeeder) Seed(context.Context, db.Tx, uuid.UUID, industrytemplate.Template) error {
	return nil
}
