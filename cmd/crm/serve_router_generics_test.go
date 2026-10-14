package main

import (
	"go/token"
	"strings"
	"testing"
)

func TestRouterRevision15TypeParameterMutations(t *testing.T) {
	fset := token.NewFileSet()
	imp := compositionImporter(t, fset)
	envStruct := revision14EnvStruct(t, compositionSources(t)["main.go"])
	listenType := "func(context.Context, string, string) (net.Listener, error)"
	tagged := strings.Replace(envStruct, listenType, listenType+` "x"`, 1)
	shape := strings.Replace(envStruct, listenType, "F", 1)
	for _, tc := range []struct {
		name, declaration, statement, structMutation string
		imports                                      []string
	}{
		{"r-type-parameter-literal", "func relisten[T env](e env) T { return T{e.getenv, e.stdout, e.stderr, (&net.ListenConfig{}).Listen} }", "e = relisten[env](e)", "", []string{"net"}},
		{"s-type-parameter-conversion", "func conv[D env](s " + tagged + ") D { return D(s) }", "var s " + tagged + "\ns.listen = (&net.ListenConfig{}).Listen\ne = conv[env](s)", tagged, []string{"net", "io"}},
		{"t-generic-alias", "type shape[F any] = " + shape, "var s shape[" + listenType + "]\ns.listen = (&net.ListenConfig{}).Listen\ne = s", shape, []string{"net", "io"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.structMutation != "" && tc.structMutation == envStruct {
				t.Fatal("mutation did not change sources: listenType in env struct")
			}
			sources := compositionSources(t)
			original := sources["serve.go"]
			source := strings.Replace(original, "fs := flag.NewFlagSet", tc.statement+"\nfs := flag.NewFlagSet", 1)
			if original == source {
				t.Fatal("mutation did not change sources: fs := flag.NewFlagSet")
			}
			for _, path := range tc.imports {
				original := source
				source = strings.Replace(source, `"fmt"`, "\"fmt\"\n\""+path+"\"", 1)
				if original == source {
					t.Fatal("mutation did not change sources: import anchor")
				}
			}
			sources["serve.go"] = source + "\n" + tc.declaration + "\n"
			vs, err := checkRouterComposition(fset, imp, sources)
			if err != nil {
				t.Fatalf("mutation must type check without adjustment: %v", err)
			}
			t.Logf("violations=%d", len(vs))
			found := false
			for _, v := range vs {
				if v.rule == 4 && v.message == "cmd/crm cannot declare type parameters" {
					if v.position.Filename != "serve.go" || v.position.Line <= 0 {
						t.Fatalf("missing mutation file/line: %v", v)
					}
					t.Log(v.String())
					found = true
				}
			}
			if !found {
				t.Errorf("mutation escaped type-parameter clause of rule (4): %v", vs)
			}
		})
	}
}

func TestRouterRevision15ExternalGenericInstantiation(t *testing.T) {
	fset := token.NewFileSet()
	imp := compositionImporter(t, fset)
	sources := compositionSources(t)
	source := strings.Replace(sources["serve.go"], `"fmt"`, "\"fmt\"\n\"slices\"", 1)
	sources["serve.go"] = strings.Replace(source, "fs := flag.NewFlagSet", "_ = slices.Index([]string{}, \"\")\nfs := flag.NewFlagSet", 1)
	assertValidComposition(t, fset, imp, sources)
}
