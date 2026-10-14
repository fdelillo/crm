//go:build integration

package isolation_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// T-B804: exactly ListManagedUsers' projection/join/order, without its explicit WHERE.
const unfilteredUsers = `SELECT u.id, u.email, u.name, u.role, u.status, u.email_verified_at, u.created_at,
       tok.expires_at AS invitation_expires_at
FROM app.users AS u
LEFT JOIN app.user_tokens AS tok ON tok.tenant_id = u.tenant_id AND tok.user_id = u.id
  AND u.status = 'invited' AND tok.purpose = 'invitation' AND tok.used_at IS NULL AND tok.revoked_at IS NULL
ORDER BY u.created_at, u.id;`

func TestIsolationDefenseInDepth(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "identity", "store", "users.sql"))
	if err != nil {
		t.Fatal(err)
	}
	query := strings.SplitN(strings.SplitN(string(source), "-- name: ListManagedUsers :many", 2)[1], "-- name:", 2)[0]
	withoutFilter := strings.Replace(query, "WHERE u.tenant_id = @tenant_id ", "", 1)
	if strings.Join(strings.Fields(withoutFilter), " ") != strings.Join(strings.Fields(unfilteredUsers), " ") {
		t.Fatal("unfiltered test query drifted from real ListManagedUsers projection/join/order")
	}
	f := e2e.New(t, pgtest.AppPool(t))
	err = f.Runner.InTenantTx(context.Background(), f.A.ID, func(ctx context.Context, tx db.Tx) error {
		rows, err := tx.Query(ctx, unfilteredUsers)
		if err != nil {
			return err
		}
		ids := map[uuid.UUID]bool{}
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				rows.Close()
				return err
			}
			id := uuid.UUID(values[0].([16]byte))
			ids[id] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(ids) != len(f.A.Users) {
			t.Errorf("RLS unfiltered users=%d want=%d", len(ids), len(f.A.Users))
		}
		for _, u := range f.A.Users {
			if !ids[u.ID] {
				t.Errorf("RLS missing A user %s", u.ID)
			}
		}
		for _, u := range f.B.Users {
			if ids[u.ID] {
				t.Errorf("RLS leaked B user %s", u.ID)
			}
		}
		own, err := store.New(tx).ListManagedUsers(ctx, f.A.ID)
		if err != nil {
			return err
		}
		if len(own) != 5 {
			t.Errorf("real query with A: %d want 5", len(own))
		}
		foreign, err := store.New(tx).ListManagedUsers(ctx, f.B.ID)
		if err != nil {
			return err
		}
		if len(foreign) != 0 {
			t.Errorf("real query with B inside A: %d want 0", len(foreign))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("RLS without tenant WHERE: only A; real query with parameter B under role A: 0 rows")
}

// Observe the real PostgreSQL session immediately before EVERY worker UPDATE, on the same
// transaction/connection. Merely recording AsTenant calls would not prove the session's role.
// No connection is acquired for observation: the dispatcher uses one pool slot at a time.
type updateObservation struct {
	tx              int
	message, tenant uuid.UUID
	role, setting   string
	query           string
}
type observingRunner struct {
	db.TxRunner
	t          *testing.T
	next       int
	updates    []updateObservation
	failTenant uuid.UUID
	failed     bool
}
type observingTx struct {
	db.Tx
	owner  *observingRunner
	number int
}

var workerUpdate = regexp.MustCompile(`(?m)^UPDATE\s`)
var workerQueryName = regexp.MustCompile(`^-- name: (\w+) :\w+`)
var tenantArgument = regexp.MustCompile(`tenant_id\s*=\s*\$(\d+)`)
var idArgument = regexp.MustCompile(`\bid\s*=\s*\$(\d+)`)

func (r *observingRunner) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	return r.TxRunner.InSystemTx(ctx, role, func(ctx context.Context, tx db.Tx) error {
		r.next++
		return fn(ctx, &observingTx{Tx: tx, owner: r, number: r.next})
	})
}
func (tx *observingTx) AsTenant(ctx context.Context, id uuid.UUID) error {
	if id == tx.owner.failTenant && !tx.owner.failed {
		tx.owner.failed = true
		return fmt.Errorf("injected first AsTenant(B) failure")
	}
	return tx.Tx.AsTenant(ctx, id)
}

