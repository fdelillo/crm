package industrytemplate

import "testing"

func TestInvalidCatalog(t *testing.T) {
	for _, raw := range []string{
		`[]`,
		`[{"code":"","name":"Genérico","version":1}]`,
		`[{"code":"generic","name":"","version":1}]`,
		`[{"code":"generic","name":"Genérico","version":0}]`,
		`[{"code":"generic","name":"Genérico","version":1},{"code":"generic","name":"Otro","version":1}]`,
		`[{"code":"generic","name":"Genérico","version":1,"unknown":true}]`,
	} {
		if _, err := parseCatalog([]byte(raw)); err == nil {
			t.Errorf("invalid catalog accepted: %s", raw)
		}
	}
}
