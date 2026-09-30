// Package reporules checks source-tree rules that the compiler cannot: they protect the tenant
// isolation invariants (INV-03, INV-04) and the API/SPA split (INV-22). It is used by a test that
// runs on the real tree and on fixtures under testdata/.
package reporules

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Rule names, reported in Violation.Rule.
const (
	// RuleRoleSwitch: SET ROLE, SET LOCAL ROLE, RESET ROLE and set_config('role' exist only in
	// internal/platform/db, where the role of a transaction is decided (INV-03, ADR-005).
	RuleRoleSwitch = "role-switch"
	// RuleDynamicSQL: SQL is never built with fmt.Sprintf or string concatenation outside
	// internal/platform/db (INV-04, ADR-003): Sprintf/Fprintf/concatenation/+=/builders/Join/Replace, and
	// any Exec/Query/QueryRow/Batch.Queue whose SQL argument is not a string literal or a const.
	RuleDynamicSQL = "dynamic-sql"
	// RuleChiOutsideAPI: the chi router of the API never registers the SPA: no Mount("/"), no
	// NotFound towards the SPA and no "/*" (INV-22, DD-22).
	RuleChiOutsideAPI = "chi-outside-api"
	// RuleClientIPHeaders: the proxy headers X-Forwarded-For, Forwarded and X-Real-IP are read only in
	// internal/platform/httpx (ClientIP), so an untrusted client cannot choose the IP that logs, the rate
	// limit and the audit trail record (INV-25, DD-32).
	RuleClientIPHeaders = "client-ip-header"
)

// Violation is one broken rule at a place in the tree.
type Violation struct {
	Rule   string
	File   string // relative to the root given to Check, slash-separated
	Line   int
	Detail string
}

var (
	roleRe = regexp.MustCompile(`(?i)(\bset\s+(?:(?:local|session)\s+)?role\b|\breset\s+role\b)`)
	// setConfigRe finds set_config( calls; what follows the parenthesis decides (see setConfigSwitch).
	setConfigRe = regexp.MustCompile(`(?i)\bset_config\s*\(\s*`)

	// SQL detection works on a "template": string literals joined, with \x00 for every non-constant
	// part, so "UPDATE " + table + " SET x = 1" still reads as an UPDATE.
	//
	// stmtStartRe: a statement at the start (case-insensitive), after optional comments.
	stmtStartRe = regexp.MustCompile(`(?is)^\s*(?:/\*.*?\*/\s*|--[^\n]*\n\s*)*(?:select\s|insert\s+into\s|update\s+\S+\s+set\s|delete\s+from\s|with\s+\S+\s+as\s*\()`)
	// stmtAnyRe: an UPPERCASE statement anywhere. Case-sensitive on purpose, so a lowercase message such
	// as "failed to delete from cache" is not taken for SQL.
	stmtAnyRe = regexp.MustCompile(`\b(?:SELECT\s|INSERT\s+INTO\s|UPDATE\s+\S+\s+SET\s|DELETE\s+FROM\s|WITH\s+\S+\s+AS\s*\()`)
	// fragRe: clauses that only appear in SQL, for statements built in several steps (q += " AND x = ...").
	fragRe = regexp.MustCompile(`(?i:\bwhere\s+\S+\s*(?:=|<|>|\blike\b|\bin\b|\bis\b)|\border\s+by\s|\bgroup\s+by\s|\bvalues\s*\(|\bset\s+[\w."]+\s*=|\bjoin\s+[\w."]+\s+on\b)|\b(?:AND|OR)\s+[\w."\x00]+\s*(?:=|<>|<|>|LIKE\b|IN\b)`)
)

func looksLikeSQL(template string) bool {
	return stmtStartRe.MatchString(template) || stmtAnyRe.MatchString(template) || fragRe.MatchString(template)
}

// roleSwitch reports the first place in s that changes the role of a transaction: SET [LOCAL|SESSION]
// ROLE, RESET ROLE, or set_config with 'role' / 'session_authorization' or with a GUC name that is not
// a literal (a parameter could carry "role"). It returns the byte offset and the matched text.
func roleSwitch(s string) (offset int, text string, ok bool) {
	best := -1
	if loc := roleRe.FindStringIndex(s); loc != nil {
		best, text = loc[0], s[loc[0]:loc[1]]
	}
	for _, loc := range setConfigRe.FindAllStringIndex(s, -1) {
		rest := s[loc[1]:]
		if strings.HasPrefix(rest, "'") {
			name, _, closed := strings.Cut(rest[1:], "'")
			if closed && !isRoleGUC(name) {
				continue
			}
		}
		if best == -1 || loc[0] < best {
			best, text = loc[0], strings.TrimSpace(s[loc[0]:loc[1]])+"…"
		}
	}
	return best, text, best >= 0
}

