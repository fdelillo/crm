package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

// seederArgument returns the third argument of the call to tenant.NewService in file, as source text.
func seederArgument(t *testing.T, file string) string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 3 {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewService" && types.ExprString(sel.X) == "tenant" {
			found = append(found, types.ExprString(call.Args[2]))
		}
		return true
	})
	if len(found) != 1 {
		t.Fatalf("%s: want exactly one tenant.NewService call, found %d", file, len(found))
	}
	return found[0]
}

// T-002-B306 (DD-3 of 002): the server seeds the configuration of a template at registration with the real
// seeder; the reprovisioning command never registers, so it keeps the no-op one.
func TestSeederWiring(t *testing.T) {
	if got := seederArgument(t, "serve.go"); !strings.HasPrefix(got, "settings.NewTemplateSeeder(") {
		t.Errorf("serve.go registers companies with %s, want settings.NewTemplateSeeder(...)", got)
	}
	if got := seederArgument(t, "tenants.go"); got != "industrytemplate.NoopSeeder{}" {
		t.Errorf("tenants.go builds the company service with %s, want industrytemplate.NoopSeeder{}", got)
	}
}
