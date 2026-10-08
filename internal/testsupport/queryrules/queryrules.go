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

// Exception names a store file whose listed queries run without a company filter. Only the queries in
// Queries (sqlc name -> reason) are exempt; every other query of the file is checked like any other, so
// adding a query to an exempt file never exempts it by accident (PR fdelillo/crm#7). A listed name that
// is not in the file is a violation. Path is exact and slash-separated; the file and every query carry
// the reason they exist.
type Exception struct {
	Path    string
	Reason  string
	Queries map[string]string
}

// DefaultExceptions are the files of the queries that legitimately run without a company: they exist to
// FIND the company or to clean up, as a system role that sees only routing columns (plan §4.4, INV-05).
// The four paths are exact ("Rutas exactas de las queries de sistema"); a file with any of these names
// anywhere else is a violation (see Check). No query is listed yet: each one is added here by name, with
// its reason, in the change that writes it, so that the exemption is reviewed with the query.
var DefaultExceptions = []Exception{
	{Path: "internal/identity/store/auth_lookup.sql", Reason: "phase-one lookups as crm_auth: find the company of an email, a session or a token",
		Queries: map[string]string{
			"LookupSessionByHash": "crm_auth sees only session routing columns to find the tenant of a token",
			"LookupUserByEmail":   "crm_auth sees only user routing columns to find the tenant of an email",
			"LookupTokenByHash":   "crm_auth sees only token routing columns to find its tenant",
		}},
	{Path: "internal/identity/store/cleanup.sql", Reason: "periodic cleanup as crm_worker: DELETE of expired rows of sessions, user_tokens and login_throttles; the policies of data-model §3.4 bound which rows", Queries: map[string]string{
		"DeleteExpiredSessions":   "crm_worker: old expired or revoked sessions",
		"DeleteExpiredUserTokens": "crm_worker: old tokens, preserving open invitations (DD-25)",
		"DeleteOldLoginThrottles": "crm_worker: old throttles without a live lock",
	}},
	{Path: "internal/platform/outbox/store/worker.sql", Reason: "queue and cleanup queries as crm_worker: they see queue columns of every company",
		Queries: map[string]string{
			"PendingStats":           "crm_worker: sample global pending queue columns every 60 seconds",
			"LockDueMessage":         "crm_worker locks one due routing row across tenants; recipient and payload are read only after AsTenant",
			"DeleteTerminalMessages": "crm_worker: terminal messages older than 30 days",
			"DeferMessage":           "crm_worker: aplazar un mensaje pendiente cuya fase de empresa falló (ADR-024 §6)",
		}},
	{Path: "internal/tenant/store/provisioning.sql", Reason: "crm_worker and crm_signup: list tenants.id and provision company roles", Queries: map[string]string{
		"CountTenants":            "crm_worker: sample tenants count",
		"CountTenantsWithoutRole": "crm_worker: sample companies missing their derived role",
	}},
}

