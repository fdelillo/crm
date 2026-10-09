package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const compositionPackage = "github.com/fdelillo/crm/internal/app"
const configurationPackage = "github.com/fdelillo/crm/internal/platform/config"

type routerViolation struct {
	position token.Position
	rule     int
	message  string
}

func (v routerViolation) String() string {
	return fmt.Sprintf("%s: rule (%d): %s", v.position, v.rule, v.message)
}

func compositionSources(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sources[path] = string(b)
	}
	return sources
}

func TestServeUsesSharedAPIRouter(t *testing.T) {
	fset := token.NewFileSet()
	imp := compositionImporter(t, fset)
	start := time.Now()
	violations, err := checkRouterComposition(fset, imp, compositionSources(t))
	t.Logf("router guard importer=gc duration=%s", time.Since(start))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations {
		t.Error(v.String())
	}
}

// Every decision uses types.Object identity, including assignment targets and field selections.
// A type-check error prevents analysis of the partially resolved package.
func checkRouterComposition(fset *token.FileSet, imp types.Importer, sources map[string]string) ([]routerViolation, error) {
	files := make([]*ast.File, 0, len(sources))
	parents := map[ast.Node]ast.Node{}
	for path, source := range sources {
		file, err := parser.ParseFile(fset, path, source, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) > 0 {
				parents[n] = stack[len(stack)-1]
			}
			stack = append(stack, n)
			return true
		})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("cmd/crm has no production sources")
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Implicits: map[ast.Node]types.Object{}}
	cfg := types.Config{Importer: imp}
	pkg, err := cfg.Check("github.com/fdelillo/crm/cmd/crm", fset, files, info)
	if err != nil {
		return nil, fmt.Errorf("router guard type check: %w", err)
	}
	envObject, ok := pkg.Scope().Lookup("env").(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("router guard: env type missing")
	}
	envType := envObject.Type()
	envStruct, ok := envType.Underlying().(*types.Struct)
	if !ok {
		return nil, fmt.Errorf("router guard: env must be a struct")
	}
	var listenField *types.Var
	for i := 0; i < envStruct.NumFields(); i++ {
		if field := envStruct.Field(i); field.Name() == "listen" {
			listenField = field
		}
	}
	if listenField == nil {
		return nil, fmt.Errorf("router guard: env.listen missing")
	}
	isEnv := func(typ types.Type) bool {
		if typ == nil {
			return false
		}
		typ = types.Unalias(typ)
		if ptr, ok := typ.(*types.Pointer); ok {
			typ = types.Unalias(ptr.Elem())
		}
		return types.Identical(typ, envType)
	}
	refs := map[types.Object][]*ast.Ident{}
	for id, obj := range info.Uses {
		refs[obj] = append(refs[obj], id)
	}
	var violations []routerViolation
	report := func(n ast.Node, rule int, message string) {
		violations = append(violations, routerViolation{fset.Position(n.Pos()), rule, message})
	}
	named := func(typ types.Type, path, name string) bool {
		typ = types.Unalias(typ)
		if ptr, ok := typ.(*types.Pointer); ok {
			typ = types.Unalias(ptr.Elem())
		}
		n, ok := typ.(*types.Named)
		return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == path && n.Obj().Name() == name
	}
	object := func(expr ast.Expr) types.Object {
		switch e := expr.(type) {
		case *ast.Ident:
			return info.Uses[e]
		case *ast.SelectorExpr:
			if sel := info.Selections[e]; sel != nil {
				return sel.Obj()
			}
			return info.Uses[e.Sel]
		}
		return nil
	}
	isFunc := func(expr ast.Expr, path, name string) bool {
		obj := object(expr)
		fn, ok := obj.(*types.Func)
		return ok && fn.Pkg() != nil && fn.Pkg().Path() == path && fn.Name() == name
	}
	isCall := func(n ast.Node, name string) bool {
		c, ok := n.(*ast.CallExpr)
		return ok && isFunc(c.Fun, compositionPackage, name)
	}
	// Starting from Uses closes indirect calls as well as forms we have never enumerated.
	protected := map[string]int{"BuildAPIRouter": 1, "NewRootHandler": 3, "NewServer": 3, "NewMetricsServer": 4, "Serve": 4}
	for id, obj := range info.Uses {
		fn, ok := obj.(*types.Func)
		if !ok || fn.Pkg() == nil || fn.Pkg().Path() != compositionPackage {
			continue
		}
		rule, ok := protected[fn.Name()]
		if !ok {
			continue
		}
		var use ast.Node = id
		if sel, ok := parents[id].(*ast.SelectorExpr); ok && sel.Sel == id {
			use = sel
		}
		call, ok := parents[use].(*ast.CallExpr)
		if !ok || call.Fun != use {
			report(use, 5, "app."+fn.Name()+" is allowed only as a direct call")
			report(use, rule, "indirect app."+fn.Name()+" use prevents verifying its composition")
		}
	}
	argument := func(n ast.Node, name string, index int) bool {
		call, ok := parents[n].(*ast.CallExpr)
		return ok && isCall(call, name) && len(call.Args) > index && call.Args[index] == n
	}
	local := func(n ast.Node) bool {
		for n != nil {
			switch n.(type) {
			case *ast.FuncDecl, *ast.FuncLit:
				return true
			}
			n = parents[n]
		}
		return false
	}
	binding := func(expr ast.Expr) (*ast.Ident, bool) {
		var id *ast.Ident
		switch parent := parents[expr].(type) {
		case *ast.AssignStmt:
			if parent.Tok == token.DEFINE && len(parent.Rhs) == 1 && len(parent.Lhs) > 0 {
				id, _ = parent.Lhs[0].(*ast.Ident)
			}
		case *ast.ValueSpec:
			if len(parent.Values) == 1 && len(parent.Names) > 0 {
				id = parent.Names[0]
			}
		}
		if id == nil || id.Name == "_" || info.Defs[id] == nil || !local(id) {
			return nil, false
		}
		_, ok := info.Defs[id].(*types.Var)
		return id, ok
	}
	consumer := func(expr ast.Expr, rule int, direct bool, allowed func(ast.Node) bool, destination string) *ast.Ident {
		if direct && allowed(expr) {
			return nil
		}
		id, ok := binding(expr)
		if !ok {
			report(expr, rule, "factory result requires a local declaration for "+destination)
			return nil
		}
		uses := refs[info.Defs[id]]
		if len(uses) != 1 {
			report(id, rule, "variable requires exactly one use in "+destination)
			for _, use := range uses {
				if !allowed(use) {
					report(use, rule, "extra variable use outside "+destination)
				}
			}
			return nil
		}
		if !allowed(uses[0]) {
			report(uses[0], rule, "value must directly reach "+destination)
			return nil
		}
		return uses[0]
	}
	apiField := func(n ast.Node) bool {
		kv, ok := parents[n].(*ast.KeyValueExpr)
		if !ok || kv.Value != n {
			return false
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "API" {
			return false
		}
		lit, ok := parents[kv].(*ast.CompositeLit)
		return ok && named(info.TypeOf(lit), compositionPackage, "RootDeps")
	}
	factories := map[string][]*ast.CallExpr{}
	var serves []*ast.CallExpr
	for node, obj := range info.Implicits {
		if _, ok := obj.(*types.Var); ok && named(obj.Type(), compositionPackage, "RootDeps") {
			report(node, 1, "cmd/crm cannot declare app.RootDeps parameters or results")
		}
	}
	for id, obj := range info.Defs {
		if obj == nil {
			continue
		}
		if _, ok := obj.(*types.Var); ok && named(obj.Type(), compositionPackage, "RootDeps") {
			report(id, 1, "cmd/crm cannot retain app.RootDeps or *app.RootDeps")
		}
	}
	for _, file := range files {
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			if path == "expvar" || path == "net/http" || strings.HasPrefix(path, "net/http/") || path == "github.com/go-chi/chi/v5" || strings.HasPrefix(path, "github.com/go-chi/chi/v5/") {
				report(spec, 2, "cmd/crm cannot import "+path)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CompositeLit:
				if named(info.TypeOf(node), compositionPackage, "RootDeps") && !argument(node, "NewRootHandler", 0) {
					report(node, 1, "RootDeps literal must directly be NewRootHandler's first argument")
				}
				if named(info.TypeOf(node), "net/http", "Server") {
					report(node, 3, "cmd/crm cannot construct http.Server")
				}
			case *ast.SelectorExpr:
				sel := info.Selections[node]
				if sel != nil && sel.Obj().Name() == "Handler" && named(sel.Recv(), "net/http", "Server") {
					report(node, 3, "cmd/crm cannot select http.Server.Handler")
					report(node, 4, "server must be used only by app.Serve")
				}
			case *ast.CallExpr:
				for _, name := range []string{"BuildAPIRouter", "NewRootHandler", "NewServer", "NewMetricsServer"} {
					if isFunc(node.Fun, compositionPackage, name) {
						factories[name] = append(factories[name], node)
					}
				}
				if isCall(node, "Serve") {
					serves = append(serves, node)
				}
			}
			return true
		})
	}
	for _, entry := range []struct {
		name string
		rule int
	}{{"BuildAPIRouter", 1}, {"NewRootHandler", 3}, {"NewServer", 3}, {"NewMetricsServer", 4}} {
		if len(factories[entry.name]) != 1 {
			report(files[0], entry.rule, "app."+entry.name+" must be called exactly once")
		}
	}
	if len(serves) != 2 {
		report(files[0], 4, "app.Serve must be called exactly twice")
	}
	for _, call := range factories["BuildAPIRouter"] {
		consumer(call, 1, true, apiField, "RootDeps.API")
	}
	for _, call := range factories["NewRootHandler"] {
		consumer(call, 3, true, func(n ast.Node) bool { return argument(n, "NewServer", 1) }, "NewServer handler")
	}
	approvedServers := map[ast.Expr]bool{}
	approvedListens := map[ast.Expr]bool{}
	isEnvListen := func(expr ast.Expr) bool {
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		selection := info.Selections[sel]
		return selection != nil && selection.Obj() == listenField
	}
	for _, entry := range []struct {
		name, addr string
		rule       int
	}{{"NewServer", "HTTPAddr", 3}, {"NewMetricsServer", "MetricsAddr", 4}} {
		for _, factory := range factories[entry.name] {
			use := consumer(factory, entry.rule, false, func(n ast.Node) bool { return argument(n, "Serve", 1) }, "app.Serve server")
			if use == nil {
				continue
			}
			approvedServers[use] = true
			call := parents[use].(*ast.CallExpr)
			if len(call.Args) != 4 {
				report(call, 4, "Serve requires its server/listener pair")
				continue
			}
			listener, ok := call.Args[2].(*ast.Ident)
			if !ok {
				report(call.Args[2], 4, "listener requires a local single-use variable")
				continue
			}
			var origin ast.Expr
			for id, obj := range info.Defs {
				if obj != nil && obj == info.Uses[listener] {
					switch decl := parents[id].(type) {
					case *ast.AssignStmt:
						if len(decl.Rhs) == 1 && decl.Lhs[0] == id {
							origin = decl.Rhs[0]
						}
					case *ast.ValueSpec:
						if len(decl.Values) == 1 && decl.Names[0] == id {
							origin = decl.Values[0]
						}
					}
				}
			}
			if origin == nil {
				report(listener, 4, "listener must be declared from Listen")
				continue
			}
			if consumer(origin, 4, false, func(n ast.Node) bool { return n == listener }, "matching app.Serve listener") == nil {
				continue
			}
			listen, ok := origin.(*ast.CallExpr)
			if !ok {
				report(origin, 4, "listener must come directly from env.listen")
				continue
			}
			if !isEnvListen(listen.Fun) || len(listen.Args) != 3 {
				report(listen, 4, "listener must come directly from env.listen")
				continue
			}
			approvedListens[listen.Fun] = true
			address, ok := listen.Args[2].(*ast.SelectorExpr)
			if !ok {
				report(listen, 4, "Listen address must directly select config.Config."+entry.addr)
				continue
			}
			selection := info.Selections[address]
			if selection == nil || !named(selection.Recv(), configurationPackage, "Config") || selection.Obj().Name() != entry.addr {
				report(address, 4, "listener/server pair requires config.Config."+entry.addr)
			}
		}
	}
	for _, call := range serves {
		if len(call.Args) != 4 || !approvedServers[call.Args[1]] {
			report(call, 4, "Serve must receive the single-use result of its matching server factory")
		}
	}
	initializations := 0
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				obj := info.Defs[node.Name]
				if obj != nil && obj != envObject && types.Identical(obj.Type().Underlying(), envStruct) {
					report(node, 4, "env is the only type declaration with its underlying struct")
				}
			case *ast.StructType:
				parent, original := parents[node].(*ast.TypeSpec)
				original = original && info.Defs[parent.Name] == envObject && parent.Type == node
				if !original && types.Identical(info.TypeOf(node), envStruct) {
					report(node, 4, "another struct has the same underlying type as env")
				}
			case *ast.CallExpr:
				if info.Types[node.Fun].IsType() && len(node.Args) == 1 && (isEnv(info.TypeOf(node.Fun)) || isEnv(info.TypeOf(node.Args[0]))) {
					report(node, 4, "conversions to or from env or *env are forbidden")
				}
			case *ast.SelectorExpr:
				if isEnvListen(node) && !approvedListens[node] {
					report(node, 4, "env.listen has an extra read or assignment")
				}
			case *ast.CompositeLit:
				for _, element := range node.Elts {
					kv, ok := element.(*ast.KeyValueExpr)
					if !ok {
						if isEnv(info.TypeOf(node)) {
							report(element, 4, "env must use keyed fields")
						}
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok {
						continue
					}
					field := info.Uses[key]
					if field != listenField {
						continue
					}
					initializations++
					var function *ast.FuncDecl
					for parent := parents[node]; parent != nil; parent = parents[parent] {
						if f, ok := parent.(*ast.FuncDecl); ok {
							function = f
							break
						}
					}
					valid := function != nil && function.Name.Name == "run"
					method, ok := kv.Value.(*ast.SelectorExpr)
					valid = valid && ok && isFunc(kv.Value, "net", "Listen")
					if ok {
						switch receiver := method.X.(type) {
						case *ast.ParenExpr:
							address, ok := receiver.X.(*ast.UnaryExpr)
							if !ok || address.Op != token.AND {
								valid = false
								break
							}
							literal, ok := address.X.(*ast.CompositeLit)
							valid = valid && ok && named(info.TypeOf(literal), "net", "ListenConfig") && len(literal.Elts) == 0
						case *ast.CallExpr:
							builtin, ok := object(receiver.Fun).(*types.Builtin)
							valid = valid && ok && builtin.Name() == "new" && len(receiver.Args) == 1 && info.Types[receiver.Args[0]].IsType() && named(info.TypeOf(receiver), "net", "ListenConfig")
						default:
							valid = false
						}
					}
					if !valid {
						report(kv, 4, "env.listen must be initialized in run with (&net.ListenConfig{}).Listen or new(net.ListenConfig).Listen")
					}
				}
			}
			return true
		})
	}
	if initializations != 1 {
		report(files[0], 4, "env.listen requires exactly one initialization in run")
	}
	return violations, nil
}

// The source importer was measured under -race in make check. Loading the application
// from source took about 31 s, twice per checkpoint; gc uses Go's own export files.
func compositionImporter(t *testing.T, fset *token.FileSet) types.Importer {
	t.Helper()
	exportsOnce.Do(func() {
		cmd := exec.CommandContext(context.Background(), "go", "list", "-export", "-deps", "-json", ".", "net/http/...")
		output, err := cmd.Output()
		if err != nil {
			exportsErr = fmt.Errorf("go list -export: %w", err)
			return
		}
		exports = map[string]string{}
		decoder := json.NewDecoder(bytes.NewReader(output))
		for {
			var p struct{ ImportPath, Export string }
			err := decoder.Decode(&p)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				exportsErr = err
				return
			}
			if p.Export != "" {
				exports[p.ImportPath] = p.Export
			}
		}
	})
	if exportsErr != nil {
		t.Fatal(exportsErr)
	}
	return importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		export, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export file for %s", path)
		}
		return os.Open(export)
	})
}

var (
	exportsOnce sync.Once
	exports     map[string]string
	exportsErr  error
)
