package queryrules_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
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
	exceptions := []queryrules.Exception{{
		Path:    "internal/orders/store/auth_lookup.sql",
		Reason:  "routing lookup as crm_auth",
		Queries: map[string]string{"UserByEmail": "finds the company of an email"},
	}}
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
	_, err := queryrules.Check("testdata/clean", companyTables(t), []queryrules.Exception{{Path: "internal/orders/store/auth_lookup.sql"}})
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
			t.Errorf("exception %q has no reason", e.Path)
		}
	}
}

func TestCompanyTablesSeeIfNotExistsAndAddedColumns(t *testing.T) {
	got, err := queryrules.CompanyTables("testdata/migrations2")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"late", "late_bare", "late_if", "tenants", "with_tenant"}
	if !slices.Equal(got, want) {
		t.Errorf("company tables = %v, want %v", got, want)
	}
}

// The ways a filter can look present and restrict nothing, or be missing from part of the query.
func TestTrickyQueriesAreDetected(t *testing.T) {
	vs, err := queryrules.Check("testdata/violations2", companyTables(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal/orders/store/tricky.sql CommaFrom",                 // users has no filter, only t.id = @tenant_id
		"internal/orders/store/tricky.sql CteWithoutFilter",          // the CTE reads sessions unfiltered
		"internal/orders/store/tricky.sql DeleteOrTrueBeforeAnd",     // id = @id OR (true AND tenant_id = @tenant_id): AND binds tighter
		"internal/orders/store/tricky.sql InsertSelect",              // the SELECT reads users unfiltered
		"internal/orders/store/tricky.sql OrBeforeAnd",               // email = @email OR (status = 'active' AND tenant_id = @tenant_id)
		"internal/orders/store/tricky.sql OrInsideGroup",             // (tenant_id = @tenant_id OR archived)
		"internal/orders/store/tricky.sql OrOtherColumn",             // id = @id OR tenant_id = @tenant_id
		"internal/orders/store/tricky.sql OrTrue",                    // tenant_id = @tenant_id OR true
		"internal/orders/store/tricky.sql PredicateOnlyInSelectList", // @tenant_id is not a filter
		"internal/orders/store/tricky.sql QuotedCommaFrom",           // app."users" after a comma, no filter
		"internal/orders/store/tricky.sql QuotedInsert",              // INSERT INTO app."users" without @tenant_id
		"internal/orders/store/tricky.sql QuotedJoin",                // JOIN app."sessions" unfiltered
		"internal/orders/store/tricky.sql QuotedSchemaAndTable",      // FROM "app"."users"
		"internal/orders/store/tricky.sql QuotedTable",               // FROM "users"
		"internal/orders/store/tricky.sql SubqueryWithoutFilter",     // EXISTS (SELECT ... FROM users WHERE email = ...)
		"internal/orders/store/tricky.sql UnionSecondBranch",         // the second branch has no filter
		"internal/orders/store/tricky.sql UpdateTrailingOr",          // (tenant_id = @tenant_id AND id = @id) OR email = @email
	}
	if got := summarize(vs); !slices.Equal(got, want) {
		t.Errorf("violations:\n got %v\nwant %v", got, want)
	}
}

// Only the declared paths are exempt: the same file name anywhere else is reported, even if its queries are fine.
func TestExemptFileNamesOutsideTheirPathAreViolations(t *testing.T) {
	exceptions := []queryrules.Exception{
		{Path: "internal/identity/store/auth_lookup.sql", Reason: "routing lookups"},
		{Path: "internal/platform/outbox/store/worker.sql", Reason: "queue"},
	}
	vs, err := queryrules.Check("testdata/misplaced", companyTables(t), exceptions)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(vs), 2; got != want {
		t.Errorf("violations = %v, want %d (one per misplaced file)", summarize(vs), want)
	}
	for _, v := range vs {
		if v.Detail == "" {
			t.Errorf("violation %+v has no detail", v)
		}
	}
}

func TestDefaultExceptionsAreExactPaths(t *testing.T) {
	for _, e := range queryrules.DefaultExceptions {
		if strings.ContainsAny(e.Path, "*?[") {
			t.Errorf("exception %q is a pattern; defaults must be exact paths", e.Path)
		}
	}
}

// The four system-query files of plan §4.4, by exact path and nothing else.
func TestDefaultExceptionsAreTheFourFilesOfPlanSection44(t *testing.T) {
	var got []string
	for _, e := range queryrules.DefaultExceptions {
		got = append(got, e.Path)
	}
	sort.Strings(got)
	want := []string{
		"internal/identity/store/auth_lookup.sql",
		"internal/identity/store/cleanup.sql",
		"internal/platform/outbox/store/worker.sql",
		"internal/tenant/store/provisioning.sql",
	}
	if !slices.Equal(got, want) {
		t.Errorf("default exceptions = %v, want %v", got, want)
	}
}

// The same file names in another module are violations even when they are the exact names of an exemption.
// The real cleanup file lists its query, as it will when it is written, so only the misplaced files remain.
func TestCleanupAndProvisioningNamesAreExemptOnlyAtTheirPath(t *testing.T) {
	exceptions := slices.Clone(queryrules.DefaultExceptions)
	for i, e := range exceptions {
		if e.Path == "internal/identity/store/cleanup.sql" {
			exceptions[i].Queries = map[string]string{"DeleteExpired": "cleanup of expired sessions as crm_worker"}
		}
	}
	vs, err := queryrules.Check("testdata/misplaced2", companyTables(t), exceptions)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal/orders/store/cleanup.sql ",
		"internal/orders/store/provisioning.sql ",
	}
	if got := summarize(vs); !slices.Equal(got, want) {
		t.Errorf("violations = %v, want %v", got, want)
	}
}

