//go:build integration

package sqlcspike_test

import (
	"context"
	"net/netip"
	"os"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/fdelillo/crm/internal/testsupport/sqlcspike"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func asRole(t *testing.T, role string, fn func(ctx context.Context, q *sqlcspike.Queries, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pgtest.AppPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := fixture.SetRole(ctx, tx, role); err != nil {
		t.Fatal(err)
	}
	fn(ctx, sqlcspike.New(tx), tx)
}

// The generated code runs against the real schema, as the company role and as crm_auth, with the
// type overrides of ADR-003 (uuid, inet, timestamptz) going through pgx.
func TestGeneratedQueriesRunAgainstTheRealSchema(t *testing.T) {
	c := fixture.NewCompany(t, pgtest.AppPool(t))

	asRole(t, c.Role, func(ctx context.Context, q *sqlcspike.Queries, _ pgx.Tx) {
		tenant, err := q.GetTenantByID(ctx, c.ID)
		if err != nil || tenant.ID != c.ID || tenant.CreatedAt.IsZero() || tenant.LogoObjectKey.Valid {
			t.Errorf("GetTenantByID = %+v, %v", tenant, err)
		}
		if _, err := q.GetTenantByID(ctx, uuid.New()); err == nil {
			t.Error("GetTenantByID of another company returned a row")
		}

		ip := netip.MustParseAddr("203.0.113.9")
		row, err := q.InsertAuditLog(ctx, sqlcspike.InsertAuditLogParams{
			TenantID: c.ID, ActorUserID: uuid.NullUUID{UUID: c.UserID, Valid: true},
			Action: "auth.login_succeeded", Ip: &ip,
		})
		if err != nil || row.Ip == nil || *row.Ip != ip || row.OccurredAt.IsZero() {
			t.Errorf("InsertAuditLog = %+v, %v", row, err)
		}
	})

	var email string
	if err := pgtest.SuperuserPool(t).QueryRow(context.Background(), `SELECT email FROM app.users WHERE id = $1`, c.UserID).Scan(&email); err != nil {
		t.Fatal(err)
	}
	asRole(t, "crm_auth", func(ctx context.Context, q *sqlcspike.Queries, _ pgx.Tx) {
		got, err := q.LookupUserByEmail(ctx, email)
		if err != nil || got.ID != c.UserID || got.TenantID != c.ID {
			t.Errorf("LookupUserByEmail = %+v, %v; want user %v of company %v", got, err, c.UserID, c.ID)
		}
	})
}
