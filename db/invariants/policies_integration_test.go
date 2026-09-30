//go:build integration

package invariants_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// T-B107/T-B109: the complete set of policies of schema app, exactly as data-model.md §3.4 defines them,
// in the form PostgreSQL 18 prints them (table | policy | permissive | roles | command | USING | WITH CHECK).
// An added, removed or loosened policy fails this test and needs a change in the design (plan §16).
func TestCatalog_PoliciesAreExactlyTheDocumentedOnes(t *testing.T) {
	t.Parallel()
	const tenantIsolationTenant = "(tenant_id = ( SELECT app.current_tenant_id() AS current_tenant_id))"
	const tenantIsolationID = "(id = ( SELECT app.current_tenant_id() AS current_tenant_id))"
	want := []string{
		"audit_log|tenant_isolation|PERMISSIVE|{public}|ALL|" + tenantIsolationTenant + "|" + tenantIsolationTenant,
		"login_throttles|auth_all|PERMISSIVE|{crm_auth}|ALL|true|true",
		"login_throttles|worker_cleanup|PERMISSIVE|{crm_worker}|DELETE|((last_failed_at < (now() - '1 day'::interval)) AND ((locked_until IS NULL) OR (locked_until < now())))|<none>",
		"login_throttles|worker_read|PERMISSIVE|{crm_worker}|SELECT|true|<none>",
		"outbox_messages|tenant_isolation|PERMISSIVE|{public}|ALL|" + tenantIsolationTenant + "|" + tenantIsolationTenant,
		"outbox_messages|worker_cleanup|PERMISSIVE|{crm_worker}|DELETE|((status <> 'pending'::text) AND (created_at < (now() - '30 days'::interval)))|<none>",
		"outbox_messages|worker_lock|PERMISSIVE|{crm_worker}|UPDATE|(status = 'pending'::text)|(status = 'pending'::text)",
		"outbox_messages|worker_read|PERMISSIVE|{crm_worker}|SELECT|true|<none>",
		"sessions|auth_lookup|PERMISSIVE|{crm_auth}|SELECT|true|<none>",
		"sessions|tenant_isolation|PERMISSIVE|{public}|ALL|" + tenantIsolationTenant + "|" + tenantIsolationTenant,
		"sessions|worker_cleanup|PERMISSIVE|{crm_worker}|DELETE|((expires_at < (now() - '30 days'::interval)) OR (revoked_at < (now() - '30 days'::interval)))|<none>",
		"sessions|worker_read|PERMISSIVE|{crm_worker}|SELECT|true|<none>",
		"tenants|tenant_isolation|PERMISSIVE|{public}|ALL|" + tenantIsolationID + "|" + tenantIsolationID,
		"tenants|worker_read|PERMISSIVE|{crm_worker}|SELECT|true|<none>",
		"user_tokens|auth_lookup|PERMISSIVE|{crm_auth}|SELECT|true|<none>",
		"user_tokens|tenant_isolation|PERMISSIVE|{public}|ALL|" + tenantIsolationTenant + "|" + tenantIsolationTenant,
		"user_tokens|worker_cleanup|PERMISSIVE|{crm_worker}|DELETE|((expires_at < (now() - '30 days'::interval)) AND ((purpose <> 'invitation'::text) OR (used_at IS NOT NULL) OR (revoked_at IS NOT NULL)))|<none>",
		"user_tokens|worker_read|PERMISSIVE|{crm_worker}|SELECT|true|<none>",
		"users|auth_lookup|PERMISSIVE|{crm_auth}|SELECT|true|<none>",
		"users|tenant_isolation|PERMISSIVE|{public}|ALL|" + tenantIsolationTenant + "|" + tenantIsolationTenant,
	}
	slices.Sort(want)
	got := queryStrings(t, pgtest.OwnerPool(t), `
		SELECT tablename || '|' || policyname || '|' || permissive || '|' || roles::text || '|' || cmd || '|' ||
		       coalesce(qual, '<none>') || '|' || coalesce(with_check, '<none>')
		FROM pg_policies WHERE schemaname = 'app'`)
	if !slices.Equal(got, want) {
		t.Errorf("policies of schema app differ from the documented ones\n got:\n  %v\nwant:\n  %v", got, want)
	}
}