func (tx *observingTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if !workerUpdate.MatchString(sql) {
		return tx.Tx.Exec(ctx, sql, args...)
	}
	name := workerQueryName.FindStringSubmatch(sql)
	if len(name) != 2 {
		tx.owner.t.Errorf("worker UPDATE has no sqlc query name: %s", sql)
		return pgconn.CommandTag{}, fmt.Errorf("unnamed worker UPDATE")
	}
	query := name[1]
	switch query {
	case "MarkSent", "MarkRecoverable", "MarkFailed", "DeferMessage":
	default:
		tx.owner.t.Errorf("worker UPDATE outside closed query list: %s", query)
		return pgconn.CommandTag{}, fmt.Errorf("unexpected worker UPDATE %s", query)
	}
	var role, setting string
	var currentTenant uuid.NullUUID
	if err := tx.Tx.QueryRow(ctx, `SELECT current_user, current_setting('role'), app.current_tenant_id()`).Scan(&role, &setting, &currentTenant); err != nil {
		return pgconn.CommandTag{}, err
	}
	idMatch := idArgument.FindStringSubmatch(sql)
	if len(idMatch) != 2 {
		tx.owner.t.Errorf("worker UPDATE missing message: %s", query)
		return pgconn.CommandTag{}, fmt.Errorf("missing message predicate")
	}
	idIndex, _ := strconv.Atoi(idMatch[1])
	message := args[idIndex-1].(uuid.UUID)
	bound, hasTenant := tx.TenantID()
	tenant := uuid.Nil
	if query == "DeferMessage" {
		if role != string(db.RoleWorker) || setting != string(db.RoleWorker) || currentTenant.Valid || hasTenant {
			tx.owner.t.Errorf("DeferMessage message=%s current_user=%s role=%s current_tenant=%+v bound=%s/%t", message, role, setting, currentTenant, bound, hasTenant)
		}
	} else {
		tenantMatch := tenantArgument.FindStringSubmatch(sql)
		if len(tenantMatch) != 2 {
			tx.owner.t.Errorf("%s missing explicit tenant predicate", query)
			return pgconn.CommandTag{}, fmt.Errorf("missing tenant predicate")
		}
		tenantIndex, _ := strconv.Atoi(tenantMatch[1])
		tenant = args[tenantIndex-1].(uuid.UUID)
		if !hasTenant || bound != tenant || !currentTenant.Valid || currentTenant.UUID != tenant || role != db.TenantRoleName(tenant) || setting != role {
			tx.owner.t.Errorf("%s message=%s argument=%s bound=%s current=%s role=%s setting=%s", query, message, tenant, bound, currentTenant.UUID, role, setting)
		}
		var actual uuid.UUID
		if err := tx.Tx.QueryRow(ctx, `SELECT tenant_id FROM app.outbox_messages WHERE tenant_id=$1 AND id=$2`, tenant, message).Scan(&actual); err != nil {
			return pgconn.CommandTag{}, err
		}
		if actual != tenant {
			tx.owner.t.Errorf("message %s routed to wrong tenant %s", message, tenant)
		}
	}
	tx.owner.updates = append(tx.owner.updates, updateObservation{tx: tx.number, message: message, tenant: tenant, role: role, setting: setting, query: query})
	return tx.Tx.Exec(ctx, sql, args...)
}

// Freeze dispatcher time after fixture creation: deferred/recoverable messages cannot become
// due again while earlier fixtures are drained, regardless of the machine's speed.
type frozenWorkerClock struct{ now time.Time }

func (c frozenWorkerClock) Now() time.Time { return c.now }

type pendingMessage struct {
	attempts    int
	lastError   pgtype.Text
	nextAttempt time.Time
}

type deliveryHandler struct{ messages map[uuid.UUID]bool }

func (h deliveryHandler) Handle(_ context.Context, m outbox.Message) error {
	if !h.messages[m.TenantID] {
		return nil
	}
	// Exercise all normal UPDATE outcomes (sent, recoverable, permanent) for both companies.
	switch m.Template {
	case "password_reset":
		return &outbox.DeliveryError{Cause: outbox.CauseTransient, Phase: outbox.PhaseRcptTo, Detail: "temporary"}
	case "email_verification":
		return &outbox.DeliveryError{Cause: outbox.CauseRecipient, Phase: outbox.PhaseRcptTo, Detail: "recipient rejected"}
	default:
		return nil
	}
}

