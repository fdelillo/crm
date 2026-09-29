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
	// internal/platform/db (INV-04, ADR-003).
	RuleDynamicSQL = "dynamic-sql"
	// RuleChiOutsideAPI: the chi router of the API never registers the SPA: no Mount("/"), no
	// NotFound towards the SPA and no "/*" (INV-22, DD-22).
	RuleChiOutsideAPI = "chi-outside-api"
)

// Violation is one broken rule at a place in the tree.
type Violation struct {
	Rule   string
	File   string // relative to the root given to Check, slash-separated
	Line   int
	Detail string
}

var (
	roleRe = regexp.MustCompile(`(?i)(\bset\s+(?:(?:local|session)\s+)?role\b|\breset\s+role\b|\bset_config\s*\(\s*'role')`)
	// sqlRe matches the shape of a SQL statement in a string assembled from parts. Non-constant
	// parts are replaced by \x00 first, so "UPDATE " + table + " SET x = 1" still looks like an UPDATE.
	sqlRe = regexp.MustCompile(`(?is)^\s*(select\s|insert\s+into\s|update\s+\S+\s+set\s|delete\s+from\s|with\s+\S+\s+as\s*\()`)
)

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
		if m := roleRe.FindString(line); m != "" {
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
	c := &goChecker{fset: fset, rel: rel, sqlRules: !exemptFromSQLRules(rel), seen: map[*ast.BinaryExpr]bool{}}
	ast.Inspect(file, c.visit)
	return c.out, nil
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

type goChecker struct {
	fset     *token.FileSet
	rel      string
	sqlRules bool
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
	case *ast.BinaryExpr:
		if c.sqlRules {
			c.checkConcat(n)
		}
	case *ast.CallExpr:
		if c.sqlRules {
			c.checkSprintf(n)
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
	loc := roleRe.FindStringIndex(val)
	if loc == nil {
		return
	}
	line := c.fset.Position(lit.Pos()).Line + strings.Count(val[:loc[0]], "\n")
	c.out = append(c.out, Violation{RuleRoleSwitch, c.rel, line,
		fmt.Sprintf("%q outside internal/platform/db", val[loc[0]:loc[1]])})
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
	if hasDynamic && sqlRe.MatchString(template(b)) {
		c.add(RuleDynamicSQL, b.Pos(), "SQL built by concatenation")
	}
}

func (c *goChecker) checkSprintf(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprintf" || len(call.Args) < 2 {
		return
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "fmt" {
		return
	}
	if sqlRe.MatchString(template(call.Args[0])) {
		c.add(RuleDynamicSQL, call.Pos(), "SQL built with fmt.Sprintf")
	}
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
