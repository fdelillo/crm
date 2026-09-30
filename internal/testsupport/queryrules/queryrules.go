// Package queryrules checks that every sqlc query on a company table filters by company (INV-04, plan §5):
// the explicit tenant_id predicate is the first defence, RLS the second. It reads the .sql files of
// the store packages and the migrations, and is used by a test on the real tree and on fixtures.
package queryrules

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Violation is a query that touches a company table without the tenant predicate.
type Violation struct {
	File   string // relative to the root given to Check, slash-separated
	Line   int    // of the "-- name:" line
	Query  string
	Detail string
}

// Exception excludes the files that match Pattern (path.Match on the slash-separated relative path).
// Every exception carries the reason it exists.
type Exception struct {
	Pattern string
	Reason  string
}

// DefaultExceptions are the queries that legitimately run without a company: they exist to FIND the
// company, as a system role that sees only routing columns (plan §4.4, INV-05). Anything else that
// needs an exception needs a design decision first.
var DefaultExceptions = []Exception{
	{"internal/*/store/auth_lookup.sql", "phase-one lookups as crm_auth: find the company of an email, a session or a token"},
	{"internal/*/store/worker*.sql", "queue and cleanup queries as crm_worker: they see queue columns of every company"},
	{"internal/platform/*/store/worker*.sql", "queue and cleanup queries as crm_worker: they see queue columns of every company"},
}

var (
	createTableRe = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+app\.(\w+)\s*\((.*?)\n\)\s*;`)
	tenantColRe   = regexp.MustCompile(`(?im)^\s*tenant_id\s`)
	tableRefRe    = regexp.MustCompile(`(?i)\b(?:from|join|into|update)\s+(?:app\.)?([a-z_][a-z0-9_]*)`)
	tenantArg     = `(?:@tenant_id\b|sqlc\.arg\(\s*'?tenant_id'?\s*\))`
	// tenant_id = @tenant_id, with an optional alias in front of the column.
	tenantPredRe = regexp.MustCompile(`(?i)\b(?:\w+\.)?tenant_id\s*=\s*` + tenantArg)
	// id = @tenant_id: how the tenants table is filtered.
	tenantIDPredRe = regexp.MustCompile(`(?i)(?:^|[^\w.])(?:\w+\.)?id\s*=\s*` + tenantArg)
	tenantArgRe    = regexp.MustCompile(`(?i)` + tenantArg)
	insertRe       = regexp.MustCompile(`(?i)^\s*insert\s`)
)

// CompanyTables reads the migrations (only their Up sections) and returns the tables of schema app that
// have a tenant_id column, plus tenants, which is isolated by id. Sorted.
func CompanyTables(migrationsDir string) ([]string, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{"tenants": true}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(migrationsDir, e.Name()))
		if err != nil {
			return nil, err
		}
		up, _, _ := strings.Cut(string(data), "-- +goose Down")
		for _, m := range createTableRe.FindAllStringSubmatch(up, -1) {
			if tenantColRe.MatchString(m[2]) {
				set[m[1]] = true
			}
		}
	}
	var out []string
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// Check scans internal/**/store/*.sql under root. A query that references a company table must contain
// tenant_id = @tenant_id (or id = @tenant_id when the only company table is tenants; an INSERT must
// pass @tenant_id). One predicate per query is required, not one per joined table: RLS is the second
// line. Files matching an exception are skipped. It fails if root has no internal/ directory.
func Check(root string, companyTables []string, exceptions []Exception) ([]Violation, error) {
	for _, e := range exceptions {
		if strings.TrimSpace(e.Reason) == "" {
			return nil, fmt.Errorf("queryrules: exception %q has no reason", e.Pattern)
		}
	}
	internal := filepath.Join(root, "internal")
	if _, err := os.Stat(internal); err != nil {
		return nil, fmt.Errorf("queryrules: %w", err)
	}
	company := map[string]bool{}
	for _, t := range companyTables {
		company[t] = true
	}

	var out []Violation
	err := filepath.WalkDir(internal, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".sql") || path.Base(path.Dir(rel)) != "store" || excepted(rel, exceptions) {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out = append(out, checkFile(rel, string(data), company)...)
		return nil
	})
	return out, err
}

func excepted(rel string, exceptions []Exception) bool {
	for _, e := range exceptions {
		if ok, _ := path.Match(e.Pattern, rel); ok {
			return true
		}
	}
	return false
}

type query struct {
	name string
	line int
	sql  string
}

// splitQueries cuts a sqlc file at its "-- name:" lines and drops comments from the SQL.
func splitQueries(content string) []query {
	var qs []query
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(trimmed, "-- name:"); ok {
			name, _, _ := strings.Cut(strings.TrimSpace(rest), " ")
			qs = append(qs, query{name: name, line: i + 1})
			continue
		}
		if len(qs) == 0 {
			continue
		}
		if before, _, found := strings.Cut(line, "--"); found {
			line = before
		}
		qs[len(qs)-1].sql += line + "\n"
	}
	return qs
}

func checkFile(rel, content string, company map[string]bool) []Violation {
	var out []Violation
	for _, q := range splitQueries(content) {
		var touched []string
		for _, m := range tableRefRe.FindAllStringSubmatch(q.sql, -1) {
			if company[m[1]] {
				touched = append(touched, m[1])
			}
		}
		if len(touched) == 0 {
			continue
		}
		onlyTenants := true
		for _, tb := range touched {
			onlyTenants = onlyTenants && tb == "tenants"
		}
		var ok bool
		var want string
		switch {
		case insertRe.MatchString(q.sql):
			ok, want = tenantArgRe.MatchString(q.sql), "@tenant_id as the value of tenant_id"
		case onlyTenants:
			ok, want = tenantIDPredRe.MatchString(q.sql), "id = @tenant_id"
		default:
			ok, want = tenantPredRe.MatchString(q.sql), "tenant_id = @tenant_id"
		}
		if !ok {
			out = append(out, Violation{rel, q.line, q.name,
				fmt.Sprintf("touches %s and lacks %s", strings.Join(touched, ", "), want)})
		}
	}
	return out
}
