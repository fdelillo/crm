package authz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"
)

var expected = []struct {
	permission Permission
	operator   bool
}{
	{Permission("customers.manage"), true},
	{Permission("projects.manage"), true},
	{Permission("quotes.manage"), true},
	{Permission("customer_receipts.create"), true},
	{Permission("balances.view"), true},
	{Permission("project_costs.view"), false},
	{Permission("suppliers.view_contact"), true},
	{Permission("party_finances.manage"), false},
	{Permission("money_accounts.manage"), false},
	{Permission("financial_operations.manage"), false},
	{Permission("checks.manage"), false},
	{Permission("bank_statements.import"), false},
	{Permission("reports.view"), false},
	{Permission("settings.manage"), false},
	{Permission("movements.void"), false},
}

func TestPermissionMatrixAndContract(t *testing.T) {
	var admin, operator []Permission
	for _, row := range expected {
		if !Can(RoleAdmin, row.permission) {
			t.Errorf("admin denied %s", row.permission)
		}
		if got := Can(RoleOperator, row.permission); got != row.operator {
			t.Errorf("operator %s=%v", row.permission, got)
		}
		if Can(Role("otro"), row.permission) {
			t.Errorf("unknown role allowed %s", row.permission)
		}
		admin = append(admin, row.permission)
		if row.operator {
			operator = append(operator, row.permission)
		}
	}
	if !slices.Equal(Permissions(RoleAdmin), admin) || !slices.Equal(Permissions(RoleOperator), operator) || len(Permissions(Role("otro"))) != 0 {
		t.Fatalf("lists admin=%v operator=%v", Permissions(RoleAdmin), Permissions(RoleOperator))
	}
	data, err := os.ReadFile("../../specs/001-empresas-usuarios/contracts/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Components struct {
			Schemas struct {
				Permission struct {
					Enum []string `yaml:"enum"`
				} `yaml:"Permission"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range admin {
		got = append(got, string(p))
	}
	if !slices.Equal(got, contract.Components.Schemas.Permission.Enum) {
		t.Fatalf("permission enum drift: code=%v contract=%v", got, contract.Components.Schemas.Permission.Enum)
	}
}

func TestRequirePermission(t *testing.T) {
	called := 0
	h := RequirePermission(Permission("settings.manage"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called++; w.WriteHeader(204) }))
	cases := []struct {
		name      string
		role      Role
		principal bool
		status    int
		code      string
	}{
		{"absent", "", false, 401, "unauthenticated"},
		{"operator", RoleOperator, true, 403, "forbidden"},
		{"admin", RoleAdmin, true, 204, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tc.principal {
				r = r.WithContext(WithPrincipal(context.Background(), Principal{Role: tc.role}))
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d", w.Code)
			}
			if tc.code != "" {
				var p struct{ Code string }
				if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Code != tc.code {
					t.Fatalf("problem=%s err=%v", w.Body.String(), err)
				}
			}
		})
	}
	if called != 1 {
		t.Fatalf("handler called %d times", called)
	}
}
