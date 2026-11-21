package app_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/contract"
)

// contractRouteProblems compares the declared operations with the routes the router implements.
// implemented holds "METHOD /api/v1/path" keys.
func contractRouteProblems(ops []contract.Operation, implemented map[string]bool) []string {
	var problems []string
	declared := map[string]bool{}
	for _, op := range ops {
		if op.Path == "/healthz" || op.Path == "/readyz" {
			continue
		}
		key := op.Method + " /api/v1" + op.Path
		declared[key] = true
		if !implemented[key] {
			problems = append(problems, "contract route not implemented: "+key)
		}
	}
	for key := range implemented {
		if !declared[key] {
			problems = append(problems, "implemented route outside contract: "+key)
		}
	}
	slices.Sort(problems)
	return problems
}

// T-002-B203 (DD-13), T-002-B801: the contract of 002 declares 20 operations, all of them implemented.
func TestSecondContractDeclaresItsOperations(t *testing.T) {
	declared := 0
	for _, op := range contract.Default(t).Operations() {
		if op.Document == contract.Documents[1] {
			declared++
		}
	}
	if declared != 20 {
		t.Errorf("the 002 contract declares %d operations, want 20", declared)
	}
}

// T-002-B203 (T-002-B801, DD-13): every declared operation has a route and every route is declared.
func TestContractRouteProblems(t *testing.T) {
	ops := []contract.Operation{
		{Method: "GET", Path: "/me", ID: "getMe", Document: contract.Documents[0]},
		{Method: "GET", Path: "/stages", ID: "listStages", Document: contract.Documents[1]},
		{Method: "GET", Path: "/healthz", ID: "liveness", Document: contract.Documents[0]},
	}
	tests := []struct {
		name        string
		implemented map[string]bool
		want        []string
	}{
		{"every operation has its route", map[string]bool{"GET /api/v1/me": true, "GET /api/v1/stages": true}, nil},
		{"operation without route", map[string]bool{"GET /api/v1/me": true},
			[]string{"contract route not implemented: GET /api/v1/stages"}},
		{"implemented route outside the contract", map[string]bool{"GET /api/v1/me": true, "GET /api/v1/stages": true, "GET /api/v1/other": true},
			[]string{"implemented route outside contract: GET /api/v1/other"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contractRouteProblems(ops, tt.implemented)
			if len(got) != len(tt.want) {
				t.Fatalf("problems=%q, want %d containing %q", got, len(tt.want), tt.want)
			}
			for i, want := range tt.want {
				if !strings.Contains(got[i], want) {
					t.Errorf("problem %d=%q, want it to contain %q", i, got[i], want)
				}
			}
		})
	}
}
