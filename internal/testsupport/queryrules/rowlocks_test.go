package queryrules

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Tables of the company configuration (spec 002): foreign key targets of later specs.
var configurationTables = map[string]bool{"stages": true, "categories": true, "catalog_items": true, "tenant_settings": true}

// Analyze each SELECT scope separately: an unqualified lock on a JOIN locks
// every relation, while OF limits it to the named aliases (DD-40).
func rowLockErrors(file, name, sql string) []string {
	flat, subs := extractSubqueries(sql)
	var errors []string
	for _, sub := range subs {
		errors = append(errors, rowLockErrors(file, name, sub)...)
	}
	refs := regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+(?:app\.)?(users|tenants|[a-z_][a-z0-9_]*)\b(?:\s+(?:AS\s+)?([a-z_][a-z0-9_]*))?`).FindAllStringSubmatch(flat, -1)
	locks := regexp.MustCompile(`(?i)\bFOR\s+(NO\s+KEY\s+UPDATE|KEY\s+SHARE|UPDATE|SHARE)(?:\s+OF\s+([a-z_][a-z0-9_]*(?:\s*,\s*[a-z_][a-z0-9_]*)*))?`).FindAllStringSubmatch(flat, -1)
	for _, lock := range locks {
		mode := strings.Join(strings.Fields(strings.ToUpper(lock[1])), " ")
		for _, ref := range refs {
			table, alias := strings.ToLower(ref[1]), strings.ToLower(ref[2])
			if strings.Contains(" where join left right inner outer on for order limit ", " "+alias+" ") || alias == "" {
				alias = table
			}
			selected := lock[2] == ""
			for _, target := range strings.Split(strings.ToLower(lock[2]), ",") {
				if strings.TrimSpace(target) == alias {
					selected = true
				}
			}
			if !selected {
				continue
			}
			if configurationTables[table] && mode != "NO KEY UPDATE" {
				errors = append(errors, "DD-40: configuration rows only FOR NO KEY UPDATE")
			}
			if table == "users" && mode != "NO KEY UPDATE" {
				errors = append(errors, "DD-40: users must use FOR NO KEY UPDATE")
			}
			tenantLockAllowed := name == "LockUsersTenant" && file == "internal/identity/store/users.sql" || name == "LockTenant" && file == "internal/tenant/store/queries.sql"
			if table == "tenants" && (mode != "NO KEY UPDATE" || !tenantLockAllowed) {
				errors = append(errors, "DD-40: only the named identity and tenant locks may lock tenants, with FOR NO KEY UPDATE")
			}
		}
	}
	return errors
}

func TestRowLockRuleScopes(t *testing.T) {
	for _, tc := range []struct {
		sql  string
		fail bool
	}{
		{`SELECT * FROM app.users FOR UPDATE`, true},
		{`SELECT * FROM app.users FOR SHARE`, true},
		{`SELECT * FROM app.users FOR KEY SHARE`, true},
		{`SELECT * FROM app.users FOR NO KEY UPDATE`, false},
		{`SELECT * FROM app.user_tokens t JOIN app.users u ON u.id=t.user_id FOR UPDATE`, true},
		{`SELECT * FROM app.user_tokens t JOIN app.users u ON u.id=t.user_id FOR UPDATE OF t`, false},
		{`SELECT * FROM app.user_tokens t JOIN app.users u ON u.id=t.user_id FOR UPDATE OF u`, true},
		{`SELECT * FROM app.user_tokens WHERE user_id IN (SELECT id FROM app.users) FOR UPDATE`, false},
		{`SELECT * FROM app.user_tokens WHERE user_id IN (SELECT id FROM app.users FOR UPDATE)`, true},
		{`SELECT * FROM app.tenants FOR NO KEY UPDATE`, true},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			if got := len(rowLockErrors("internal/example/store/queries.sql", "Example", tc.sql)) > 0; got != tc.fail {
				t.Fatalf("violations=%v", got)
			}
		})
	}
	if errs := rowLockErrors("internal/identity/store/users.sql", "LockUsersTenant", `SELECT id FROM app.tenants FOR NO KEY UPDATE`); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, mode := range []string{"UPDATE", "SHARE", "KEY SHARE", "NO KEY UPDATE"} {
		errs := rowLockErrors("internal/tenant/store/queries.sql", "LockTenant", `SELECT * FROM app.tenants WHERE id = @tenant_id FOR `+mode)
		if (len(errs) > 0) != (mode != "NO KEY UPDATE") {
			t.Fatalf("tenant mode %s: %v", mode, errs)
		}
	}
}

// T-002-B103 (002/INV-8, DD-40 of 001): stages, categories, catalog_items and tenant_settings are lock
// targets of foreign keys of later specs (or of the lock that serializes the configuration), so they are
// only locked FOR NO KEY UPDATE, in any query and any store file.
func TestConfigurationRowLocks(t *testing.T) {
	const message = "DD-40: configuration rows only FOR NO KEY UPDATE"
	for _, table := range []string{"stages", "categories", "catalog_items", "tenant_settings"} {
		for _, mode := range []string{"UPDATE", "SHARE", "KEY SHARE", "NO KEY UPDATE"} {
			sql := `SELECT id FROM app.` + table + ` WHERE tenant_id = @tenant_id FOR ` + mode
			errs := rowLockErrors("internal/settings/store/queries.sql", "LockSomething", sql)
			if mode == "NO KEY UPDATE" {
				if len(errs) != 0 {
					t.Errorf("%s FOR %s: violations %v, want none", table, mode, errs)
				}
				continue
			}
			if len(errs) != 1 || errs[0] != message {
				t.Errorf("%s FOR %s: violations %v, want [%q]", table, mode, errs, message)
			}
		}
	}
	for _, tc := range []struct {
		name string
		sql  string
		fail bool
	}{
		{"unqualified table", `SELECT id FROM stages FOR UPDATE`, true},
		{"alias with AS", `SELECT s.id FROM app.stages AS s FOR SHARE`, true},
		{"join locks both without OF", `SELECT c.id FROM app.categories c JOIN app.catalog_items i ON i.tenant_id = c.tenant_id FOR UPDATE`, true},
		{"join locking only another configuration table", `SELECT c.id FROM app.categories c JOIN app.stages s ON s.tenant_id = c.tenant_id FOR UPDATE OF s`, true},
		{"join with OF on a configuration table", `SELECT c.id FROM app.categories c JOIN app.users u ON u.tenant_id = c.tenant_id FOR NO KEY UPDATE OF c`, false},
		{"join with NO KEY UPDATE OF one of two configuration tables", `SELECT c.id FROM app.categories c JOIN app.stages s ON s.tenant_id = c.tenant_id FOR NO KEY UPDATE OF c`, false},
		{"configuration table inside a subquery", `SELECT 1 WHERE EXISTS (SELECT 1 FROM app.catalog_items FOR KEY SHARE)`, true},
		{"read without lock", `SELECT id FROM app.stages WHERE tenant_id = @tenant_id`, false},
		{"locking a different table with FOR UPDATE", `SELECT * FROM app.user_tokens WHERE user_id IN (SELECT id FROM app.stages) FOR UPDATE`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(rowLockErrors("internal/settings/store/queries.sql", "Example", tc.sql)) > 0; got != tc.fail {
				t.Fatalf("violations=%v, want %v", got, tc.fail)
			}
		})
	}
}

// DD-40's two tenant lock queries are allowlisted by name and owning SQL file.
func TestTenantLockAllowlist(t *testing.T) {
	for _, owner := range []struct{ file, name string }{
		{"internal/identity/store/users.sql", "LockUsersTenant"},
		{"internal/tenant/store/queries.sql", "LockTenant"},
	} {
		for _, mode := range []string{"NO KEY UPDATE", "UPDATE", "SHARE", "KEY SHARE"} {
			sql := "SELECT * FROM app.tenants FOR " + mode
			if got := len(rowLockErrors(owner.file, owner.name, sql)); (got == 0) != (mode == "NO KEY UPDATE") {
				t.Fatalf("%s %s: violations=%d", owner.name, mode, got)
			}
			if len(rowLockErrors(owner.file, "UnexpectedTenantLock", sql)) == 0 {
				t.Fatal("unnamed exception admitted a tenant lock")
			}
			if len(rowLockErrors("internal/other/store/queries.sql", owner.name, sql)) == 0 {
				t.Fatal("exception admitted a lock outside its owning store")
			}
		}
	}
}

// T-B604 R0 (DD-40): store queries take only the allowed row locks.
func TestStoreQueriesUseAllowedRowLocks(t *testing.T) {
	root := "../../.."
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(filepath.Dir(p)) != "store" || filepath.Ext(p) != ".sql" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		for _, q := range splitQueries(string(data)) {
			for _, detail := range rowLockErrors(filepath.ToSlash(rel), q.name, q.sql) {
				t.Errorf("%s:%d %s: %s", rel, q.line, q.name, detail)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
