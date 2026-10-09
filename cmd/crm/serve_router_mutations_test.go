package main

import (
	"go/token"
	"go/types"
	"regexp"
	"strings"
	"testing"
)

// Mutations are injected into copies of all production sources and checked by the same
// parser/type checker/guard. They never leave mutated files in the checkout.
func TestRouterGuardMutations(t *testing.T) {
	originals := compositionSources(t)
	source := originals["serve.go"]
	fset := token.NewFileSet()
	imp := compositionImporter(t, fset)
	addHTTP := func(s string) string { return strings.Replace(s, `"fmt"`, `"fmt"`+"\n\"net/http\"", 1) }
	wrap := func(s string) string { return addHTTP(s) + "\nfunc wrap(h http.Handler) http.Handler { return h }\n" }
	deps := func(s, edit string) string {
		s = strings.Replace(s, "root := app.NewRootHandler(app.RootDeps{", "deps := app.RootDeps{", 1)
		return strings.Replace(s, "}, app.NewCommonMiddleware(logger, cfg.IsLocal(), cfg.TrustedProxies))", "}\n"+edit+"\nroot := app.NewRootHandler(deps, app.NewCommonMiddleware(logger, cfg.IsLocal(), cfg.TrustedProxies))", 1)
	}
	for _, tc := range []struct {
		name   string
		rules  []int
		mutate func(string) string
		extra  string
	}{
		{"alias-router", []int{1}, func(s string) string {
			return strings.Replace(s, "root := app.NewRootHandler", "r := api\nr.Get(\"/api/v1/backdoor\", nil)\nroot := app.NewRootHandler", 1)
		}, ""},
		{"local-router-function", []int{1, 2}, func(s string) string {
			return strings.Replace(s, "root := app.NewRootHandler", "mutateAPI(api)\nroot := app.NewRootHandler", 1)
		}, "package main\nimport \"github.com/go-chi/chi/v5\"\nfunc mutateAPI(r chi.Router) { r.Get(\"/api/v1/backdoor\", nil) }\n"},
		{"wrapped-api", []int{1, 2}, func(s string) string {
			s = strings.Replace(s, "api := app.BuildAPIRouter(users, companies, c, logger)", "", 1)
			return wrap(strings.Replace(s, "API:       api,", "API: wrap(app.BuildAPIRouter(users, companies, c, logger)),", 1))
		}, ""},
		{"foreign-file-chi", []int{2}, func(s string) string { return s }, "package main\nimport \"github.com/go-chi/chi/v5\"\nvar hiddenAPI = chi.NewRouter()\n"},
		{"wrapped-root", []int{2, 3}, func(s string) string {
			return wrap(strings.Replace(s, "app.NewServer(cfg, root)", "app.NewServer(cfg, wrap(root))", 1))
		}, ""},
		{"retained-deps-http", []int{1, 2}, func(s string) string {
			return deps(addHTTP(s), `deps.API.(interface{ Get(string, http.HandlerFunc) }).Get("/api/v1/backdoor", nil)`)
		}, ""},
		{"retained-deps-no-http", []int{1}, func(s string) string { return deps(s, "deps.Liveness = deps.API") }, ""},
		{"different-server", []int{2, 3, 4}, func(s string) string {
			s = addHTTP(s)
			s = strings.Replace(s, "apiDone <- app.Serve(serverCtx, srv, ln, logger)", "_ = srv\napiDone <- app.Serve(serverCtx, &http.Server{Handler: app.SPAUnavailableHandler()}, ln, logger)", 1)
			return s
		}, ""},
		{"swapped-listeners", []int{4}, func(s string) string {
			// Move both Serve calls after both listeners exist so the mutation stays well typed.
			a := strings.Index(s, "\tgo func() {\n\t\tapiDone <- app.Serve")
			b := strings.Index(s[a:], "\n\tmetricsSrv :=") + a
			block := s[a:b]
			s = s[:a] + s[b:]
			pos := strings.Index(s, "\tmetricsDone :=")
			block = strings.Replace(block, "srv, ln, logger", "srv, metricsLn, logger", 1)
			s = s[:pos] + block + "\n" + s[pos:]
			return strings.Replace(s, "metricsSrv, metricsLn, logger.With", "metricsSrv, ln, logger.With", 1)
		}, ""},
		{"metrics-handler", []int{3, 4}, func(s string) string {
			return strings.Replace(s, "metricsSrv := app.NewMetricsServer(cfg)", "metricsSrv := app.NewMetricsServer(cfg)\nmetricsSrv.Handler = app.SPAUnavailableHandler()", 1)
		}, ""},
		{"unnamed-root-deps-result", []int{1}, func(s string) string { return s + "\nfunc retainedRoot() app.RootDeps { return app.RootDeps{} }\n" }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := map[string]string{}
			for name, text := range originals {
				mutated[name] = text
			}
			mutated["serve.go"] = tc.mutate(source)
			if tc.extra != "" {
				mutated["mutation.go"] = tc.extra
			}
			if mutated["serve.go"] == source && tc.extra == "" {
				t.Fatal("mutation did not modify source")
			}
			vs, err := checkRouterComposition(fset, imp, mutated)
			if err != nil {
				t.Fatalf("mutation must remain well typed: %v", err)
			}
			for _, rule := range tc.rules {
				found := false
				for _, v := range vs {
					if v.rule == rule {
						if v.position.Filename == "" || v.position.Line == 0 {
							t.Fatal("missing file/line")
						}
						found = true
						t.Log(v.String())
					}
				}
				if !found {
					t.Errorf("mutation escaped rule (%d): %v", rule, vs)
				}
			}
		})
	}
	t.Run("renamed-variables", func(t *testing.T) {
		renamed := regexp.MustCompile(`\b(api|root|srv|ln|metricsSrv|metricsLn)\b`).ReplaceAllStringFunc(source, func(s string) string { return "renamed_" + s })
		sources := map[string]string{}
		for name, text := range originals {
			sources[name] = text
		}
		sources["serve.go"] = renamed
		assertValidComposition(t, fset, imp, sources)
	})
	t.Run("app-import-alias", func(t *testing.T) {
		sources := map[string]string{}
		for name, text := range originals {
			sources[name] = text
		}
		s := strings.Replace(source, `"github.com/fdelillo/crm/internal/app"`, `composition "github.com/fdelillo/crm/internal/app"`, 1)
		sources["serve.go"] = strings.ReplaceAll(s, "app.", "composition.")
		assertValidComposition(t, fset, imp, sources)
	})
	t.Run("wiring-in-another-file", func(t *testing.T) {
		sources := map[string]string{}
		for name, text := range originals {
			if name != "serve.go" {
				sources[name] = text
			}
		}
		sources["wiring.go"] = source
		assertValidComposition(t, fset, imp, sources)
	})
	t.Run("type-error-fails-check", func(t *testing.T) {
		sources := map[string]string{}
		for name, text := range originals {
			sources[name] = text
		}
		sources["invalid.go"] = "package main\nvar invalid = undefinedObject\n"
		vs, err := checkRouterComposition(fset, imp, sources)
		if err == nil || len(vs) != 0 {
			t.Fatalf("partially resolved package accepted: %v %v", vs, err)
		}
	})
}

func assertValidComposition(t *testing.T, fset *token.FileSet, imp types.Importer, sources map[string]string) {
	t.Helper()
	vs, err := checkRouterComposition(fset, imp, sources)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		t.Error(v.String())
	}
}
