package industrytemplate

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/google/uuid"
)

// Template is a versioned, business-independent initial company configuration: the names, pipeline,
// categories, sample catalog and commercial conditions a company starts with (002/DD-2).
// Enumerated values are plain strings: settings validates them with its own rules (002/INV-9).
type Template struct {
	Code          string                `json:"code"`
	Name          string                `json:"name"`
	Version       int                   `json:"version"`
	Labels        TemplateLabels        `json:"labels"`
	Stages        []TemplateStage       `json:"stages"`
	Categories    []TemplateCategory    `json:"categories"` // the company's own; settings adds the system ones
	CatalogItems  []TemplateCatalogItem `json:"catalog_items"`
	QuoteDefaults TemplateQuoteDefaults `json:"quote_defaults"`
}

// TemplateLabel is the singular, plural and gender ("masculine" or "feminine") of one custom name.
type TemplateLabel struct {
	Singular string `json:"singular"`
	Plural   string `json:"plural"`
	Gender   string `json:"gender"`
}

// TemplateLabels holds the custom names of the three entities.
type TemplateLabels struct {
	Project  TemplateLabel `json:"project"`
	Customer TemplateLabel `json:"customer"`
	Quote    TemplateLabel `json:"quote"`
}

// TemplateStage is a pipeline stage; the template lists them in order. Type is "open", "won" or
// "lost"; Color is one of the palette names of settings.
type TemplateStage struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Color string `json:"color"`
}

// TemplateCategory is a category of the company; Type is "income" or "expense".
type TemplateCategory struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// TemplateCatalogItem is a sample catalog item, always without a reference price.
type TemplateCatalogItem struct {
	Name        string `json:"name"`
	Description string `json:"description"` // "" = no description
	Unit        string `json:"unit"`
	UnitLabel   string `json:"unit_label"` // only with unit "other"
}

// TemplateQuoteDefaults are the default validity in days and terms ("" = no terms) of a quote.
type TemplateQuoteDefaults struct {
	ValidityDays int    `json:"validity_days"`
	Terms        string `json:"terms"`
}

//go:embed catalog.json
var catalogJSON []byte

var loadCatalog = sync.OnceValues(func() ([]Template, error) { return parseCatalog(catalogJSON) })

func parseCatalog(data []byte) ([]Template, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var templates []Template
	if err := dec.Decode(&templates); err != nil {
		return nil, fmt.Errorf("industrytemplate: decode catalog: %w", err)
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("industrytemplate: catalog has trailing data")
	}
	if len(templates) == 0 {
		return nil, errors.New("industrytemplate: catalog is empty")
	}
	seen := make(map[string]bool, len(templates))
	for _, item := range templates {
		if strings.TrimSpace(item.Code) == "" || strings.TrimSpace(item.Name) == "" || item.Version < 1 || seen[item.Code] {
			return nil, fmt.Errorf("industrytemplate: invalid or duplicate code %q", item.Code)
		}
		// Every template has a pipeline. The other rules (enumerated values, lengths) are checked
		// against settings' own rules in a settings test (002/INV-9): this package does not know them.
		if len(item.Stages) == 0 {
			return nil, fmt.Errorf("industrytemplate: template %q has no stages", item.Code)
		}
		seen[item.Code] = true
	}
	return templates, nil
}

// All returns a deep copy of the embedded catalog, so a caller cannot alter what the next registration
// seeds. Invalid build-time data panics on first use.
func All() []Template {
	items, err := loadCatalog()
	if err != nil {
		panic(err)
	}
	out := make([]Template, len(items))
	for i, t := range items {
		t.Stages = slices.Clone(t.Stages)
		t.Categories = slices.Clone(t.Categories)
		t.CatalogItems = slices.Clone(t.CatalogItems)
		out[i] = t
	}
	return out
}

// Lookup finds a template by its stable code.
func Lookup(code string) (Template, bool) {
	for _, item := range All() {
		if item.Code == code {
			return item, true
		}
	}
	return Template{}, false
}

// Seeder adds the template's initial domain data inside the registration transaction.
// Spec 001 has no initial domain data; spec 002 will replace the no-op implementation.
type Seeder interface {
	Seed(ctx context.Context, tx db.Tx, tenantID uuid.UUID, template Template) error
}

type NoopSeeder struct{}

func (NoopSeeder) Seed(context.Context, db.Tx, uuid.UUID, Template) error { return nil }
