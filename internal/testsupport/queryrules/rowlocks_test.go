package queryrules

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

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
			if table == "users" && mode != "NO KEY UPDATE" {
				errors = append(errors, "DD-40: users must use FOR NO KEY UPDATE")
			}
			if table == "tenants" && (mode != "NO KEY UPDATE" || name != "LockUsersTenant" || file != "internal/identity/store/users.sql") {
				errors = append(errors, "DD-40: only LockUsersTenant may lock tenants, with FOR NO KEY UPDATE")
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
}

func TestR0IdentityRowLocks(t *testing.T) {
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