func TestIsolationWorkerSessionRoles(t *testing.T) {
	f := e2e.New(t, pgtest.AppPool(t))
	ctx := context.Background()
	// Fixtures from earlier tests may have pending mail; retain all target IDs so we can prove
	// each A/B message is observed exactly once even if it takes several dispatcher batches.
	targets := map[uuid.UUID]uuid.UUID{}
	initial := map[uuid.UUID]pendingMessage{}
	for _, c := range []e2e.Company{f.A, f.B} {
		err := f.Runner.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
			rows, err := tx.Query(ctx, `SELECT id, attempts, last_error, next_attempt_at FROM app.outbox_messages WHERE tenant_id=$1 AND status='pending'`, c.ID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id uuid.UUID
				var before pendingMessage
				if err := rows.Scan(&id, &before.attempts, &before.lastError, &before.nextAttempt); err != nil {
					return err
				}
				targets[id] = c.ID
				initial[id] = before
			}
			return rows.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	r := &observingRunner{TxRunner: f.Runner, t: t, failTenant: f.B.ID}
	worker := outbox.NewDispatcher(r, deliveryHandler{messages: map[uuid.UUID]bool{f.A.ID: true, f.B.ID: true}}, frozenWorkerClock{now: f.Clock.Now()}, f.Logger)
	observed := map[uuid.UUID]bool{}
	for range 100 {
		if err := worker.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		for _, u := range r.updates {
			if expected, ok := targets[u.message]; ok {
				if u.query != "DeferMessage" && u.tenant != expected {
					t.Errorf("message %s from %s passed through %s", u.message, expected, u.tenant)
				}
				observed[u.message] = true
			}
		}
		if len(observed) == len(targets) {
			break
		}
	}
	if len(observed) != len(targets) {
		t.Fatalf("worker observed %d/%d fixture messages", len(observed), len(targets))
	}
	if !r.failed {
		t.Fatal("AsTenant(B) failure was never injected")
	}
	deferred := uuid.Nil
	deferrals := 0
	for _, u := range r.updates {
		if u.query == "DeferMessage" {
			deferrals++
			deferred = u.message
			if targets[u.message] != f.B.ID {
				t.Errorf("deferred message %s is not B's fixture message", u.message)
			}
			t.Logf("DeferMessage message=%s current_user=%s current_setting(role)=%s current_tenant=NULL bound=false", u.message, u.role, u.setting)
		}
	}
	if deferrals != 1 {
		t.Fatalf("DeferMessage count=%d want=1", deferrals)
	}
	perTx := map[int]uuid.UUID{}
	counts := map[uuid.UUID]int{}
	for _, u := range r.updates {
		if u.query == "DeferMessage" {
			continue
		}
		if previous, ok := perTx[u.tx]; ok && previous != u.tenant {
			t.Errorf("transaction %d crossed tenants: %s -> %s", u.tx, previous, u.tenant)
		}
		perTx[u.tx] = u.tenant
		if _, ok := targets[u.message]; ok {
			counts[u.message]++
			t.Logf("worker UPDATE message=%s tenant=%s current_user=%s current_setting(role)=%s", u.message, u.tenant, u.role, u.setting)
		}
	}
	for id := range targets {
		want := 1
		if id == deferred {
			want = 0
		}
		if counts[id] != want {
			t.Errorf("message %s mark count=%d want=%d", id, counts[id], want)
		}
	}
	for _, c := range []e2e.Company{f.A, f.B} {
		err := f.Runner.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
			rows, err := tx.Query(ctx, `SELECT id, template, status, attempts, last_error, next_attempt_at FROM app.outbox_messages WHERE tenant_id=$1`, c.ID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id uuid.UUID
				var template, status string
				var attempts int
				var lastError pgtype.Text
				var nextAttempt time.Time
				if err := rows.Scan(&id, &template, &status, &attempts, &lastError, &nextAttempt); err != nil {
					return err
				}
				wantStatus, wantAttempts := "sent", 0
				switch template {
				case "password_reset":
					wantStatus, wantAttempts = "pending", 1
				case "email_verification":
					wantStatus, wantAttempts = "failed", 1
				}
				if id == deferred {
					before := initial[id]
					wantStatus, wantAttempts = "pending", before.attempts
					if lastError != before.lastError || !nextAttempt.After(before.nextAttempt) {
						t.Errorf("deferred message %s: last_error changed or next_attempt_at not advanced", id)
					}
				}
				if status != wantStatus || attempts != wantAttempts {
					t.Errorf("message %s %s: %s/%d want %s/%d", id, template, status, attempts, wantStatus, wantAttempts)
				}
			}
			return rows.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
