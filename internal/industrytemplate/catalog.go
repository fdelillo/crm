package industrytemplate

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/google/uuid"
)

// Template is a versioned, business-independent initial company configuration.
type Template struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

//go:embed catalog.json
var catalogJSON []byte

var loadCatalog = sync.OnceValues(func() ([]Template, error) {
	dec := json.NewDecoder(bytes.NewReader(catalogJSON))
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
		seen[item.Code] = true
	}
	return templates, nil
})

// All returns a copy of the embedded catalog. Invalid build-time data panics on first use.
func All() []Template {
	items, err := loadCatalog()
	if err != nil {
		panic(err)
	}
	return append([]Template(nil), items...)
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
