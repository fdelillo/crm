package main

import (
	"go/token"
	"strings"
	"testing"
)

func TestRouterRevision13Mutations(t *testing.T) {
	fset := token.NewFileSet()
	imp := compositionImporter(t, fset)
	for _, tc := range []struct {
		name                   string
		rules                  []int
		file, old, replacement string
	}{
		{"e-extra-metrics-function-value", []int{5, 4}, "serve.go", "fs := flag.NewFlagSet", "mk := app.NewMetricsServer\n_ = mk(config.Config{})\nfs := flag.NewFlagSet"},
		{"f-extra-serve-function-value", []int{5}, "serve.go", "fs := flag.NewFlagSet", "serve := app.Serve\n_ = serve(ctx, nil, nil, slog.Default())\nfs := flag.NewFlagSet"},
		{"g-parenthesized-serve", []int{5}, "serve.go", "fs := flag.NewFlagSet", "_ = (app.Serve)(ctx, nil, nil, slog.Default())\nfs := flag.NewFlagSet"},
		{"h-direct-listen", []int{4}, "serve.go", "e.listen(serverCtx, \"tcp\", cfg.HTTPAddr)", "net.Listen(\"tcp\", cfg.HTTPAddr)"},
		{"h-direct-listen-config", []int{4}, "serve.go", "e.listen(serverCtx, \"tcp\", cfg.HTTPAddr)", "(&net.ListenConfig{}).Listen(serverCtx, \"tcp\", cfg.HTTPAddr)"},
		{"i-reassign-listen", []int{4}, "serve.go", "fs := flag.NewFlagSet", "e.listen = (&net.ListenConfig{}).Listen\nfs := flag.NewFlagSet"},
		{"i-other-listen-in-run", []int{4}, "main.go", "listen: (&net.ListenConfig{}).Listen", "listen: func(ctx context.Context, network, address string) (net.Listener,error) { return (&net.ListenConfig{}).Listen(ctx,network,address) }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sources := compositionSources(t)
			original := sources[tc.file]
			sources[tc.file] = strings.Replace(original, tc.old, tc.replacement, 1)
			if tc.file == "serve.go" && strings.Contains(tc.replacement, "net.") {
				sources[tc.file] = strings.Replace(sources[tc.file], `"fmt"`, "\"fmt\"\n\"net\"", 1)
			}
			if original == sources[tc.file] {
				t.Fatal("mutation did not change sources")
			}
			vs, err := checkRouterComposition(fset, imp, sources)
			if err != nil {
				t.Fatalf("mutation must type check: %v", err)
			}
			t.Logf("violations=%d", len(vs))
			for _, rule := range tc.rules {
				found := false
				for _, v := range vs {
					if v.rule == rule {
						if v.position.Filename == "" || v.position.Line == 0 {
							t.Fatal("missing file/line")
						}
						t.Log(v.String())
						found = true
					}
				}
				if !found {
					t.Errorf("mutation escaped rule (%d): %v", rule, vs)
				}
			}
		})
	}
	t.Run("new-listen-config-valid", func(t *testing.T) {
		sources := compositionSources(t)
		sources["main.go"] = strings.Replace(sources["main.go"], "(&net.ListenConfig{}).Listen", "new(net.ListenConfig).Listen", 1)
		assertValidComposition(t, fset, imp, sources)
	})
}
