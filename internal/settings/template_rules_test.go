package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/industrytemplate"
)

// Names of the rules validateTemplate reports; the failure message always carries the template code
// and one of these.
const (
	ruleTemplateLabels       = "labels"
	ruleTemplatePipeline     = "pipeline"
	ruleTemplateStage        = "stage"
	ruleTemplateStageNames   = "stage names are unique"
	ruleTemplateCategory     = "category"
	ruleTemplateCategoryName = "category names are unique per type"
	ruleTemplateCatalogItem  = "catalog item"
	ruleTemplateQuote        = "quote defaults"
)

// validateTemplate applies the real rules of the configuration to the content of a template, so a
// template the API would reject cannot ship (002/DD-2, INV-9). It lives in the test: seeding does not
// validate at run time; the catalog is build-time data. The error names the template and the rule.
func validateTemplate(t industrytemplate.Template) error {
	fail := func(rule, format string, args ...any) error {
		return fmt.Errorf("template %q breaks the rule %q: %s", t.Code, rule, fmt.Sprintf(format, args...))
	}

	toLabel := func(l industrytemplate.TemplateLabel) EntityLabel {
		return EntityLabel{Singular: l.Singular, Plural: l.Plural, Gender: Gender(l.Gender)}
	}
	if _, errs := normalizeLabels(Labels{
		Project: toLabel(t.Labels.Project), Customer: toLabel(t.Labels.Customer), Quote: toLabel(t.Labels.Quote),
	}); errs != nil {
		return fail(ruleTemplateLabels, "%v", map[string]string(errs))
	}

	stages := make([]Stage, len(t.Stages))
	seenStage := map[string]bool{}
	for i, s := range t.Stages {
		name, code := normalizeStageName(s.Name)
		if code == "" {
			code = validateStageType(StageType(s.Type))
		}
		if code == "" {
			code = validateColor(Color(s.Color))
		}
		if code != "" {
			return fail(ruleTemplateStage, "stage %d (%q): %s", i+1, s.Name, code)
		}
		if key := strings.ToLower(name); seenStage[key] {
			return fail(ruleTemplateStageNames, "%q is repeated", name)
		} else {
			seenStage[key] = true
		}
		stages[i] = Stage{Name: name, Type: StageType(s.Type), Color: Color(s.Color), Position: i + 1}
	}
	if err := validatePipeline(stages); err != nil {
		return fail(ruleTemplatePipeline, "%v", err)
	}

	// The names of the active categories are unique per type, the system ones included (data-model.md
	// categories_active_name_key); the system categories are seeded in every company.
	type typedName struct{ typ, name string }
	seenCategory := map[typedName]bool{}
	for _, sc := range systemCategories {
		seenCategory[typedName{string(sc.Type), strings.ToLower(sc.Name)}] = true
	}
	for i, c := range t.Categories {
		name, code := normalizeCategoryName(c.Name)
		if code == "" {
			code = validateCategoryType(CategoryType(c.Type))
		}
		if code != "" {
			return fail(ruleTemplateCategory, "category %d (%q): %s", i+1, c.Name, code)
		}
		key := typedName{c.Type, strings.ToLower(name)}
		if seenCategory[key] {
			return fail(ruleTemplateCategoryName, "%q (%s) is repeated or is a system category", name, c.Type)
		}
		seenCategory[key] = true
	}

	for i, it := range t.CatalogItems {
		in := NewCatalogItem{Name: it.Name, Unit: Unit(it.Unit)}
		if it.Description != "" {
			in.Description = &it.Description
		}
		if it.UnitLabel != "" {
			in.UnitLabel = &it.UnitLabel
		}
		if _, errs := normalizeNewCatalogItem(in); errs != nil {
			return fail(ruleTemplateCatalogItem, "item %d (%q): %v", i+1, it.Name, map[string]string(errs))
		}
		if in.ReferencePrice != nil {
			return fail(ruleTemplateCatalogItem, "item %d (%q) has a price", i+1, it.Name)
		}
	}

	terms := &t.QuoteDefaults.Terms
	if *terms == "" {
		terms = nil
	}
	if _, errs := normalizeQuoteDefaults(QuoteDefaults{ValidityDays: t.QuoteDefaults.ValidityDays, Terms: terms}); errs != nil {
		return fail(ruleTemplateQuote, "%v", map[string]string(errs))
	}
	return nil
}

// T-002-B303 (002/INV-9, DD-2): every shipped template passes the rules the API applies to the same data.
func TestEveryShippedTemplateObeysTheConfigurationRules(t *testing.T) {
	all := industrytemplate.All()
	if len(all) == 0 {
		t.Fatal("no templates")
	}
	for _, tpl := range all {
		t.Run(tpl.Code, func(t *testing.T) {
			if err := validateTemplate(tpl); err != nil {
				t.Error(err)
			}
		})
	}
}

func loadTemplateFixture(t *testing.T, file string) industrytemplate.Template {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "templates", file))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var tpl industrytemplate.Template
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return tpl
}

// T-002-B303 (INV-9): a template that breaks a rule fails with a message that names the template and the
// rule, so the person fixing catalog.json knows where to look.
func TestInvalidTemplatesFailNamingTemplateAndRule(t *testing.T) {
	tests := []struct {
		file, code, rule string
	}{
		{"first_stage_won.json", "testdata_first_stage_won", ruleTemplatePipeline},
		{"system_category_as_income.json", "testdata_system_category_income", ruleTemplateCategoryName},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			err := validateTemplate(loadTemplateFixture(t, tc.file))
			if err == nil {
				t.Fatal("an invalid template was accepted")
			}
			if !strings.Contains(err.Error(), tc.code) || !strings.Contains(err.Error(), tc.rule) {
				t.Errorf("error %q does not name the template %q and the rule %q", err, tc.code, tc.rule)
			}
		})
	}
}