func isRoleGUC(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "role", "session_authorization":
		return true
	}
	return false
}

// Check scans internal/ and cmd/ under root. Test files, generated code and testdata are skipped;
// internal/platform/db is exempt from the role-switch and dynamic-SQL rules and so is
// internal/testsupport (test-only helpers). It fails if root has neither directory, so a wrong
// path cannot pass silently.
func Check(root string) ([]Violation, error) {
	var out []Violation
	scanned := false
	for _, sub := range []string{"internal", "cmd"} {
		dir := filepath.Join(root, sub)
		if _, err := os.Stat(dir); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		scanned = true
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "testdata", "node_modules", "vendor", ".git":
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			switch {
			case strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go"):
				vs, err := checkGo(path, rel)
				out = append(out, vs...)
				return err
			case strings.HasSuffix(path, ".sql"):
				vs, err := checkSQLFile(path, rel)
				out = append(out, vs...)
				return err
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if !scanned {
		return nil, fmt.Errorf("reporules: %q has neither internal/ nor cmd/", root)
	}
	return out, nil
}

// exemptFromSQLRules is true where role switching and dynamic SQL are allowed.
func exemptFromSQLRules(rel string) bool {
	return strings.HasPrefix(rel, "internal/platform/db/") || strings.HasPrefix(rel, "internal/testsupport/")
}

func checkSQLFile(path, rel string) ([]Violation, error) {
	if exemptFromSQLRules(rel) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Violation
	for i, line := range strings.Split(string(data), "\n") {
		if before, _, found := strings.Cut(line, "--"); found {
			line = before
		}
		if _, m, ok := roleSwitch(line); ok {
			out = append(out, Violation{RuleRoleSwitch, rel, i + 1, fmt.Sprintf("%q outside internal/platform/db", m)})
		}
	}
	return out, nil
}

func checkGo(path, rel string) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("reporules: parsing %s: %w", rel, err)
	}
	if isGenerated(file) {
		return nil, nil
	}
	consts, err := packageConsts(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	c := &goChecker{fset: fset, rel: rel, sqlRules: !exemptFromSQLRules(rel), ipRules: !strings.HasPrefix(rel, "internal/platform/httpx/") && !strings.HasPrefix(rel, "internal/testsupport/"),
		consts: consts, seen: map[*ast.BinaryExpr]bool{}}
	ast.Inspect(file, c.visit)
	return dedupe(c.out), nil
}

func isGenerated(f *ast.File) bool {
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "// Code generated") && strings.HasSuffix(c.Text, "DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}

// packageConsts lists the names declared with const in the Go files of dir (generated ones
// included): sqlc emits its queries as consts of the store package.
func packageConsts(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("reporules: parsing %s: %w", e.Name(), err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.CONST {
				for _, spec := range gd.Specs {
					for _, id := range spec.(*ast.ValueSpec).Names {
						names[id.Name] = true
					}
				}
			}
			return true
		})
	}
	return names, nil
}

// dedupe keeps one violation per rule and line (a call argument can trip two checks).
func dedupe(vs []Violation) []Violation {
	type key struct {
		rule string
		line int
	}
	seen := map[key]bool{}
	var out []Violation
	for _, v := range vs {
		if k := (key{v.Rule, v.Line}); !seen[k] {
			seen[k] = true
			out = append(out, v)
		}
	}
	return out
}

type goChecker struct {
	fset     *token.FileSet
	rel      string
	sqlRules bool
	ipRules  bool // false inside internal/platform/httpx (the only reader of the proxy headers) and test support
	consts   map[string]bool
	seen     map[*ast.BinaryExpr]bool // concatenation nodes already reported as part of a chain
	out      []Violation
}

func (c *goChecker) add(rule string, pos token.Pos, detail string) {
	c.out = append(c.out, Violation{rule, c.rel, c.fset.Position(pos).Line, detail})
}

