package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const compositionPackage = "github.com/fdelillo/crm/internal/app"

// Check every production source file, including code outside serve.go. Local identifiers use
// parser-resolved declaration objects; imports resolve to ImportSpec, respecting aliases and
// local shadowing. No decision depends on variables being named api, root, srv or app.
func TestServeUsesSharedAPIRouter(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	parents := map[ast.Node]ast.Node{}
	files := map[ast.Node]*ast.File{}
	//nolint:staticcheck // PR #16 explicitly permits go/ast declaration objects for local value-use checks.
	references := map[*ast.Object][]*ast.Ident{}
	var sources []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		sources = append(sources, file)
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) > 0 {
				parents[n] = stack[len(stack)-1]
			}
			files[n] = file
			if id, ok := n.(*ast.Ident); ok && id.Obj != nil {
				references[id.Obj] = append(references[id.Obj], id)
			}
			stack = append(stack, n)
			return true
		})
	}
	report := func(n ast.Node, message string) { t.Helper(); t.Errorf("%s: %s", fset.Position(n.Pos()), message) }
	importedSelector := func(expr ast.Expr, path, name string) bool {
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != name {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return false
		}
		var declaration *ast.ImportSpec
		if id.Obj != nil {
			declaration, _ = id.Obj.Decl.(*ast.ImportSpec)
		} else {
			for _, spec := range files[id].Imports {
				p, _ := strconv.Unquote(spec.Path.Value)
				alias := filepath.Base(p)
				if spec.Name != nil {
					alias = spec.Name.Name
				}
				if alias == id.Name {
					declaration = spec
					break
				}
			}
		}
		if declaration == nil {
			return false
		}
		p, _ := strconv.Unquote(declaration.Path.Value)
		return p == path
	}
	isCall := func(n ast.Node, path, name string) bool {
		call, ok := n.(*ast.CallExpr)
		return ok && importedSelector(call.Fun, path, name)
	}
	apiField := func(n ast.Node) bool {
		field, ok := parents[n].(*ast.KeyValueExpr)
		if !ok || field.Value != n {
			return false
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok || key.Name != "API" {
			return false
		}
		literal, ok := parents[field].(*ast.CompositeLit)
		return ok && importedSelector(literal.Type, compositionPackage, "RootDeps")
	}
	serverArgument := func(n ast.Node) bool {
		call, ok := parents[n].(*ast.CallExpr)
		return ok && isCall(call, compositionPackage, "NewServer") && len(call.Args) == 2 && call.Args[1] == n
	}
	isLocal := func(n ast.Node) bool {
		for n != nil {
			switch n.(type) {
			case *ast.FuncDecl, *ast.FuncLit:
				return true
			}
			n = parents[n]
		}
		return false
	}
	// A factory result may be consumed directly or through one unmodified local variable.
	// Additional references include aliases, calls to local functions and assignment targets.
	singleConsumer := func(call *ast.CallExpr, allowed func(ast.Node) bool, destination string) {
		if allowed(call) {
			return
		}
		var assigned *ast.Ident
		switch declaration := parents[call].(type) {
		case *ast.AssignStmt:
			if len(declaration.Rhs) == 1 && len(declaration.Lhs) == 1 {
				assigned, _ = declaration.Lhs[0].(*ast.Ident)
			}
		case *ast.ValueSpec:
			if len(declaration.Values) == 1 && len(declaration.Names) == 1 {
				assigned = declaration.Names[0]
			}
		}
		if assigned == nil || assigned.Obj == nil {
			report(call, "factory result must directly reach "+destination)
			return
		}
		declaration, ok := assigned.Obj.Decl.(ast.Node)
		if !ok || !isLocal(declaration) {
			report(assigned, "factory result must use a local declaration")
			return
		}
		declarationNames := map[*ast.Ident]bool{assigned: true}
		switch decl := declaration.(type) {
		case *ast.AssignStmt:
			for _, lhs := range decl.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Obj == assigned.Obj {
					declarationNames[id] = true
				}
			}
		case *ast.ValueSpec:
			for _, id := range decl.Names {
				if id.Obj == assigned.Obj {
					declarationNames[id] = true
				}
			}
		default:
			report(assigned, "factory result cannot reassign a parameter")
			return
		}
		var uses []*ast.Ident
		for _, id := range references[assigned.Obj] {
			if !declarationNames[id] {
				uses = append(uses, id)
			}
		}
		if len(uses) != 1 {
			report(assigned, "factory variable must have exactly one use in "+destination)
			for _, use := range uses {
				if !allowed(use) {
					report(use, "extra factory-variable use outside "+destination)
				}
			}
			return
		}
		if !allowed(uses[0]) {
			report(uses[0], "factory result must directly reach "+destination)
		}
	}
	var builders, roots []*ast.CallExpr
	for _, file := range sources {
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			if path == "github.com/go-chi/chi/v5" || strings.HasPrefix(path, "github.com/go-chi/chi/v5/") {
				report(spec, "cmd/crm cannot import chi; register API routes in app.BuildAPIRouter")
			}
			if spec.Name != nil && spec.Name.Name == "." && (path == compositionPackage || path == "net/http") {
				report(spec, "composition and HTTP imports need explicit package bindings")
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				switch {
				case isCall(call, compositionPackage, "BuildAPIRouter"):
					builders = append(builders, call)
					singleConsumer(call, apiField, "app.RootDeps.API")
				case isCall(call, compositionPackage, "NewRootHandler"):
					roots = append(roots, call)
					singleConsumer(call, serverArgument, "app.NewServer's handler argument")
				case isCall(call, "net/http", "NewServeMux"), isCall(call, "net/http", "Handle"), isCall(call, "net/http", "HandleFunc"):
					report(call, "cmd/crm cannot register HTTP routes outside app's composition root")
				}
			}
			if assignment, ok := n.(*ast.AssignStmt); ok {
				for _, lhs := range assignment.Lhs {
					if field, ok := lhs.(*ast.SelectorExpr); ok && field.Sel.Name == "Handler" {
						report(field, "cmd/crm cannot replace the server's Handler")
					}
				}
			}
			return true
		})
	}
	if len(sources) == 0 {
		t.Fatal("cmd/crm has no production Go source")
	}
	if len(builders) != 1 {
		report(sources[0], "cmd/crm must call app.BuildAPIRouter exactly once")
		for _, call := range builders {
			report(call, "API factory call")
		}
	}
	if len(roots) == 0 {
		report(sources[0], "cmd/crm must pass app.NewRootHandler to app.NewServer")
	}
}