var (
	createTableRe = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?app\.(\w+)\s*\((.*?)\n\)\s*;`)
	tenantColRe   = regexp.MustCompile(`(?im)^\s*tenant_id\s`)
	// ALTER TABLE app.x ... ADD [COLUMN] [IF NOT EXISTS] tenant_id: every ADD in the statement is looked at.
	alterTableRe = regexp.MustCompile(`(?is)ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?app\.(\w+)\s+(.*?);`)
	addTenantRe  = regexp.MustCompile(`(?is)\bADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?tenant_id\b`)
	tableRefRe   = regexp.MustCompile(`(?i)\b(?:from|join|into|update)\s+(?:only\s+)?(?:"?app"?\.)?"?([a-z_][a-z0-9_]*)`)
	tenantArg    = `(?:@tenant_id\b|sqlc\.arg\(\s*'?tenant_id'?\s*\))`
	// A conjunct that is exactly tenant_id = @tenant_id (alias allowed, either order).
	tenantPredRe = regexp.MustCompile(`(?i)^(?:(?:\w+\.)?tenant_id\s*=\s*` + tenantArg + `|` + tenantArg + `\s*=\s*(?:\w+\.)?tenant_id)$`)
	// id = @tenant_id: how the tenants table is filtered.
	tenantIDPredRe = regexp.MustCompile(`(?i)^(?:(?:\w+\.)?id\s*=\s*` + tenantArg + `|` + tenantArg + `\s*=\s*(?:\w+\.)?id)$`)
	tenantArgRe    = regexp.MustCompile(`(?i)` + tenantArg)
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
		for _, m := range alterTableRe.FindAllStringSubmatch(up, -1) {
			if addTenantRe.MatchString(m[2]) {
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
// line. In a file named by an exception, only the listed queries are skipped; a listed query that does not
// exist, in the file or because the file is missing, is a violation. It fails if root has no internal/
// directory.
func Check(root string, companyTables []string, exceptions []Exception) ([]Violation, error) {
	for _, e := range exceptions {
		if strings.TrimSpace(e.Reason) == "" {
			return nil, fmt.Errorf("queryrules: exception %q has no reason", e.Path)
		}
		if strings.ContainsAny(e.Path, "*?[") {
			return nil, fmt.Errorf("queryrules: exception %q is a pattern; exceptions are exact paths", e.Path)
		}
		for name, reason := range e.Queries {
			if strings.TrimSpace(reason) == "" {
				return nil, fmt.Errorf("queryrules: exempt query %s in %q has no reason", name, e.Path)
			}
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
	visited := map[string]bool{}
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
		if !strings.HasSuffix(rel, ".sql") || path.Base(path.Dir(rel)) != "store" {
			return nil
		}
		exc, isExempt := exceptionFor(rel, exceptions)
		visited[rel] = isExempt
		if !isExempt && lookalike(rel, exceptions) {
			out = append(out, Violation{File: rel, Line: 1, Detail: "has the name of an exempt file but is not at the exempt path: " +
				"only the declared files may skip the company filter"})
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out = append(out, checkFile(rel, string(data), company, exc.Queries)...)
		return nil
	})
	if err != nil {
		return out, err
	}
	for _, e := range exceptions {
		if visited[e.Path] {
			continue
		}
		names := make([]string, 0, len(e.Queries))
		for name := range e.Queries {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			out = append(out, Violation{e.Path, 1, name, "is listed as an exempt query but the file does not exist"})
		}
	}
	return out, nil
}

// lookalike reports a file that shares its base name with an exception but is not one.
func lookalike(rel string, exceptions []Exception) bool {
	for _, e := range exceptions {
		if path.Base(e.Path) == path.Base(rel) {
			return true
		}
	}
	return false
}

func exceptionFor(rel string, exceptions []Exception) (Exception, bool) {
	for _, e := range exceptions {
		if e.Path == rel {
			return e, true
		}
	}
	return Exception{}, false
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

// checkFile analyzes every query of a file except the exempt ones, and reports exempt names that the
// file does not have.
func checkFile(rel, content string, company map[string]bool, exempt map[string]string) []Violation {
	var out []Violation
	seen := map[string]bool{}
	for _, q := range splitQueries(content) {
		seen[q.name] = true
		if _, ok := exempt[q.name]; ok {
			continue
		}
		if details := analyze(q.sql, company); len(details) > 0 {
			out = append(out, Violation{rel, q.line, q.name, strings.Join(details, "; ")})
		}
	}
	var stale []string
	for name := range exempt {
		if !seen[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	for _, name := range stale {
		out = append(out, Violation{rel, 1, name, "is listed as an exempt query but the file has no query with that name"})
	}
	return out
}

// ---- A small SQL reader: enough structure to tell the top level of a statement from what is nested. ----

// mask returns s with everything inside parentheses or single quotes replaced by spaces (the parentheses
// stay), so keywords can be searched at the top level with the same offsets as s.
func mask(s string) string {
	b := []byte(s)
	depth, inQuote := 0, false
	for i, c := range b {
		switch {
		case inQuote:
			if c == '\'' {
				inQuote = false
			} else {
				b[i] = ' '
			}
		case c == '\'':
			inQuote = true
			b[i] = ' '
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth > 0:
			b[i] = ' '
		}
	}
	return string(b)
}

// extractSubqueries replaces every parenthesised SELECT/WITH with "(subquery)" and returns their texts.
func extractSubqueries(s string) (string, []string) {
	var subs []string
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '(' {
			out.WriteByte(s[i])
			i++
			continue
		}
		depth, j := 0, i
		for ; j < len(s); j++ {
			if s[j] == '(' {
				depth++
			} else if s[j] == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if j >= len(s) {
			out.WriteString(s[i:])
			break
		}
		inner := s[i+1 : j]
		if lower := strings.ToLower(strings.TrimSpace(inner)); strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with") {
			subs = append(subs, inner)
			out.WriteString("(subquery)")
		} else {
			// Not a subquery, but one may hide inside (IN (SELECT ...), COALESCE((SELECT ...))).
			flat, inSubs := extractSubqueries(inner)
			subs = append(subs, inSubs...)
			out.WriteString("(" + flat + ")")
		}
		i = j + 1
	}
	return out.String(), subs
}

var (
	setOpRe      = regexp.MustCompile(`(?i)\b(?:union(?:\s+all)?|intersect|except)\b`)
	fromRe       = regexp.MustCompile(`(?i)\bfrom\b`)
	fromEndRe    = regexp.MustCompile(`(?i)\b(?:where|group|order|limit|offset|having|returning|window|for)\b|;`)
	whereRe      = regexp.MustCompile(`(?i)\bwhere\b`)
	whereEndRe   = regexp.MustCompile(`(?i)\b(?:group\s+by|order\s+by|limit|offset|having|returning|window|for)\b|;`)
	andRe        = regexp.MustCompile(`(?i)\band\b`)
	orRe         = regexp.MustCompile(`(?i)\bor\b`)
	insertHeadRe = regexp.MustCompile(`(?i)^\s*insert\s+into\s+(?:"?app"?\.)?"?([a-z_][a-z0-9_]*)`)
	selectRe     = regexp.MustCompile(`(?i)\bselect\b`)
	wordRe       = regexp.MustCompile(`^\s*(?:"?app"?\.)?"?([a-z_][a-z0-9_]*)`)
)

// analyze returns what is wrong with a statement (empty when it filters by company or touches no company table).
func analyze(sql string, company map[string]bool) []string {
	sql = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
	flat, subs := extractSubqueries(sql)
	var out []string
	for _, sub := range subs {
		out = append(out, analyze(sub, company)...)
	}
	masked := mask(flat)
	start := 0
	for _, loc := range append(setOpRe.FindAllStringIndex(masked, -1), []int{len(flat), len(flat)}) {
		out = append(out, analyzeBranch(flat[start:loc[0]], company)...)
		start = loc[1]
	}
	return out
}

// analyzeBranch checks one SELECT/UPDATE/DELETE/INSERT with no set operators and no nested subqueries.
func analyzeBranch(b string, company map[string]bool) []string {
	m := mask(b)
	if h := insertHeadRe.FindStringSubmatch(m); h != nil {
		return analyzeInsert(b, m, h[1], company)
	}
	tables := referencedTables(m, company)
	if len(tables) == 0 {
		return nil
	}
	onlyTenants := true
	for _, t := range tables {
		onlyTenants = onlyTenants && t == "tenants"
	}
	pred, want := tenantPredRe, "tenant_id = @tenant_id"
	if onlyTenants {
		pred, want = tenantIDPredRe, "id = @tenant_id"
	}
	loc := whereRe.FindStringIndex(m)
	if loc == nil {
		return []string{fmt.Sprintf("touches %s without a WHERE (%s)", strings.Join(tables, ", "), want)}
	}
	end := len(b)
	if e := whereEndRe.FindStringIndex(m[loc[1]:]); e != nil {
		end = loc[1] + e[0]
	}
	leaves, orGroups := conjuncts(b[loc[1]:end])
	for _, g := range orGroups {
		if pred.MatchString(g) || tenantArgRe.MatchString(g) {
			return []string{fmt.Sprintf("the tenant predicate of the query on %s is inside an OR (%q): it does not restrict anything", strings.Join(tables, ", "), strings.TrimSpace(g))}
		}
	}
	for _, leaf := range leaves {
		if pred.MatchString(leaf) {
			return nil
		}
	}
	return []string{fmt.Sprintf("touches %s and its WHERE lacks a top-level %s", strings.Join(tables, ", "), want)}
}

// analyzeInsert: an INSERT into a company table must pass @tenant_id; an INSERT ... SELECT must also
// filter the tables it reads.
func analyzeInsert(b, m, target string, company map[string]bool) []string {
	var out []string
	if company[target] && !tenantArgRe.MatchString(b) {
		out = append(out, fmt.Sprintf("INSERT into %s does not take @tenant_id", target))
	}
	if loc := selectRe.FindStringIndex(m); loc != nil {
		out = append(out, analyzeBranch(b[loc[0]:], company)...)
	}
	return out
}

// referencedTables lists the company tables a masked statement reads or writes: after FROM (including
// a comma list), JOIN, INTO and UPDATE.
func referencedTables(m string, company map[string]bool) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		name = strings.ToLower(name)
		if company[name] && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, r := range tableRefRe.FindAllStringSubmatch(m, -1) {
		add(r[1])
	}
	for _, loc := range fromRe.FindAllStringIndex(m, -1) {
		list := m[loc[1]:]
		if e := fromEndRe.FindStringIndex(list); e != nil {
			list = list[:e[0]]
		}
		for _, item := range strings.Split(list, ",")[min(1, len(strings.Split(list, ","))):] {
			if w := wordRe.FindStringSubmatch(item); w != nil {
				add(w[1])
			}
		}
	}
	sort.Strings(out)
	return out
}

// conjuncts splits a WHERE expression into what must all hold, looking through redundant parentheses.
// AND binds tighter than OR, so an OR at the top level makes the whole expression a disjunction, whatever
// surrounds it: `a OR b AND c` is `a OR (b AND c)` and `a AND b OR c` is `(a AND b) OR c`, and in neither
// does the predicate on the company restrict the result. Such an expression comes back as one orGroup.
// Without a top-level OR the expression is split at its ANDs, and each part is analysed in turn (a part
// may be a parenthesised OR). leaves are the conditions that have no OR at their top level.
func conjuncts(expr string) (leaves, orGroups []string) {
	expr = strings.TrimSpace(expr)
	for len(expr) > 1 && expr[0] == '(' && closingParen(expr) == len(expr)-1 {
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}
	m := mask(expr)
	if orRe.MatchString(m) {
		return nil, []string{expr}
	}
	if ands := andRe.FindAllStringIndex(m, -1); len(ands) > 0 {
		start := 0
		for _, loc := range append(ands, []int{len(expr), len(expr)}) {
			l, o := conjuncts(expr[start:loc[0]])
			leaves, orGroups = append(leaves, l...), append(orGroups, o...)
			start = loc[1]
		}
		return leaves, orGroups
	}
	return []string{expr}, nil
}

// closingParen returns the index of the parenthesis that closes the one at s[0].
func closingParen(s string) int {
	depth := 0
	for i := range len(s) {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