// insertAs runs sql as the company role in a committed transaction and returns the id of the row.
func insertAs(t *testing.T, c fixture.Company, sql string, args ...any) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	pool := pgtest.AppPool(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+pgx.Identifier{c.Role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	if err := tx.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return id
}

// workerDeletes runs DELETE ... WHERE id = ANY(ids) as crm_worker (rolled back) and returns which ids it removed.
func workerDeletes(t *testing.T, table string, ids []uuid.UUID) []uuid.UUID {
	t.Helper()
	var deleted []uuid.UUID
	asRole(t, pgtest.AppPool(t), "crm_worker", func(ctx context.Context, tx pgx.Tx) {
		rows, err := tx.Query(ctx, `DELETE FROM app.`+table+` WHERE id = ANY($1) RETURNING id`, ids)
		if err != nil {
			t.Fatalf("DELETE FROM %s as crm_worker: %v", table, err)
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			deleted = append(deleted, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	})
	return deleted
}

func expectDeleted(t *testing.T, what string, got []uuid.UUID, want map[string]uuid.UUID, shouldGo ...string) {
	t.Helper()
	var wantIDs []uuid.UUID
	for _, name := range shouldGo {
		wantIDs = append(wantIDs, want[name])
	}
	slices.SortFunc(got, func(a, b uuid.UUID) int { return compareUUID(a, b) })
	slices.SortFunc(wantIDs, func(a, b uuid.UUID) int { return compareUUID(a, b) })
	if !slices.Equal(got, wantIDs) {
		t.Errorf("%s: crm_worker deleted %v, want exactly %v (%v)", what, got, wantIDs, shouldGo)
	}
}

func compareUUID(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) }

func allIDs(m map[string]uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range m {
		out = append(out, id)
	}
	return out
}

// T-B111, DD-25: the cleanup policies of crm_worker only reach what they are meant to.
func TestIsolation_WorkerCleanupOfSessionsOnlyReachesOldRows(t *testing.T) {
	t.Parallel()
	c := fixture.NewCompany(t, pgtest.AppPool(t))
	const q = `INSERT INTO app.sessions (tenant_id, user_id, token_hash, created_at, last_seen_at, expires_at, revoked_at, revoked_reason)
		VALUES ($1, $2, $3, $4, $4, $5, $6, $7) RETURNING id`
	now := time.Now()
	day := 24 * time.Hour
	hash := func() []byte { b := uuid.New(); return append(b[:], b[:]...) }
	revoked := func(ago time.Duration) (any, any) { return now.Add(-ago), "logout" }
	rev31, why := revoked(31 * day)
	rev1, _ := revoked(1 * day)

	rows := map[string]uuid.UUID{
		"current":           insertAs(t, c, q, c.ID, c.UserID, hash(), now.Add(-time.Hour), now.Add(7*day), nil, nil),
		"expired 31 days":   insertAs(t, c, q, c.ID, c.UserID, hash(), now.Add(-40*day), now.Add(-31*day), nil, nil),
		"expired 5 days":    insertAs(t, c, q, c.ID, c.UserID, hash(), now.Add(-12*day), now.Add(-5*day), nil, nil),
		"revoked 31 days":   insertAs(t, c, q, c.ID, c.UserID, hash(), now.Add(-40*day), now.Add(30*day), rev31, why),
		"revoked yesterday": insertAs(t, c, q, c.ID, c.UserID, hash(), now.Add(-2*day), now.Add(30*day), rev1, why),
	}
	got := workerDeletes(t, "sessions", allIDs(rows))
	expectDeleted(t, "sessions", got, rows, "expired 31 days", "revoked 31 days")
}

func TestIsolation_WorkerCleanupOfTokensKeepsTheOpenInvitation(t *testing.T) {
	t.Parallel()
	c := fixture.NewCompany(t, pgtest.AppPool(t))
	const q = `INSERT INTO app.user_tokens (tenant_id, user_id, purpose, token_hash, created_at, expires_at, used_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	now := time.Now()
	day := 24 * time.Hour
	hash := func() []byte { b := uuid.New(); return append(b[:], b[:]...) }
	old, older := now.Add(-40*day), now.Add(-31*day)

	rows := map[string]uuid.UUID{
		// DD-25: the open invitation of an invited user stays, however old, so the UI can say "Invitación vencida".
		"open invitation expired 31 days": insertAs(t, c, q, c.ID, c.UserID, "invitation", hash(), old, older, nil, nil),
		"used invitation expired 31 days": insertAs(t, c, q, c.ID, c.UserID, "invitation", hash(), old, older, older, nil),
		"revoked invitation expired 31":   insertAs(t, c, q, c.ID, c.UserID, "invitation", hash(), old, older, nil, older),
		"reset expired 31 days":           insertAs(t, c, q, c.ID, c.UserID, "password_reset", hash(), old, older, nil, nil),
		"verification expired 31 days":    insertAs(t, c, q, c.ID, c.UserID, "email_verification", hash(), old, older, nil, nil),
		"reset expired 5 days":            insertAs(t, c, q, c.ID, c.UserID, "password_reset", hash(), now.Add(-10*day), now.Add(-5*day), nil, nil),
		"reset still valid":               insertAs(t, c, q, c.ID, c.UserID, "password_reset", hash(), now.Add(-time.Minute), now.Add(time.Hour), nil, nil),
	}
	got := workerDeletes(t, "user_tokens", allIDs(rows))
	expectDeleted(t, "user_tokens", got, rows,
		"used invitation expired 31 days", "revoked invitation expired 31", "reset expired 31 days", "verification expired 31 days")
}

func TestIsolation_WorkerCleanupOfOutboxOnlyReachesOldFinishedMessages(t *testing.T) {
	t.Parallel()
	c := fixture.NewCompany(t, pgtest.AppPool(t))
	now := time.Now()
	day := 24 * time.Hour
	pending := `INSERT INTO app.outbox_messages (tenant_id, kind, template, recipient, payload, created_at)
		VALUES ($1, 'email', 'password_reset', 'x@example.com', '{"a":"b"}', $2) RETURNING id`
	sent := `INSERT INTO app.outbox_messages (tenant_id, kind, template, recipient, status, sent_at, created_at)
		VALUES ($1, 'email', 'password_reset', 'x@example.com', 'sent', $2, $2) RETURNING id`
	failed := `INSERT INTO app.outbox_messages (tenant_id, kind, template, recipient, status, failed_at, created_at)
		VALUES ($1, 'email', 'password_reset', 'x@example.com', 'failed', $2, $2) RETURNING id`

	rows := map[string]uuid.UUID{
		"pending 40 days":   insertAs(t, c, pending, c.ID, now.Add(-40*day)),
		"sent 31 days":      insertAs(t, c, sent, c.ID, now.Add(-31*day)),
		"failed 31 days":    insertAs(t, c, failed, c.ID, now.Add(-31*day)),
		"sent 5 days":       insertAs(t, c, sent, c.ID, now.Add(-5*day)),
		"pending yesterday": insertAs(t, c, pending, c.ID, now.Add(-day)),
	}
	got := workerDeletes(t, "outbox_messages", allIDs(rows))
	expectDeleted(t, "outbox_messages", got, rows, "sent 31 days", "failed 31 days")
}

func TestIsolation_WorkerCleanupOfLoginThrottles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	app := pgtest.AppPool(t)
	now := time.Now()
	day := 24 * time.Hour
	insert := func(lastFailed time.Time, lockedUntil *time.Time) []byte {
		id := uuid.New()
		hmac := append(id[:], id[:]...)
		asRoleCommit(t, app, "crm_auth", func(ctx context.Context, tx pgx.Tx) {
			_, err := tx.Exec(ctx, `INSERT INTO app.login_throttles (email_hmac, failed_count, first_failed_at, last_failed_at, locked_until)
				VALUES ($1, 3, $2, $2, $3)`, hmac, lastFailed, lockedUntil)
			if err != nil {
				t.Fatal(err)
			}
		})
		return hmac
	}
	future, past := now.Add(time.Hour), now.Add(-3*day)
	old := insert(now.Add(-2*day), nil)
	oldLockExpired := insert(now.Add(-2*day), &past)
	oldStillLocked := insert(now.Add(-2*day), &future)
	recent := insert(now.Add(-time.Hour), nil)

	asRole(t, app, "crm_worker", func(ctx context.Context, tx pgx.Tx) {
		rows, err := tx.Query(ctx, `DELETE FROM app.login_throttles WHERE email_hmac = ANY($1) RETURNING email_hmac`,
			[][]byte{old, oldLockExpired, oldStillLocked, recent})
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		deleted := map[string]bool{}
		for rows.Next() {
			var h []byte
			if err := rows.Scan(&h); err != nil {
				t.Fatal(err)
			}
			deleted[string(h)] = true
		}
		want := map[string]bool{string(old): true, string(oldLockExpired): true}
		if len(deleted) != len(want) || !deleted[string(old)] || !deleted[string(oldLockExpired)] {
			t.Errorf("deleted %d throttles; want exactly the old unlocked ones (a lock still in force and recent failures stay)", len(deleted))
		}
	})
	_ = ctx
}

// asRoleCommit is asRole with a COMMIT at the end.
func asRoleCommit(t *testing.T, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, role string, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	fn(ctx, tx)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
