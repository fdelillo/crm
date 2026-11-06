package industrytemplate

import (
	"strings"
	"testing"
)

// validTemplate is a minimal catalog entry that parseCatalog accepts; each case below breaks one thing.
const validTemplate = `{"code":"generic","name":"Genérico","version":2,
 "labels":{"project":{"singular":"Proyecto","plural":"Proyectos","gender":"masculine"},
  "customer":{"singular":"Cliente","plural":"Clientes","gender":"masculine"},
  "quote":{"singular":"Presupuesto","plural":"Presupuestos","gender":"masculine"}},
 "stages":[{"name":"Presupuesto","type":"open","color":"blue"},{"name":"Finalizado","type":"won","color":"green"}],
 "categories":[],"catalog_items":[],"quote_defaults":{"validity_days":15,"terms":""}}`

// T-002-B303 (DD-2): the base entry of the rejection cases is itself valid, so each of them fails for its own rule.
func TestParseCatalogAcceptsAValidEntry(t *testing.T) {
	got, err := parseCatalog([]byte("[" + validTemplate + "]"))
	if err != nil || len(got) != 1 || len(got[0].Stages) != 2 {
		t.Fatalf("parseCatalog = %+v, %v", got, err)
	}
}

// T-002-B303 (DD-2): the catalog is read strictly; every case breaks exactly one rule of the valid entry.
func TestParseCatalogRejectsInvalidData(t *testing.T) {
	replace := func(old, new string) string {
		if !strings.Contains(validTemplate, old) {
			t.Fatalf("validTemplate has no %q", old)
		}
		return "[" + strings.Replace(validTemplate, old, new, 1) + "]"
	}
	tests := []struct {
		name string
		raw  string
	}{
		{"empty catalog", `[]`},
		{"empty code", replace(`"code":"generic"`, `"code":""`)},
		{"empty name", replace(`"name":"Genérico"`, `"name":""`)},
		{"version 0", replace(`"version":2`, `"version":0`)},
		{"duplicate code", "[" + validTemplate + "," + validTemplate + "]"},
		{"unknown field in the template", replace(`"categories":[]`, `"categories":[],"unknown":true`)},
		{"unknown field in a stage", replace(`"color":"blue"`, `"color":"blue","icon":"x"`)},
		{"unknown field in a label", replace(`"gender":"masculine"}`, `"gender":"masculine","x":1}`)},
		{"template without stages", replace(`"stages":[{"name":"Presupuesto","type":"open","color":"blue"},{"name":"Finalizado","type":"won","color":"green"}]`, `"stages":[]`)},
		{"template with null stages", replace(`"stages":[{"name":"Presupuesto","type":"open","color":"blue"},{"name":"Finalizado","type":"won","color":"green"}]`, `"stages":null`)},
		{"template without the stages key", replace(`"stages":[{"name":"Presupuesto","type":"open","color":"blue"},{"name":"Finalizado","type":"won","color":"green"}],`, ``)},
		{"trailing data", "[" + validTemplate + "] []"},
	}
	for _, tc := range tests {
		if _, err := parseCatalog([]byte(tc.raw)); err == nil {
			t.Errorf("%s: accepted", tc.name)
		}
	}
}
