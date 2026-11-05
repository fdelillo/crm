package app_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/contract"
)

// pendingOperations lists the operationIds of spec 002 whose routes are not registered yet.
// Each phase of 002 removes the operations it implements; the last one leaves it empty (DD-13).
var pendingOperations = map[string]bool{
	"listStages": true, "createStage": true, "updateStage": true, "deleteStage": true, "moveStage": true,
	"listCategories": true, "createCategory": true, "renameCategory": true, "archiveCategory": true, "restoreCategory": true,
	"listCatalogItems": true, "createCatalogItem": true, "getCatalogItem": true, "updateCatalogItem": true,
	"archiveCatalogItem": true, "restoreCatalogItem": true,
	"getLabels": true, "replaceLabels": true, "getQuoteDefaults": true, "replaceQuoteDefaults": true,
}

// contractRouteProblems compares the declared operations with the routes the router implements.
// implemented holds "METHOD /api/v1/path" keys; pending holds operationIds.
func contractRouteProblems(ops []contract.Operation, implemented, pending map[string]bool) []string {
	var problems []string
	declared := map[string]bool{}
	known := map[string]bool{}
	for _, op := range ops {
		if op.Path == "/healthz" || op.Path == "/readyz" {
			continue
		}
		key := op.Method + " /api/v1" + op.Path
		declared[key] = true
		known[op.ID] = true
		switch {
		case pending[op.ID] && op.Document == contract.Documents[0]:
			problems = append(problems, fmt.Sprintf("pendingOperations lists %s, an operation of %s", op.ID, op.Document))
		case pending[op.ID] && implemented[key]:
			problems = append(problems, fmt.Sprintf("route %s is implemented: remove it from pendingOperations (%s)", key, op.ID))
		case !pending[op.ID] && !implemented[key]:
			problems = append(problems, "contract route not implemented: "+key)
		}
	}
	for key := range implemented {
		if !declared[key] {
			problems = append(problems, "implemented route outside contract: "+key)
		}
	}
	for id := range pending {
		if !known[id] {
			problems = append(problems, "pendingOperations lists unknown operation "+id)
		}
	}
	slices.Sort(problems)
	return problems
}

// T-002-B203 (DD-13): pendingOperations is exactly the 20 operations of 002 and none of 001.
func TestPendingOperationsAreExactlyThoseOfTheSecondContract(t *testing.T) {
	var declared []string
	for _, op := range contract.Default(t).Operations() {
		if op.Document == contract.Documents[1] {
			declared = append(declared, op.ID)
		} else if pendingOperations[op.ID] {
			t.Errorf("pendingOperations lists %s, an operation of %s", op.ID, op.Document)
		}
	}
	if len(declared) != 20 {
		t.Errorf("the 002 contract declares %d operations, want 20", len(declared))
	}
	var pending []string
	for id := range pendingOperations {
		pending = append(pending, id)
	}
	slices.Sort(declared)
	slices.Sort(pending)
	if !slices.Equal(declared, pending) {
		t.Fatalf("pendingOperations differs from the 002 contract\n got: %v\nwant: %v", pending, declared)
	}
}

// T-002-B203 (T-B801, DD-13): pending operations may lack a route, nothing else may.
func TestContractRouteProblems(t *testing.T) {
	ops := []contract.Operation{
		{Method: "GET", Path: "/me", ID: "getMe", Document: contract.Documents[0]},
		{Method: "GET", Path: "/stages", ID: "listStages", Document: contract.Documents[1]},
		{Method: "GET", Path: "/healthz", ID: "liveness", Document: contract.Documents[0]},
	}
	tests := []struct {
		name        string
		implemented map[string]bool
		pending     map[string]bool
		want        []string
	}{
		{"pending operation without route is fine", map[string]bool{"GET /api/v1/me": true}, map[string]bool{"listStages": true}, nil},
		{"pending operation with a route", map[string]bool{"GET /api/v1/me": true, "GET /api/v1/stages": true}, map[string]bool{"listStages": true},
			[]string{"remove it from pendingOperations"}},
		{"operation without route and not pending", map[string]bool{"GET /api/v1/me": true}, nil,
			[]string{"contract route not implemented: GET /api/v1/stages"}},
		{"implemented route outside the contract", map[string]bool{"GET /api/v1/me": true, "GET /api/v1/stages": true, "GET /api/v1/other": true}, nil,
			[]string{"implemented route outside contract: GET /api/v1/other"}},
		{"pending operation unknown to every contract", map[string]bool{"GET /api/v1/me": true}, map[string]bool{"listStages": true, "noSuchOperation": true},
			[]string{"pendingOperations lists unknown operation noSuchOperation"}},
		{"pending operation of 001", map[string]bool{}, map[string]bool{"listStages": true, "getMe": true},
			[]string{"pendingOperations lists getMe"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contractRouteProblems(ops, tt.implemented, tt.pending)
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
