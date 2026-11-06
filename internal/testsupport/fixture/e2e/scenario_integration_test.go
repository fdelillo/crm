//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
)

func TestIsolationFixture(t *testing.T) {
	pool := pgtest.AppPool(t)
	t.Logf("runtime pool max connections=%d; fixture and assertions acquire at most one at a time", pool.Config().MaxConns)
	f := e2e.New(t, pool)
	for _, c := range []*e2e.Company{&f.A, &f.B} {
		if len(c.Users) != 5 || c.Reset == "" || c.Verification == "" || c.Invitation == "" || len(c.Logo) == 0 {
			t.Fatal("incomplete isolation fixture")
		}
		for key, u := range c.Users {
			p, err := f.Users.ResolveSession(context.Background(), u.Cookie.Value)
			if key == "admin" || key == "operator" {
				if err != nil || p.TenantID != c.ID || p.UserID != u.ID {
					t.Fatalf("%s: principal=%+v err=%v", key, p, err)
				}
			} else if !errors.Is(err, identity.ErrUnauthenticated) {
				t.Fatalf("%s: inactive session accepted: %v", key, err)
			}
		}
		preview, err := f.Users.PreviewInvitation(context.Background(), c.Invitation)
		if err != nil || preview.Email != c.Users["invited"].Email {
			t.Fatalf("invitation: %+v %v", preview, err)
		}
	}
}

// T-002-B306 (DD-3 of 002): the scenario registers its companies with the real seeder, so company A (generic)
// and company B (aluminum_carpentry) start with the configuration of their template.
func TestIsolationFixtureCompaniesStartWithTheirTemplateConfiguration(t *testing.T) {
	f := e2e.New(t, pgtest.AppPool(t))
	for _, tc := range []struct {
		company                   *e2e.Company
		stages, categories, items int
	}{{&f.A, 5, 13, 0}, {&f.B, 6, 14, 4}} {
		var stages, categories, items int
		err := f.Runner.InTenantTx(context.Background(), tc.company.ID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.stages), (SELECT count(*) FROM app.categories),
				(SELECT count(*) FROM app.catalog_items)`).Scan(&stages, &categories, &items)
		})
		if err != nil || stages != tc.stages || categories != tc.categories || items != tc.items {
			t.Fatalf("company %s: stages=%d categories=%d items=%d err=%v, want %d/%d/%d",
				tc.company.Suffix, stages, categories, items, err, tc.stages, tc.categories, tc.items)
		}
	}
}

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }
