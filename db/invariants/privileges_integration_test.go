//go:build integration

package invariants_test

import (
	"context"
	"slices"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5"
)

// privilegeSnapshot lists the privileges a grantee holds on the tables of schema app, from the ACLs
// (not from has_*_privilege, which would fold table-level grants into every column):
// "table:PRIV" for table-level ones and "table.column:PRIV" for column-level ones.
func privilegeSnapshot(t *testing.T, grantee string) []string {
	t.Helper()
	return queryStrings(t, pgtest.SuperuserPool(t), `
		SELECT CASE WHEN n.nspname='public' THEN 'public.' ELSE '' END || c.relname || ':' || a.privilege_type
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace,
		     LATERAL aclexplode(c.relacl) a
		WHERE n.nspname IN ('app','public') AND c.relkind IN ('r', 'p') AND a.grantee = CASE WHEN $1 = 'PUBLIC' THEN 0 ELSE $1::regrole::oid END
		UNION ALL
		SELECT CASE WHEN n.nspname='public' THEN 'public.' ELSE '' END || c.relname || '.' || att.attname || ':' || a.privilege_type
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		     JOIN pg_attribute att ON att.attrelid = c.oid AND NOT att.attisdropped,
		     LATERAL aclexplode(att.attacl) a
		WHERE n.nspname IN ('app','public') AND c.relkind IN ('r', 'p') AND a.grantee = CASE WHEN $1 = 'PUBLIC' THEN 0 ELSE $1::regrole::oid END
 UNION ALL SELECT 'public:' || a.privilege_type FROM pg_namespace n, LATERAL aclexplode(n.nspacl) a
 WHERE n.nspname='public' AND a.grantee=CASE WHEN $1='PUBLIC' THEN 0 ELSE $1::regrole::oid END`, grantee)
}

// T-B109, INV-05: the privileges of the system roles are exactly those of data-model.md §3.4. A
// change here needs a change in the design (maintenance matrix, plan §16).
func TestCatalog_SystemRolePrivilegesAreExactlyTheDocumentedOnes(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		"crm_auth": {
			"public:USAGE", "public.goose_db_version.version_id:SELECT",
			"users.id:SELECT", "users.tenant_id:SELECT", "users.email:SELECT",
			"sessions.id:SELECT", "sessions.tenant_id:SELECT", "sessions.token_hash:SELECT",
			"user_tokens.id:SELECT", "user_tokens.tenant_id:SELECT", "user_tokens.token_hash:SELECT", "user_tokens.purpose:SELECT",
			"login_throttles:SELECT", "login_throttles:INSERT", "login_throttles:UPDATE", "login_throttles:DELETE",
		},
		"crm_worker": {
			"tenants.id:SELECT",
			"sessions.id:SELECT", "sessions.expires_at:SELECT", "sessions.revoked_at:SELECT", "sessions:DELETE",
			"user_tokens.id:SELECT", "user_tokens.purpose:SELECT", "user_tokens.expires_at:SELECT",
			"user_tokens.used_at:SELECT", "user_tokens.revoked_at:SELECT", "user_tokens:DELETE",
			"outbox_messages.id:SELECT", "outbox_messages.tenant_id:SELECT", "outbox_messages.status:SELECT",
			"outbox_messages.next_attempt_at:SELECT", "outbox_messages.created_at:SELECT",
			"outbox_messages.next_attempt_at:UPDATE", "outbox_messages:DELETE",
			"login_throttles.email_hmac:SELECT", "login_throttles.last_failed_at:SELECT", "login_throttles.locked_until:SELECT",
			"login_throttles:DELETE",
		},
		"crm_signup": {},
		"PUBLIC":     {},
		"crm_app":    {},
	}
	for role, expected := range want {
		t.Run(role, func(t *testing.T) {
			got := privilegeSnapshot(t, role)
			slices.Sort(expected)
			if got == nil {
				got = []string{}
			}
			if !slices.Equal(got, expected) {
				t.Errorf("privileges of %s on app tables:\n got %v\nwant %v", role, got, expected)
			}
		})
	}
}

// T-B109: the routing roles cannot read what they must not: hashes, payloads, personal data.
func TestCatalog_SystemRolesCannotReadBusinessColumns(t *testing.T) {
	t.Parallel()
	app := pgtest.AppPool(t)
	tests := []struct{ role, sql string }{
		{"crm_auth", `SELECT password_hash FROM app.users`},
		{"crm_auth", `SELECT name FROM app.users`},
		{"crm_auth", `SELECT role, status FROM app.users`},
		{"crm_auth", `SELECT * FROM app.users`},
		{"crm_auth", `SELECT user_id FROM app.sessions`},
		{"crm_auth", `SELECT used_at FROM app.user_tokens`},
		{"crm_worker", `SELECT payload FROM app.outbox_messages`},
		{"crm_worker", `SELECT recipient FROM app.outbox_messages`},
		{"crm_worker", `SELECT token_hash FROM app.user_tokens`},
		{"crm_worker", `SELECT token_hash FROM app.sessions`},
		{"crm_worker", `SELECT email FROM app.users`},
		{"crm_worker", `SELECT * FROM app.tenants`},
		{"crm_signup", `SELECT id FROM app.tenants`},
		{"crm_signup", `SELECT 1 FROM app.users`},
		{"crm_auth", `UPDATE app.users SET status = 'active'`},
		{"crm_worker", `UPDATE app.outbox_messages SET status = 'sent'`},
	}
	for _, tt := range tests {
		t.Run(tt.role+": "+tt.sql, func(t *testing.T) {
			asRole(t, app, tt.role, func(ctx context.Context, tx pgx.Tx) {
				if _, err := tx.Exec(ctx, tt.sql); sqlState(err) != "42501" {
					t.Errorf("err = %v, want SQLSTATE 42501", err)
				}
			})
		})
	}
}
