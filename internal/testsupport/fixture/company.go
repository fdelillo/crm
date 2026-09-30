package fixture

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Inserter inserts one valid row of a company table, as the role of tenantID, and returns its id.
// userID is an existing user of that company, for tables that reference one.
type Inserter func(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) (uuid.UUID, error)

// CompanyInserters has one entry per company table (every table of schema app with a tenant_id, plus
// tenants). The isolation test walks the catalog and fails if a table has no entry here: a spec that
// adds a table adds its inserter, and the generic tests then cover it (data-model.md §6, checklist).
var CompanyInserters = map[string]Inserter{
	"tenants": func(ctx context.Context, tx pgx.Tx, tenantID, _ uuid.UUID) (uuid.UUID, error) {
		_, err := tx.Exec(ctx, `INSERT INTO app.tenants (id, name, base_currency, industry_template_code, industry_template_version)
			VALUES ($1, 'Fixture SA', 'ARS', 'generic', 1)`, tenantID)
		return tenantID, err
	},
	"users": func(ctx context.Context, tx pgx.Tx, tenantID, _ uuid.UUID) (uuid.UUID, error) {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO app.users (tenant_id, email, name, password_hash, role, status)
			VALUES ($1, $2, 'Fixture User', '$argon2id$fixture', 'admin', 'active') RETURNING id`,
			tenantID, "u-"+uuid.NewString()+"@fixture.example").Scan(&id)
		return id, err
	},
	"sessions": func(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) (uuid.UUID, error) {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO app.sessions (tenant_id, user_id, token_hash, expires_at)
			VALUES ($1, $2, $3, now() + interval '7 days') RETURNING id`, tenantID, userID, randomHash()).Scan(&id)
		return id, err
	},
	"user_tokens": func(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) (uuid.UUID, error) {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO app.user_tokens (tenant_id, user_id, purpose, token_hash, expires_at)
			VALUES ($1, $2, 'password_reset', $3, now() + interval '1 hour') RETURNING id`, tenantID, userID, randomHash()).Scan(&id)
		return id, err
	},
	"outbox_messages": func(ctx context.Context, tx pgx.Tx, tenantID, _ uuid.UUID) (uuid.UUID, error) {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO app.outbox_messages (tenant_id, kind, template, recipient, payload)
			VALUES ($1, 'email', 'password_reset', 'someone@fixture.example', '{"link":"x"}') RETURNING id`, tenantID).Scan(&id)
		return id, err
	},
	"audit_log": func(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) (uuid.UUID, error) {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO app.audit_log (tenant_id, actor_user_id, action, target_type, target_id)
			VALUES ($1, $2, 'tenant.registered', 'tenant', $1) RETURNING id`, tenantID, userID).Scan(&id)
		return id, err
	},
}

func randomHash() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

// Company is a company with its role and one row in every company table.
type Company struct {
	ID     uuid.UUID
	Role   string
	UserID uuid.UUID
	// Rows maps each table to the id of the row inserted for this company.
	Rows map[string]uuid.UUID
}

// NewCompany provisions a company role and inserts one row in every company table, each insert made as
// the company's own role (so the data is what RLS would let the application write). It commits.
func NewCompany(t testing.TB, pool *pgxpool.Pool) Company {
	t.Helper()
	ctx := context.Background()
	c := Company{ID: uuid.New(), Rows: map[string]uuid.UUID{}}
	c.Role = ProvisionRole(t, pool, c.ID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+pgx.Identifier{c.Role}.Sanitize()); err != nil {
		t.Fatal(err)
	}

	// tenants first, then users (others reference them), then the rest in a stable order.
	var rest []string
	for name := range CompanyInserters {
		if name != "tenants" && name != "users" {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range append([]string{"tenants", "users"}, rest...) {
		id, err := CompanyInserters[name](ctx, tx, c.ID, c.UserID)
		if err != nil {
			t.Fatalf("fixture: inserting into %s as %s: %v", name, c.Role, err)
		}
		c.Rows[name] = id
		if name == "users" {
			c.UserID = id
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return c
}

// Insert runs the inserter of table in tx (already switched to some role) for tenantID.
func Insert(ctx context.Context, tx pgx.Tx, table string, tenantID, userID uuid.UUID) (uuid.UUID, error) {
	in, ok := CompanyInserters[table]
	if !ok {
		return uuid.Nil, fmt.Errorf("fixture: no inserter for table %s", table)
	}
	return in(ctx, tx, tenantID, userID)
}
