package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Route registration stays in app's composition root. A registration added only to serve would
// escape chi.Walk in T-B801; reject that drift before the API can silently gain an uncovered route.
func TestServeUsesSharedAPIRouter(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "serve.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	builds := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if receiver.Name == "app" && selector.Sel.Name == "BuildAPIRouter" {
			builds++
		}
		if receiver.Name == "api" {
			t.Errorf("serve.go must register routes in app.BuildAPIRouter: api.%s", selector.Sel.Name)
		}
		for _, arg := range call.Args {
			if id, ok := arg.(*ast.Ident); ok && id.Name == "api" {
				t.Errorf("serve.go must not pass its router to another route registrar: %s.%s", receiver.Name, selector.Sel.Name)
			}
		}
		return true
	})
	if builds != 1 {
		t.Errorf("serve.go must use app.BuildAPIRouter once: got %d", builds)
	}
}