// An exception exempts named queries, not a whole file: a new query added to an exempt file is checked
// like any other. ListAllUsers would read the emails of every company as crm_auth (PR fdelillo/crm#7).
func TestExemptionsAreByQueryNotByFile(t *testing.T) {
	exceptions := []queryrules.Exception{{
		Path:    "internal/identity/store/auth_lookup.sql",
		Reason:  "routing lookups as crm_auth",
		Queries: map[string]string{"UserByEmail": "finds the company of an email at login"},
	}}
	vs, err := queryrules.Check("testdata/exemptfile", companyTables(t), exceptions)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"internal/identity/store/auth_lookup.sql ListAllUsers"}
	if got := summarize(vs); !slices.Equal(got, want) {
		t.Errorf("violations:\n got %v\nwant %v", got, want)
	}
}

// The default exceptions list no query yet: every query written in an exempt file is checked until its
// name is added to the exception with a reason, which is where the design review happens.
func TestDefaultExceptionsExemptNoQueryUntilListed(t *testing.T) {
	vs, err := queryrules.Check("testdata/exemptfile", companyTables(t), queryrules.DefaultExceptions)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal/identity/store/auth_lookup.sql ListAllUsers",
		"internal/identity/store/auth_lookup.sql UserByEmail",
	}
	if got := summarize(vs); !slices.Equal(got, want) {
		t.Errorf("violations:\n got %v\nwant %v", got, want)
	}
}

// A listed query that is not in the file is reported, so the exemptions cannot outlive their queries.
func TestExemptQueryNamesMustExist(t *testing.T) {
	exceptions := []queryrules.Exception{{
		Path:   "internal/identity/store/auth_lookup.sql",
		Reason: "routing lookups as crm_auth",
		Queries: map[string]string{
			"UserByEmail":  "finds the company of an email at login",
			"ListAllUsers": "not a routing lookup, but listed on purpose for this test",
			"GoneQuery":    "was renamed",
		},
	}}
	vs, err := queryrules.Check("testdata/exemptfile", companyTables(t), exceptions)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"internal/identity/store/auth_lookup.sql GoneQuery"}
	if got := summarize(vs); !slices.Equal(got, want) {
		t.Errorf("violations:\n got %v\nwant %v", got, want)
	}
}

// Every exempt query carries its own reason.
func TestExemptQueriesNeedAReason(t *testing.T) {
	exceptions := []queryrules.Exception{{
		Path:    "internal/identity/store/auth_lookup.sql",
		Reason:  "routing lookups as crm_auth",
		Queries: map[string]string{"UserByEmail": " "},
	}}
	if _, err := queryrules.Check("testdata/exemptfile", companyTables(t), exceptions); err == nil {
		t.Error("Check accepted an exempt query without a reason")
	}
}

// An exception that lists queries of a file that does not exist is reported too.
func TestExemptQueriesOfAMissingFileAreReported(t *testing.T) {
	exceptions := []queryrules.Exception{{
		Path:    "internal/identity/store/cleanup.sql",
		Reason:  "periodic cleanup as crm_worker",
		Queries: map[string]string{"DeleteExpiredSessions": "cleanup of expired sessions"},
	}}
	vs, err := queryrules.Check("testdata/exemptfile", companyTables(t), exceptions)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal/identity/store/auth_lookup.sql ListAllUsers",
		"internal/identity/store/auth_lookup.sql UserByEmail",
		"internal/identity/store/cleanup.sql DeleteExpiredSessions",
	}
	if got := summarize(vs); !slices.Equal(got, want) {
		t.Errorf("violations:\n got %v\nwant %v", got, want)
	}
}
