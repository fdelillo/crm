package reporules_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/reporules"
)

const (
	ruleRole  = reporules.RuleRoleSwitch
	ruleSQL   = reporules.RuleDynamicSQL
	ruleRoute = reporules.RuleChiOutsideAPI
)

func summarize(vs []reporules.Violation) []string {
	var out []string
	for _, v := range vs {
		out = append(out, fmt.Sprintf("%s %s", v.Rule, filepath.ToSlash(v.File)))
	}
	sort.Strings(out)
	return out
}

// The real tree obeys the rules (INV-03, INV-04, INV-22, ADR-001).
func TestRepositoryObeysTheRules(t *testing.T) {
	vs, err := reporules.Check(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		t.Errorf("%s: %s:%d: %s", v.Rule, v.File, v.Line, v.Detail)
	}
}

// Rule 1: SET ROLE, SET LOCAL ROLE, RESET ROLE and set_config('role' only in internal/platform/db.
func TestRule_RoleSwitchOnlyInPlatformDB(t *testing.T) {
	vs, err := reporules.Check("testdata/violations")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range vs {
		if v.Rule == ruleRole {
			got = append(got, fmt.Sprintf("%s:%d", filepath.ToSlash(v.File), v.Line))
		}
	}
	sort.Strings(got)
	want := []string{
		"cmd/crm/main.go:4",                         // RESET ROLE in cmd
		"internal/orders/setrole.go:4",              // SET ROLE
		"internal/orders/setrole.go:6",              // set local role
		"internal/orders/setrole.go:8",              // RESET ROLE
		"internal/orders/setrole.go:10",             // set_config('role'
		"internal/platform/httpx/setrole_sql.sql:1", // .sql files count too
		"internal/orders/sqlcalls.go:56",            // set_config with the GUC name as a parameter
		"internal/orders/sqlcalls.go:58",            // set_config('session_authorization'...)
	}
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Errorf("role-switch violations:\n got %v\nwant %v", got, want)
	}
}

// Rule 2: no SQL built with fmt.Sprintf or concatenation outside internal/platform/db.
func TestRule_NoDynamicSQLOutsidePlatformDB(t *testing.T) {
	vs, err := reporules.Check("testdata/violations")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range vs {
		if v.Rule == ruleSQL {
			got = append(got, fmt.Sprintf("%s:%d", filepath.ToSlash(v.File), v.Line))
		}
	}
	sort.Strings(got)
	want := []string{
		"internal/orders/sqlsprintf.go:7",  // Sprintf
		"internal/orders/sqlsprintf.go:11", // "SELECT " + "id ..." + id
		"internal/orders/sqlsprintf.go:14", // "SELECT " + x
		"internal/orders/sqlsprintf.go:17", // "UPDATE " + table + " SET ..."
		"internal/orders/sqlcalls.go:20",   // Exec(ctx, q): the SQL argument is a variable
		"internal/orders/sqlcalls.go:25",   // strings.Builder.WriteString of SQL
		"internal/orders/sqlcalls.go:27",   // Query(ctx, sb.String())
		"internal/orders/sqlcalls.go:32",   // q += " AND name = '" + name + "'"
		"internal/orders/sqlcalls.go:33",   // QueryRow(ctx, q)
		"internal/orders/sqlcalls.go:37",   // strings.Join([]string{"DELETE FROM", t}, " ")
		"internal/orders/sqlcalls.go:41",   // Sprintf with a comment before SELECT
		"internal/orders/sqlcalls.go:45",   // Batch.Queue(q)
		"internal/orders/sqlcalls.go:49",   // strings.ReplaceAll on a SQL literal
		"internal/orders/sqlcalls.go:53",   // fmt.Fprintf building SQL
	}
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Errorf("dynamic-SQL violations:\n got %v\nwant %v", got, want)
	}
}

// Rule 3: the API router never registers the SPA (no Mount("/"), no NotFound towards the SPA, no /*).
func TestRule_ChiRouterStaysUnderAPI(t *testing.T) {
	vs, err := reporules.Check("testdata/violations")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range vs {
		if v.Rule == ruleRoute {
			got = append(got, fmt.Sprintf("%s:%d", filepath.ToSlash(v.File), v.Line))
		}
	}
	sort.Strings(got)
	want := []string{
		"internal/orders/routes.go:12", // r.Mount("/", spa)
		"internal/orders/routes.go:13", // r.NotFound(spa.ServeHTTP)
		"internal/orders/routes.go:14", // r.Get("/*", ...)
	}
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Errorf("router violations:\n got %v\nwant %v", got, want)
	}
}

// Allowed code: platform/db, testsupport, test files, constant SQL and look-alike messages.
func TestCleanTreeHasNoViolations(t *testing.T) {
	vs, err := reporules.Check("testdata/clean")
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 0 {
		t.Errorf("unexpected violations: %v", summarize(vs))
	}
}

// A root without internal/ or cmd/ is a mistake in the test, not a silent pass.
func TestCheckFailsOnAMissingRoot(t *testing.T) {
	if _, err := reporules.Check("testdata/does-not-exist"); err == nil {
		t.Error("Check on a directory without internal/ or cmd/ returned no error")
	}
}
