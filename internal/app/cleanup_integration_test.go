//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/securetoken"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type cleanupRunner struct {
	db.TxRunner
	t       *testing.T
	deletes int
}

func (r *cleanupRunner) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	return r.TxRunner.InSystemTx(ctx, role, func(ctx context.Context, tx db.Tx) error { return fn(ctx, &cleanupTx{Tx: tx, owner: r}) })
}

type cleanupTx struct {
	db.Tx
	owner *cleanupRunner
}

func (tx *cleanupTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "DELETE FROM") {
		var user, role string
		var tenant *string
		if err := tx.Tx.QueryRow(ctx, "SELECT current_user, current_setting('role'), app.current_tenant_id()::text").Scan(&user, &role, &tenant); err != nil {
			tx.owner.t.Fatal(err)
		}
		_, bound := tx.TenantID()
		if user != "crm_worker" || role != "crm_worker" || tenant != nil || bound {
			tx.owner.t.Errorf("cleanup as %s/%s tenant=%v bound=%v", user, role, tenant, bound)
		}
		tx.owner.deletes++
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

type cleanupDelivery struct{}

func (cleanupDelivery) Handle(context.Context, outbox.Message) error { return nil }

type stopCleanup struct{ cancel context.CancelFunc }

func (stopCleanup) Name() string                { return "test_cleanup_complete" }
func (stopCleanup) Every() time.Duration        { return time.Hour }
func (s stopCleanup) Run(context.Context) error { s.cancel(); return nil }

// Two companies, owner-independent worker deletes, database policy probes and the real users API.
func TestPhase9Cleanup(t *testing.T) {
	f := e2e.New(t, pgtest.AppPool(t))
	ctx := context.Background()
	super := pgtest.SuperuserPool(t)
	now := time.Now().UTC()
	old := now.Add(-40 * 24 * time.Hour)
	created := now.Add(-50 * 24 * time.Hour)
	recent := now.Add(-20 * 24 * time.Hour)
	future := now.Add(time.Hour)
	type expectedRow struct {
		table string
		id    uuid.UUID
		keep  bool
		label string
	}
	var rows []expectedRow
	var throttleRows []throttleExpectation
	insert := func(table, label, sql string, keep bool, args ...any) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := super.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatal(label, err)
		}
		rows = append(rows, expectedRow{table, id, keep, label})
		return id
	}
	var activeSession, openInvitation uuid.UUID
	for _, company := range []e2e.Company{f.A, f.B} {
		admin := company.Users["admin"].ID
		invited := company.Users["invited"].ID
		session := func(label string, expires time.Time, revoked *time.Time, keep bool) uuid.UUID {
			var reason *string
			if revoked != nil {
				r := "logout"
				reason = &r
			}
			return insert("sessions", label, `INSERT INTO app.sessions(tenant_id,user_id,token_hash,created_at,last_seen_at,expires_at,revoked_at,revoked_reason) VALUES($1,$2,$3,$4,$4,$5,$6,$7) RETURNING id`, keep, company.ID, admin, securetoken.Hash(uuid.NewString()), created, expires, revoked, reason)
		}
		session("expired-old", old, nil, false)
		session("expired-recent", recent, nil, true)
		activeSession = session("active", future, nil, true)
		session("revoked-old", future, &old, false)
		session("revoked-recent", future, &recent, true)
		token := func(label, purpose string, user uuid.UUID, expires time.Time, used, revoked *time.Time, keep bool) uuid.UUID {
			return insert("user_tokens", label, `INSERT INTO app.user_tokens(tenant_id,user_id,purpose,token_hash,created_at,expires_at,used_at,revoked_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, keep, company.ID, user, purpose, securetoken.Hash(uuid.NewString()), created, expires, used, revoked)
		}
		var id uuid.UUID
		if err := super.QueryRow(ctx, `UPDATE app.user_tokens SET created_at=$3,expires_at=$4 WHERE tenant_id=$1 AND user_id=$2 AND purpose='invitation' AND used_at IS NULL AND revoked_at IS NULL RETURNING id`, company.ID, invited, created, old).Scan(&id); err != nil {
			t.Fatal(err)
		}
		openInvitation = id
		rows = append(rows, expectedRow{"user_tokens", id, true, "open-invitation"})
		token("reinvited-revoked", "invitation", invited, old, nil, &old, false)
		token("accepted-used", "invitation", company.Users["operator"].ID, old, &old, nil, false)
		token("disabled-revoked", "invitation", company.Users["disabled_no_password"].ID, old, nil, &old, false)
		token("reset-old", "password_reset", admin, old, nil, nil, false)
		token("reset-recent", "password_reset", admin, recent, nil, nil, true)
		token("verification-live", "email_verification", admin, future, nil, nil, true)
		for _, message := range []struct {
			status string
			age    time.Time
			keep   bool
		}{{"sent", created, false}, {"failed", created, false}, {"pending", created, true}, {"sent", recent, true}, {"failed", recent, true}} {
			var payload []byte
			var sent, failed *time.Time
			switch message.status {
			case "pending":
				payload = []byte(`{}`)
			case "sent":
				sent = &old
			case "failed":
				failed = &old
			}
			insert("outbox_messages", message.status, `INSERT INTO app.outbox_messages(tenant_id,kind,template,recipient,payload,status,created_at,next_attempt_at,sent_at,failed_at) VALUES($1,'email','invitation','cleanup@example.test',$2,$3,$4,$5,$6,$7) RETURNING id`, message.keep, company.ID, payload, message.status, message.age, future, sent, failed)
		}
	}
	for _, tc := range []struct {
		last   time.Time
		locked *time.Time
		keep   bool
	}{{now.Add(-25 * time.Hour), nil, false}, {now.Add(-23 * time.Hour), nil, true}, {now.Add(-25 * time.Hour), &future, true}, {now.Add(-25 * time.Hour), &old, false}} {
		h := securetoken.Hash(uuid.NewString())
		if _, err := super.Exec(ctx, `INSERT INTO app.login_throttles(email_hmac,failed_count,first_failed_at,last_failed_at,locked_until) VALUES($1,1,$2,$2,$3)`, h, tc.last, tc.locked); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = super.Exec(context.Background(), `DELETE FROM app.login_throttles WHERE email_hmac=$1`, h)
		})
		throttleRows = append(throttleRows, throttleExpectation{h, tc.keep})
	}
	for _, probe := range []struct {
		table string
		id    uuid.UUID
	}{{"sessions", activeSession}, {"user_tokens", openInvitation}} {
		err := f.Runner.InSystemTx(ctx, db.RoleWorker, func(ctx context.Context, tx db.Tx) error {
			tag, err := tx.Exec(ctx, "DELETE FROM app."+probe.table+" WHERE id=$1", probe.id)
			if err == nil && tag.RowsAffected() != 0 {
				t.Errorf("worker deleted protected %s: %d", probe.table, tag.RowsAffected())
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	observed := &cleanupRunner{TxRunner: f.Runner, t: t}
	cleanup := identity.NewCleanup(observed, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if cleanup.Every() != time.Hour {
		t.Fatalf("cleanup Every=%s", cleanup.Every())
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	tasks := app.PeriodicTasks(observed, f.Clock, f.Logger)
	tasks = append(tasks, stopCleanup{cancel})
	dispatcher := outbox.NewDispatcher(observed, cleanupDelivery{}, f.Clock, f.Logger, tasks...)
	if err := dispatcher.Run(runCtx); err != nil {
		t.Fatal(err)
	}
	if observed.deletes != 4 {
		t.Fatalf("cleanup DELETE statements=%d want=4", observed.deletes)
	}
	for _, row := range rows {
		var exists bool
		if err := super.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM app."+row.table+" WHERE id=$1)", row.id).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists != row.keep {
			t.Errorf("%s/%s exists=%v want=%v", row.table, row.label, exists, row.keep)
		}
	}
	for _, row := range throttleRows {
		var exists bool
		if err := super.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.login_throttles WHERE email_hmac=$1)`, row.hash).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists != row.keep {
			t.Errorf("throttle exists=%v want=%v", exists, row.keep)
		}
	}
	api := app.BuildAPIRouter(f.Users, f.Companies, f.Clock, f.Logger)
	for _, company := range []e2e.Company{f.A, f.B} {
		rec := isolationRequest(t, api, ctx, company.Users["admin"].Cookie, http.MethodGet, "/api/v1/users", "", nil, "", 200)
		var response struct {
			Items []struct {
				ID      uuid.UUID  `json:"id"`
				Expires *time.Time `json:"invitation_expires_at"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, user := range response.Items {
			if user.ID == company.Users["invited"].ID {
				found = true
				if user.Expires == nil || !user.Expires.Equal(old.Truncate(time.Microsecond)) {
					t.Errorf("DD-25 date=%v want=%s", user.Expires, old)
				}
			}
		}
		if !found {
			t.Fatal("invited user missing")
		}
	}
	t.Logf("cleanup: both companies checked; 4 deletes as crm_worker; DD-25 preserved")
}

type throttleExpectation struct {
	hash []byte
	keep bool
}
