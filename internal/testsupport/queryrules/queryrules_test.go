package queryrules_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/queryrules"
)

func summarize(vs []queryrules.Violation) []string {
	var out []string
	for _, v := range vs {
		out = append(out, fmt.Sprintf("%s %s", filepath.ToSlash(v.File), v.Query))
	}
	sort.Strings(out)
	return out
}

func TestCompanyTablesComeFromTheUpSectionOfTheMigrations(t *testing.T) {
	got, err := queryrules.CompanyTables("testdata/migrations")
	if err != nil {
		t.Fatal(err)
	}
	// tenants is always a company table (isolated by id); login_throttles has no tenant_id; not_in_up
	// only exists in a Down section.
	want := []string{"outbox_messages", "sessions", "tenants", "user_tokens", "users"}
	if !slices.Equal(got, want) {
		t.Errorf("company tables = %v, want %v", got, want)
	}
}

func companyTables(t *testing.T) []string {
	t.Helper()
	tables, err := queryrules.CompanyTables("testdata/migrations")
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

// T-B112, INV-04: a query on a company table without tenant_id = @tenant_id is reported.
func TestQueriesWithoutTheTenantFilterAreDetected(t *testing.T) {
	vs, err := queryrules.Check("testdata/violations", companyTables(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal/orders/store/auth_lookup.sql UserByEmail", // no exceptions given
		"internal/orders/store/bad.sql EnqueueEmail",        // INSERT without @tenant_id
		"internal/orders/store/bad.sql GetSession",          // filters by id only
		"internal/orders/store/bad.sql ListUsers",           // no WHERE
		"internal/orders/store/bad.sql RenameTenant",        // tenants needs id = @tenant_id
		"internal/orders/store/bad.sql WrongColumn",         // unqualified table, no tenant filter
	}
	if got := summarize(vs); !slices.Equal(got, want) {
		t.Errorf("violations:\n got %v\nwant %v", got, want)
	}
}

// Filtered queries, comments that mention tables, non-company tables and the explicit exceptions pass.
func TestCleanQueriesHaveNoViolations(t *testing.T) {
	exceptions := []queryrules.Exception{{Pattern: "internal/*/store/auth_lookup.sql", Reason: "routing lookup as crm_auth"}}
	vs, err := queryrules.Check("testdata/clean", companyTables(t), exceptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 0 {
		t.Errorf("unexpected violations: %v", summarize(vs))
	}
}

// An exception without a reason is a mistake: exceptions are explicit and justified.
func TestExceptionsNeedAReason(t *testing.T) {
	_, err := queryrules.Check("testdata/clean", companyTables(t), []queryrules.Exception{{Pattern: "internal/*/store/auth_lookup.sql"}})
	if err == nil {
		t.Error("Check accepted an exception without a reason")
	}
}

func TestCheckFailsOnAMissingRoot(t *testing.T) {
	if _, err := queryrules.Check("testdata/nope", companyTables(t), nil); err == nil {
		t.Error("Check on a directory without internal/ returned no error")
	}
}

// The real tree: every query of every store filters by company, except the documented lookups.
func TestRepositoryQueriesFilterByCompany(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	tables, err := queryrules.CompanyTables(filepath.Join(root, "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tenants", "users", "sessions", "user_tokens", "outbox_messages", "audit_log"} {
		if !slices.Contains(tables, want) {
			t.Fatalf("company tables read from the migrations = %v, missing %s", tables, want)
		}
	}
	if slices.Contains(tables, "login_throttles") {
		t.Error("login_throttles has no tenant_id and must not be a company table")
	}
	vs, err := queryrules.Check(root, tables, queryrules.DefaultExceptions)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		t.Errorf("%s:%d %s: %s", v.File, v.Line, v.Query, v.Detail)
	}
	for _, e := range queryrules.DefaultExceptions {
		if e.Reason == "" {
			t.Errorf("exception %q has no reason", e.Pattern)
		}
	}
}