// T-002-B303 (INV-9): the decisions of validateTemplate, one per rule, each against a valid base.
func TestTemplateRuleDecisions(t *testing.T) {
	// A valid template written here, not read from the catalog: these cases test the rules.
	base := func() industrytemplate.Template {
		label := func(s, p string) industrytemplate.TemplateLabel {
			return industrytemplate.TemplateLabel{Singular: s, Plural: p, Gender: "masculine"}
		}
		return industrytemplate.Template{
			Code: "base", Name: "Base", Version: 2,
			Labels: industrytemplate.TemplateLabels{
				Project: label("Proyecto", "Proyectos"), Customer: label("Cliente", "Clientes"), Quote: label("Presupuesto", "Presupuestos"),
			},
			Stages: []industrytemplate.TemplateStage{
				{Name: "Presupuesto", Type: "open", Color: "blue"},
				{Name: "Aprobado", Type: "open", Color: "teal"},
				{Name: "Finalizado", Type: "won", Color: "green"},
				{Name: "Perdido", Type: "lost", Color: "gray"},
			},
			Categories: []industrytemplate.TemplateCategory{
				{Name: "Vidrios", Type: "expense"}, {Name: "Otros ingresos", Type: "income"},
			},
			CatalogItems:  []industrytemplate.TemplateCatalogItem{{Name: "Ventana", Unit: "square_meter"}},
			QuoteDefaults: industrytemplate.TemplateQuoteDefaults{ValidityDays: 15, Terms: "Seña del 50 %."},
		}
	}
	tests := []struct {
		name   string
		change func(*industrytemplate.Template)
		rule   string // "" = valid
	}{
		{"the base template is valid", func(*industrytemplate.Template) {}, ""},
		{"no stages", func(tpl *industrytemplate.Template) { tpl.Stages = nil }, ruleTemplatePipeline},
		{"no won stage", func(tpl *industrytemplate.Template) { tpl.Stages = tpl.Stages[:2] }, ruleTemplatePipeline},
		{"stage color outside the palette", func(tpl *industrytemplate.Template) { tpl.Stages[0].Color = "pink" }, ruleTemplateStage},
		{"stage type outside the enum", func(tpl *industrytemplate.Template) { tpl.Stages[0].Type = "Open" }, ruleTemplateStage},
		{"stage name too long", func(tpl *industrytemplate.Template) { tpl.Stages[0].Name = strings.Repeat("a", 61) }, ruleTemplateStage},
		{"stage names repeat ignoring case", func(tpl *industrytemplate.Template) { tpl.Stages[1].Name = "PRESUPUESTO" }, ruleTemplateStageNames},
		{"category type outside the enum", func(tpl *industrytemplate.Template) { tpl.Categories[0].Type = "neutral" }, ruleTemplateCategory},
		{"empty category name", func(tpl *industrytemplate.Template) { tpl.Categories[0].Name = " " }, ruleTemplateCategory},
		{"category repeated in the same type, ignoring case", func(tpl *industrytemplate.Template) {
			tpl.Categories = append(tpl.Categories, industrytemplate.TemplateCategory{Name: "VIDRIOS", Type: "expense"})
		}, ruleTemplateCategoryName},
		{"the same name in the other type is fine", func(tpl *industrytemplate.Template) {
			tpl.Categories = append(tpl.Categories, industrytemplate.TemplateCategory{Name: "Vidrios", Type: "income"})
		}, ""},
		{"a system name in the same type", func(tpl *industrytemplate.Template) {
			tpl.Categories[0] = industrytemplate.TemplateCategory{Name: "pagos a proveedores", Type: "expense"}
		}, ruleTemplateCategoryName},
		{"a system name in the other type is fine", func(tpl *industrytemplate.Template) {
			tpl.Categories[0] = industrytemplate.TemplateCategory{Name: "Pagos a proveedores", Type: "income"}
		}, ""},
		{"item unit outside the enum", func(tpl *industrytemplate.Template) { tpl.CatalogItems[0].Unit = "kg" }, ruleTemplateCatalogItem},
		{"item unit other without a label", func(tpl *industrytemplate.Template) { tpl.CatalogItems[0].Unit = "other" }, ruleTemplateCatalogItem},
		{"item unit other with a label", func(tpl *industrytemplate.Template) {
			tpl.CatalogItems[0].Unit, tpl.CatalogItems[0].UnitLabel = "other", "jornada"
		}, ""},
		{"label gender outside the enum", func(tpl *industrytemplate.Template) { tpl.Labels.Quote.Gender = "neutral" }, ruleTemplateLabels},
		{"label singular too long", func(tpl *industrytemplate.Template) { tpl.Labels.Project.Singular = strings.Repeat("a", 41) }, ruleTemplateLabels},
		{"validity 0", func(tpl *industrytemplate.Template) { tpl.QuoteDefaults.ValidityDays = 0 }, ruleTemplateQuote},
		{"validity 366", func(tpl *industrytemplate.Template) { tpl.QuoteDefaults.ValidityDays = 366 }, ruleTemplateQuote},
		{"terms too long", func(tpl *industrytemplate.Template) { tpl.QuoteDefaults.Terms = strings.Repeat("a", 2001) }, ruleTemplateQuote},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tpl := base()
			tc.change(&tpl)
			err := validateTemplate(tpl)
			switch {
			case tc.rule == "" && err != nil:
				t.Errorf("rejected: %v", err)
			case tc.rule != "" && err == nil:
				t.Error("accepted")
			case tc.rule != "" && !strings.Contains(err.Error(), `"`+tc.rule+`"`):
				t.Errorf("error %q does not name the rule %q", err, tc.rule)
			}
		})
	}
}
