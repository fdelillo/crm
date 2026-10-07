//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/fdelillo/crm/internal/identity"
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

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }
