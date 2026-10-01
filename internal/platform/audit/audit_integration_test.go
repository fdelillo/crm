//go:build integration

package audit_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func TestRecordTransactionIsolationAndPrivileges(t *testing.T) {
	a := fixture.NewCompany(t, pgtest.AppPool(t))
	b := fixture.NewCompany(t, pgtest.AppPool(t))
	runner := db.NewTxRunner(pgtest.AppPool(t))
	recorder := audit.NewRecorder()
	var requestCtx context.Context
	httpx.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requestCtx = r.Context() })).ServeHTTP(
		httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	requestID := httpx.RequestIDFrom(requestCtx)
	if requestID == "" {
		t.Fatal("missing request id")
	}
	entry := audit.Entry{TenantID: a.ID, Action: "user.role_changed", TargetType: "user", TargetID: &a.UserID,
		ActorUserID: &a.UserID, Data: map[string]any{"role": "operator"}}
	err := runner.InTenantTx(requestCtx, a.ID, func(ctx context.Context, tx db.Tx) error { return recorder.Record(ctx, tx, entry) })
	if err != nil {
		t.Fatal(err)
	}
	err = runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
		var count int
		var gotRequestID string
		if err := tx.QueryRow(ctx, `SELECT count(*), max(request_id) FROM app.audit_log
   WHERE tenant_id = $1 AND action = 'user.role_changed'`, a.ID).Scan(&count, &gotRequestID); err != nil {
			return err
		}
		if count != 1 || gotRequestID != requestID {
			t.Errorf("count=%d request_id=%q", count, gotRequestID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
		wrong := entry
		wrong.TenantID = b.ID
		return recorder.Record(ctx, tx, wrong)
	})
	if err == nil {
		t.Fatal("cross-tenant audit inserted")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("cross-tenant error=%v", err)
	}

	abort := errors.New("abort")
	err = runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
		entry.Action = "user.invited"
		if err := recorder.Record(ctx, tx, entry); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("rollback error=%v", err)
	}
	err = runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id = $1 AND action = 'user.invited'`, a.ID).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			t.Errorf("rolled back rows=%d", count)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE app.audit_log SET action = 'user.invited' WHERE tenant_id = $1`,
		`DELETE FROM app.audit_log WHERE tenant_id = $1`,
	} {
		err := runner.InTenantTx(context.Background(), a.ID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, statement, a.ID)
			return err
		})
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("append-only error=%v", err)
		}
	}
}

func TestRecordRejectsSecrets(t *testing.T) {
	recorder := audit.NewRecorder()
	for _, data := range []map[string]any{{"password": "secret"}, {"token": "secret"}, {"nested": map[string]any{"reset_token": "secret"}}} {
		err := recorder.Record(context.Background(), nil, audit.Entry{Data: data})
		if err == nil {
			t.Fatalf("accepted secret data: %v", data)
		}
	}
}