func (c *goChecker) visit(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.BasicLit:
		if c.sqlRules && n.Kind == token.STRING {
			c.checkRoleLiteral(n)
		}
		if c.ipRules && n.Kind == token.STRING {
			c.checkProxyHeader(n)
		}
	case *ast.BinaryExpr:
		if c.sqlRules {
			c.checkConcat(n)
		}
	case *ast.CallExpr:
		if c.sqlRules {
			c.checkFormatCall(n)
			c.checkStringBuilding(n)
			c.checkSQLCall(n)
		}
		c.checkRouterCall(n)
	}
	return true
}

func (c *goChecker) checkRoleLiteral(lit *ast.BasicLit) {
	val, err := strconv.Unquote(lit.Value)
	if err != nil {
		return
	}
	offset, text, ok := roleSwitch(val)
	if !ok {
		return
	}
	line := c.fset.Position(lit.Pos()).Line + strings.Count(val[:offset], "\n")
	c.out = append(c.out, Violation{RuleRoleSwitch, c.rel, line,
		fmt.Sprintf("%q outside internal/platform/db", text)})
}

var proxyHeaders = []string{"x-forwarded-for", "forwarded", "x-real-ip"}

// checkProxyHeader flags a string literal that is exactly one of the proxy headers.
func (c *goChecker) checkProxyHeader(lit *ast.BasicLit) {
	val, err := strconv.Unquote(lit.Value)
	if err != nil {
		return
	}
	for _, h := range proxyHeaders {
		if strings.EqualFold(strings.TrimSpace(val), h) {
			c.add(RuleClientIPHeaders, lit.Pos(), fmt.Sprintf("%q is read outside internal/platform/httpx: the client IP comes only from httpx.ClientIP", val))
			return
		}
	}
}

// template renders an expression made of string literals and "+" with \x00 for anything else.
func template(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if s, err := strconv.Unquote(e.Value); err == nil {
				return s
			}
		}
	case *ast.ParenExpr:
		return template(e.X)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return template(e.X) + template(e.Y)
		}
	}
	return "\x00"
}

func (c *goChecker) checkConcat(b *ast.BinaryExpr) {
	if b.Op != token.ADD || c.seen[b] {
		return
	}
	// Mark the whole chain so only its outermost node is judged.
	hasDynamic := false
	var mark func(e ast.Expr)
	mark = func(e ast.Expr) {
		switch e := e.(type) {
		case *ast.BinaryExpr:
			if e.Op == token.ADD {
				c.seen[e] = true
				mark(e.X)
				mark(e.Y)
				return
			}
			hasDynamic = true
		case *ast.ParenExpr:
			mark(e.X)
		case *ast.BasicLit:
			if e.Kind != token.STRING {
				hasDynamic = true
			}
		default:
			hasDynamic = true
		}
	}
	mark(b)
	if hasDynamic && looksLikeSQL(template(b)) {
		c.add(RuleDynamicSQL, b.Pos(), "SQL built by concatenation")
	}
}

// checkFormatCall flags fmt.Sprintf / fmt.Fprintf whose format string is SQL and that has arguments.
func (c *goChecker) checkFormatCall(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !isPkg(sel.X, "fmt") {
		return
	}
	format := 0
	switch sel.Sel.Name {
	case "Sprintf":
	case "Fprintf":
		format = 1
	default:
		return
	}
	if len(call.Args) > format+1 && looksLikeSQL(template(call.Args[format])) {
		c.add(RuleDynamicSQL, call.Pos(), "SQL built with fmt."+sel.Sel.Name)
	}
}

func isPkg(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

// checkStringBuilding flags the other ways of assembling SQL text: a builder's WriteString,
// strings.Join over a slice with non-constant parts, and strings.Replace(All) on a SQL literal.
func (c *goChecker) checkStringBuilding(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return
	}
	switch {
	case sel.Sel.Name == "WriteString":
		if looksLikeSQL(template(call.Args[0])) {
			c.add(RuleDynamicSQL, call.Pos(), "SQL assembled piece by piece with WriteString")
		}
	case isPkg(sel.X, "strings") && sel.Sel.Name == "Join":
		lit, ok := call.Args[0].(*ast.CompositeLit)
		if !ok {
			return
		}
		var parts []string
		dynamic := false
		for _, e := range lit.Elts {
			t := template(e)
			dynamic = dynamic || t == "\x00"
			parts = append(parts, t)
		}
		if dynamic && looksLikeSQL(strings.Join(parts, " ")) {
			c.add(RuleDynamicSQL, call.Pos(), "SQL built with strings.Join")
		}
	case isPkg(sel.X, "strings") && (sel.Sel.Name == "Replace" || sel.Sel.Name == "ReplaceAll") && len(call.Args) >= 3:
		if looksLikeSQL(template(call.Args[0])) && (template(call.Args[2]) == "\x00" || template(call.Args[1]) == "\x00") {
			c.add(RuleDynamicSQL, call.Pos(), "SQL built with strings."+sel.Sel.Name)
		}
	}
}

