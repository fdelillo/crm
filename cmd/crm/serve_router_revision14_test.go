package main

import (
	"go/token"
	"strings"
	"testing"
)

func revision14EnvStruct(t *testing.T, source string) string {
	t.Helper()
	start := strings.Index(source, "type env struct {")
	if start < 0 {
		t.Fatal("env declaration missing")
	}
	start += len("type env ")
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		t.Fatal("env struct end missing")
	}
	return source[start : start+end+2]
}

func TestRouterRevision14Rule4Mutations(t *testing.T) {
	fset := token.NewFileSet()
	imp := compositionImporter(t, fset)
	envStruct := revision14EnvStruct(t, compositionSources(t)["main.go"])
	listen := "(&net.ListenConfig{}).Listen"
	for _, tc := range []struct {
		name, file, old, replacement string
		imports                      []string
	}{
		{"j-promoted-field", "serve.go", "fs := flag.NewFlagSet", "w := struct{ env }{e}\nw.listen = " + listen + "\ne = w.env\nfs := flag.NewFlagSet", []string{"net"}},
		{"k-defined-env", "serve.go", "fs := flag.NewFlagSet", "type env2 env\ne2 := env2(e)\ne2.listen = " + listen + "\ne = env(e2)\nfs := flag.NewFlagSet", []string{"net"}},
		{"l-anonymous-conversion", "serve.go", "fs := flag.NewFlagSet", "e = env(" + envStruct + "{listen: " + listen + "})\nfs := flag.NewFlagSet", []string{"net", "io"}},
		{"m-new-expression", "main.go", listen, "new(net.ListenConfig{Control: nil}).Listen", nil},
		{"n-anonymous-assignment", "serve.go", "fs := flag.NewFlagSet", "var s " + envStruct + "\ns.listen = " + listen + "\ne = s\nfs := flag.NewFlagSet", []string{"net", "io"}},
		{"o-pointer-conversion", "serve.go", "fs := flag.NewFlagSet", "p := (*" + envStruct + ")(&e)\np.listen = " + listen + "\nfs := flag.NewFlagSet", []string{"net", "io"}},
		{"type-alias", "serve.go", "fs := flag.NewFlagSet", "type variant = env\nfs := flag.NewFlagSet", nil},
		{"pointer-struct-type", "serve.go", "fs := flag.NewFlagSet", "type variant *" + envStruct + "\nfs := flag.NewFlagSet", []string{"net", "io"}},
		{"constraint-struct", "serve.go", "fs := flag.NewFlagSet", "type variant interface{ ~" + envStruct + " }\nfs := flag.NewFlagSet", []string{"net", "io"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sources := compositionSources(t)
			original := sources[tc.file]
			sources[tc.file] = strings.Replace(original, tc.old, tc.replacement, 1)
			for _, path := range tc.imports {
				sources[tc.file] = strings.Replace(sources[tc.file], `"fmt"`, "\"fmt\"\n\""+path+"\"", 1)
			}
			if original == sources[tc.file] {
				t.Fatal("mutation did not change sources")
			}
			vs, err := checkRouterComposition(fset, imp, sources)
			if err != nil {
				t.Fatalf("mutation must type check: %v", err)
			}
			t.Logf("violations=%d", len(vs))
			found := false
			for _, v := range vs {
				if v.rule == 4 {
					if v.position.Filename == "" || v.position.Line == 0 {
						t.Fatal("missing file/line")
					}
					t.Log(v.String())
					found = true
				}
			}
			if !found {
				t.Errorf("mutation escaped rule (4): %v", vs)
			}
		})
	}
}

func TestRouterRevision14Rule6Mutations(t *testing.T) {
	fset := token.NewFileSet()
	imp := compositionImporter(t, fset)
	for _, tc := range []struct{ name, path, statement string }{
		{"p-unsafe-write", "unsafe", "off := unsafe.Sizeof(env{}) - unsafe.Sizeof((func(context.Context,string,string)(net.Listener,error))(nil))\n*(*func(context.Context,string,string)(net.Listener,error))(unsafe.Add(unsafe.Pointer(&e),off)) = (&net.ListenConfig{}).Listen"},
		{"q-reflect-field", "reflect", "_ = reflect.ValueOf(&e).Elem().FieldByName(\"listen\")"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sources := compositionSources(t)
			s := sources["serve.go"]
			s = strings.Replace(s, "fs := flag.NewFlagSet", tc.statement+"\nfs := flag.NewFlagSet", 1)
			s = strings.Replace(s, `"fmt"`, "\"fmt\"\n\""+tc.path+"\"", 1)
			if tc.path == "unsafe" {
				s = strings.Replace(s, `"fmt"`, "\"fmt\"\n\"net\"", 1)
			}
			sources["serve.go"] = s
			vs, err := checkRouterComposition(fset, imp, sources)
			if err != nil {
				t.Fatalf("mutation must type check: %v", err)
			}
			t.Logf("violations=%d", len(vs))
			found := false
			for _, v := range vs {
				if v.rule == 6 {
					if v.position.Filename == "" || v.position.Line == 0 {
						t.Fatal("missing file/line")
					}
					t.Log(v.String())
					found = true
				}
			}
			if !found {
				t.Errorf("mutation escaped rule (6): %v", vs)
			}
		})
	}
	t.Run("cgo-import", func(t *testing.T) {
		sources := compositionSources(t)
		sources["cgo.go"] = "package main\nimport \"C\"\n"
		vs, err := checkRouterComposition(fset, imp, sources)
		if err != nil || len(vs) != 1 || vs[0].rule != 6 || vs[0].position.Filename != "cgo.go" || vs[0].position.Line != 2 {
			t.Fatalf("cgo import lacks rule/file/line: %v, %v", vs, err)
		}
		t.Log(vs[0].String())
	})
}
