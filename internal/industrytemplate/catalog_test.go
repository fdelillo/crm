package industrytemplate_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/go-chi/chi/v5"
)

func TestCatalogAndPublicHandler(t *testing.T) {
	all := industrytemplate.All()
	if len(all) < 2 {
		t.Fatalf("catalog has %d templates", len(all))
	}
	for code, name := range map[string]string{"generic": "Genérico", "aluminum_carpentry": "Carpintería de aluminio"} {
		got, ok := industrytemplate.Lookup(code)
		if !ok || got.Name != name || got.Version < 1 {
			t.Errorf("template %s = %+v, %v", code, got, ok)
		}
	}
	if _, ok := industrytemplate.Lookup("inexistente"); ok {
		t.Fatal("unknown template found")
	}
	r := chi.NewRouter()
	industrytemplate.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/industry-templates", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	contract.Default(t).RequireRecorded(t, req, rec)
	var body struct {
		Items []struct{ Code, Name string } `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != len(all) {
		t.Fatalf("items = %d, want %d", len(body.Items), len(all))
	}
}