// sqlCallMethods are the pgx and database/sql methods whose SQL argument must be constant.
var sqlCallMethods = map[string]bool{
	"Exec": true, "Query": true, "QueryRow": true,
	"ExecContext": true, "QueryContext": true, "QueryRowContext": true,
}

// checkSQLCall: the SQL text passed to Exec / Query / QueryRow / Batch.Queue must be a string literal
// or a const (sqlc emits `const name = ...`), never a value computed at run time (INV-04, ADR-003).
// The SQL argument is the first one that is not a context. A selector (pkg.Const) is accepted: without
// type information it cannot be told from a constant.
func (c *goChecker) checkSQLCall(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name
	isQueue := name == "Queue" && strings.Contains(strings.ToLower(types.ExprString(sel.X)), "batch")
	if !sqlCallMethods[name] && !isQueue {
		return
	}
	args := call.Args
	for len(args) > 0 && isContextArg(args[0]) {
		args = args[1:]
	}
	if len(args) == 0 || c.isConstantSQL(args[0]) {
		return
	}
	c.add(RuleDynamicSQL, args[0].Pos(), fmt.Sprintf("%s: the SQL argument must be a string literal or a const", name))
}

func isContextArg(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident:
		return strings.HasSuffix(strings.ToLower(e.Name), "ctx") || strings.EqualFold(e.Name, "context")
	case *ast.SelectorExpr:
		return strings.HasSuffix(strings.ToLower(e.Sel.Name), "ctx")
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "Context", "Background", "TODO", "WithTimeout", "WithCancel", "WithDeadline", "WithValue":
				return true
			}
		}
	}
	return false
}

func (c *goChecker) isConstantSQL(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		return true // a non-string literal is not SQL; either way nothing is built at run time
	case *ast.Ident:
		return c.consts[e.Name]
	case *ast.ParenExpr:
		return c.isConstantSQL(e.X)
	case *ast.BinaryExpr:
		return e.Op == token.ADD && c.isConstantSQL(e.X) && c.isConstantSQL(e.Y)
	case *ast.SelectorExpr:
		return true
	}
	return false
}

var routeMethods = map[string]bool{
	"Get": true, "Post": true, "Put": true, "Patch": true, "Delete": true, "Head": true,
	"Options": true, "Connect": true, "Trace": true, "Handle": true, "HandleFunc": true,
	"Method": true, "MethodFunc": true, "Mount": true,
}

func (c *goChecker) checkRouterCall(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name
	switch {
	case name == "NotFound" || name == "MethodNotAllowed":
		for _, arg := range call.Args {
			if id := spaIdent(arg); id != "" {
				c.add(RuleChiOutsideAPI, call.Pos(), fmt.Sprintf("%s handled by %q: the SPA must not hang from the API router", name, id))
				return
			}
		}
	case name == "Mount" && len(call.Args) > 0:
		if p, ok := stringArg(call.Args[0]); ok && (p == "/" || p == "" || p == "/*") {
			c.add(RuleChiOutsideAPI, call.Pos(), `Mount at "/" would put the SPA (or anything) under the API router`)
		}
	case routeMethods[name] && len(call.Args) > 0:
		if p, ok := stringArg(call.Args[0]); ok && (p == "/*" || p == "*") {
			c.add(RuleChiOutsideAPI, call.Pos(), fmt.Sprintf("%s %q is a root wildcard in the API router", name, p))
		}
	}
}

func stringArg(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// spaIdent returns the first identifier in e that names the SPA or the web package, or "".
func spaIdent(e ast.Expr) string {
	found := ""
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && found == "" {
			l := strings.ToLower(id.Name)
			if strings.HasPrefix(l, "spa") || l == "web" {
				found = id.Name
			}
		}
		return found == ""
	})
	return found
}
