//go:build integration

package tenant_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/objectstore"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

var errUncertainCommit = fmt.Errorf("test: commit result lost: %w", context.Canceled)
var errForcedRollback = errors.New("test: rollback instead of commit")

type uncertainLogoRunner struct {
	db.TxRunner
	mode   string
	cancel context.CancelFunc
	calls  int
}

func (r *uncertainLogoRunner) InTenantTx(ctx context.Context, id uuid.UUID, fn func(context.Context, db.Tx) error) error {
	r.calls++
	if r.calls > 1 {
		if r.mode == "unknown" {
			return db.ErrUnavailable
		}
		return r.TxRunner.InTenantTx(ctx, id, fn)
	}
	if r.mode == "before_function" {
		r.cancel()
		return db.ErrUnavailable
	}
	err := r.TxRunner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
		if err := fn(ctx, tx); err != nil {
			return err
		}
		if r.mode == "rolled_back" {
			return errForcedRollback
		}
		return nil
	})
	if err != nil && !errors.Is(err, errForcedRollback) {
		return err
	}
	r.cancel()
	if r.mode == "confirmed" {
		return nil
	}
	return errUncertainCommit
}
func TestLogoUncertainCommitReconciles(t *testing.T) {
	for _, mode := range []string{"committed", "rolled_back", "unknown", "before_function"} {
		t.Run(mode, func(t *testing.T) {
			_, _, real := newRegistrationServices(t)
			f := newLogoStorage()
			var logs bytes.Buffer
			svc := logoService(t, real, audit.NewRecorder(), f, &logs)
			p := companyAdmin(t, svc)
			if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
				t.Fatal(err)
			}
			old, _ := companyRow(t, real, p)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runner := &uncertainLogoRunner{TxRunner: real, mode: mode, cancel: cancel}
			svc = logoService(t, runner, audit.NewRecorder(), f, &logs)
			out, err := svc.SetLogo(ctx, p, pngLogo(t, 11, 10, 0), identity.RequestMeta{})
			key, _ := companyRow(t, real, p)
			newKey := f.puts[len(f.puts)-1]
			if key == nil {
				t.Fatal("missing company logo reference")
			}
			if _, ok := f.objects[*key]; !ok {
				t.Fatalf("DD-42/INV-17: referenced logo deleted: %s", *key)
			}
			switch mode {
			case "committed":
				if err != nil || !out.HasLogo || *key != newKey || len(f.objects) != 1 {
					t.Fatalf("committed result=%+v err=%v objects=%d logs=%s", out, err, len(f.objects), logs.String())
				}
			case "rolled_back", "before_function":
				if err == nil || *key != *old || len(f.objects) != 1 {
					t.Fatalf("rollback reference=%s err=%v objects=%d", *key, err, len(f.objects))
				}
				if _, ok := f.objects[newKey]; ok {
					t.Fatal("rollback left new object")
				}
			case "unknown":
				if !errors.Is(err, errUncertainCommit) || *key != newKey || len(f.objects) != 2 || len(f.deletes) != 0 {
					t.Fatalf("unknown outcome deleted objects: err=%v objects=%d deletes=%v", err, len(f.objects), f.deletes)
				}
			}
			if mode != "before_function" && (!strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "event=logo_commit_uncertain") || !strings.Contains(logs.String(), "outcome="+mode)) {
				t.Fatalf("uncertain event missing: %s", logs.String())
			}
			wantCalls := 2
			if mode == "before_function" {
				wantCalls = 1
			}
			if runner.calls != wantCalls {
				t.Fatalf("calls=%d want=%d", runner.calls, wantCalls)
			}
		})
	}
}

type flyingLogoRunner struct {
	db.TxRunner
	pid      chan int32
	release  chan struct{}
	finished chan error
	calls    int
}

func (r *flyingLogoRunner) InTenantTx(ctx context.Context, id uuid.UUID, fn func(context.Context, db.Tx) error) error {
	r.calls++
	if r.calls > 1 {
		return r.TxRunner.InTenantTx(ctx, id, fn)
	}
	ready := make(chan error, 1)
	go func() {
		err := r.TxRunner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
			if err := fn(ctx, tx); err != nil {
				ready <- err
				return err
			}
			var pid int32
			if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				ready <- err
				return err
			}
			r.pid <- pid
			ready <- nil
			<-r.release
			return nil
		})
		r.finished <- err
	}()
	if err := <-ready; err != nil {
		return err
	}
	return errUncertainCommit
}
func TestLogoCommitInFlightWaitsForLock(t *testing.T) {
	_, _, real := newRegistrationServices(t)
	f := newLogoStorage()
	var logs bytes.Buffer
	svc := logoService(t, real, audit.NewRecorder(), f, &logs)
	p := companyAdmin(t, svc)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r := &flyingLogoRunner{TxRunner: real, pid: make(chan int32, 1), release: make(chan struct{}), finished: make(chan error, 1)}
	svc = logoService(t, r, audit.NewRecorder(), f, &logs)
	done := make(chan error, 1)
	go func() { _, err := svc.SetLogo(ctx, p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); done <- err }()
	pid := <-r.pid
	var once sync.Once
	release := func() { once.Do(func() { close(r.release) }) }
	defer release()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	var operationErr, errorBeforeLock error
	completed := false
	waiting := false
	for !waiting && errorBeforeLock == nil {
		select {
		case operationErr = <-done:
			completed = true
			errorBeforeLock = fmt.Errorf("DD-42: SetLogo returned before reconciliation locked the in-flight row: %w", errors.Join(operationErr, errors.New("no waiter")))
		case <-ticker.C:
			if err := pgtest.AppPool(t).QueryRow(ctx, `SELECT EXISTS(SELECT pid FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&waiting); err != nil {
				errorBeforeLock = err
			}
		case <-ctx.Done():
			errorBeforeLock = fmt.Errorf("DD-42: reconciliation never waited for in-flight COMMIT: %w", ctx.Err())
		}
	}
	release()
	commitErr := <-r.finished
	if !completed {
		operationErr = <-done
	}
	if errorBeforeLock != nil {
		t.Fatal(errorBeforeLock)
	}
	if commitErr != nil || operationErr != nil {
		t.Fatalf("commit=%v set=%v", commitErr, operationErr)
	}
	key, _ := companyRow(t, real, p)
	if key == nil {
		t.Fatal("no committed key")
	}
	if _, ok := f.objects[*key]; !ok {
		t.Fatal("DD-42: committed object was deleted")
	}
	if !strings.Contains(logs.String(), "event=logo_commit_uncertain") || !strings.Contains(logs.String(), "outcome=committed") {
		t.Fatal(logs.String())
	}
}

type contextLogoStorage struct {
	*logoStorage
	alive    bool
	deadline time.Duration
	onGet    func(string) error
}

func (f *contextLogoStorage) Delete(ctx context.Context, key string) error {
	f.alive = ctx.Err() == nil
	if deadline, ok := ctx.Deadline(); ok {
		f.deadline = time.Until(deadline)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return f.logoStorage.Delete(ctx, key)
}
func (f *contextLogoStorage) Get(ctx context.Context, key string) (io.ReadCloser, objectstore.ObjectInfo, error) {
	if f.onGet != nil {
		if err := f.onGet(key); err != nil {
			return nil, objectstore.ObjectInfo{}, err
		}
	}
	return f.logoStorage.Get(ctx, key)
}
func TestLogoCleanupUsesOwnContext(t *testing.T) {
	_, _, real := newRegistrationServices(t)
	f := &contextLogoStorage{logoStorage: newLogoStorage()}
	svc := logoService(t, real, audit.NewRecorder(), f, io.Discard)
	p := companyAdmin(t, svc)
	if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	old, _ := companyRow(t, real, p)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc = logoService(t, &uncertainLogoRunner{TxRunner: real, mode: "confirmed", cancel: cancel}, audit.NewRecorder(), f, io.Discard)
	if _, err := svc.SetLogo(ctx, p, pngLogo(t, 11, 10, 0), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if !f.alive || f.deadline <= 0 || f.deadline > 5*time.Second {
		t.Fatalf("cleanup alive=%v deadline=%v", f.alive, f.deadline)
	}
	if _, ok := f.objects[*old]; ok {
		t.Fatal("cleanup kept old object with canceled request")
	}
}
func TestLogoRemoveUncertainAndAbsent(t *testing.T) {
	_, _, real := newRegistrationServices(t)
	f := newLogoStorage()
	svc := logoService(t, real, audit.NewRecorder(), f, io.Discard)
	p := companyAdmin(t, svc)
	if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	old, _ := companyRow(t, real, p)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	uncertain := logoService(t, &uncertainLogoRunner{TxRunner: real, mode: "committed", cancel: cancel}, audit.NewRecorder(), f, io.Discard)
	if err := uncertain.RemoveLogo(ctx, p, identity.RequestMeta{}); !errors.Is(err, errUncertainCommit) {
		t.Fatalf("remove=%v", err)
	}
	if len(f.deletes) != 0 {
		t.Fatal("uncertain RemoveLogo deleted an object")
	}
	if _, ok := f.objects[*old]; !ok {
		t.Fatal("uncertain removal deleted old object")
	}
	before, _ := svc.Get(context.Background(), p)
	var xid string
	var audits int
	read := func() error {
		return real.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT xmin::text,(SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND action='tenant.logo_removed') FROM app.tenants WHERE id=$1`, p.TenantID).Scan(&xid, &audits)
		})
	}
	if err := read(); err != nil {
		t.Fatal(err)
	}
	oldXid, oldAudits := xid, audits
	if err := svc.RemoveLogo(context.Background(), p, identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := read(); err != nil {
		t.Fatal(err)
	}
	after, _ := svc.Get(context.Background(), p)
	if oldXid != xid || oldAudits != audits || !before.UpdatedAt.Equal(after.UpdatedAt) {
		t.Fatal("absent RemoveLogo wrote or audited")
	}
}
func TestConcurrentLogoUploadsKeepOnlyReferencedObject(t *testing.T) {
	_, _, r := newRegistrationServices(t)
	f := newLogoStorage()
	svc := logoService(t, r, audit.NewRecorder(), f, io.Discard)
	p := companyAdmin(t, svc)
	if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	var calls []func(context.Context) error
	for i := range 3 {
		data := pngLogo(t, 11+i, 10, 0)
		calls = append(calls, func(ctx context.Context) error {
			_, err := svc.SetLogo(ctx, p, data, identity.RequestMeta{})
			return err
		})
	}
	runTenantQueue(t, r, p, false, calls...)
	key, _ := companyRow(t, r, p)
	if key == nil || len(f.objects) != 1 {
		t.Fatalf("INV-34: uploads left %d objects key=%v", len(f.objects), key)
	}
	if _, ok := f.objects[*key]; !ok {
		t.Fatal("referenced object missing")
	}
	var count int
	if err := r.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND action='tenant.logo_updated'`, p.TenantID).Scan(&count)
	}); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("3 queued uploads must audit: count=%d including original", count)
	}
}
func TestConcurrentLogoUploadThenRemove(t *testing.T) {
	_, _, r := newRegistrationServices(t)
	f := newLogoStorage()
	svc := logoService(t, r, audit.NewRecorder(), f, io.Discard)
	p := companyAdmin(t, svc)
	if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	data := pngLogo(t, 11, 10, 0)
	runTenantQueue(t, r, p, true, func(ctx context.Context) error {
		_, err := svc.SetLogo(ctx, p, data, identity.RequestMeta{})
		return err
	}, func(ctx context.Context) error { return svc.RemoveLogo(ctx, p, identity.RequestMeta{}) })
	key, ct := companyRow(t, r, p)
	if key != nil || ct != nil || len(f.objects) != 0 {
		t.Fatalf("INV-34/FIFO: upload then remove left key=%v objects=%d", key, len(f.objects))
	}
	var updated, removed int
	if err := r.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE action='tenant.logo_updated'),count(*) FILTER(WHERE action='tenant.logo_removed') FROM app.audit_log WHERE tenant_id=$1`, p.TenantID).Scan(&updated, &removed)
	}); err != nil {
		t.Fatal(err)
	}
	if updated != 2 || removed != 1 {
		t.Fatalf("updated=%d removed=%d", updated, removed)
	}
}
func TestLogoMissingObjectRereadsOnce(t *testing.T) {
	for _, mode := range []string{"absent", "missing", "replace", "remove", "replacement_missing"} {
		t.Run(mode, func(t *testing.T) {
			_, _, r := newRegistrationServices(t)
			f := &contextLogoStorage{logoStorage: newLogoStorage()}
			var logs bytes.Buffer
			svc := logoService(t, r, audit.NewRecorder(), f, &logs)
			p := companyAdmin(t, svc)
			if mode == "absent" {
				_, err := svc.GetLogo(context.Background(), p, "")
				if !errors.Is(err, tenant.ErrLogoNotFound) {
					t.Fatalf("sentinel=%v", err)
				}
				return
			}
			if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
				t.Fatal(err)
			}
			old, _ := companyRow(t, r, p)
			var newKey string
			data := pngLogo(t, 11, 10, 0)
			if mode == "missing" {
				if err := f.logoStorage.Delete(context.Background(), *old); err != nil {
					t.Fatal(err)
				}
			} else {
				f.onGet = func(_ string) error {
					f.onGet = nil
					if mode == "remove" {
						return svc.RemoveLogo(context.Background(), p, identity.RequestMeta{})
					}
					if _, err := svc.SetLogo(context.Background(), p, data, identity.RequestMeta{}); err != nil {
						return err
					}
					key, _ := companyRow(t, r, p)
					newKey = *key
					if mode == "replacement_missing" {
						return f.logoStorage.Delete(context.Background(), newKey)
					}
					return nil
				}
			}
			out, err := svc.GetLogo(context.Background(), p, "")
			if mode == "replace" {
				if err != nil {
					t.Fatal(err)
				}
				body, readErr := io.ReadAll(out.Body)
				out.Body.Close()
				if readErr != nil || !bytes.Equal(body, data) || len(f.gets) != 2 || !strings.Contains(newKey, strings.Trim(out.ETag, `"`)) {
					t.Fatalf("replacement read=%+v gets=%v err=%v", out, f.gets, readErr)
				}
			} else if !errors.Is(err, tenant.ErrLogoNotFound) {
				t.Fatalf("missing logo=%v", err)
			}
			if mode == "missing" {
				for _, part := range []string{"level=ERROR", "event=logo_object_missing", "tenant_id=" + p.TenantID.String(), "object_key=" + *old} {
					if !strings.Contains(logs.String(), part) {
						t.Fatalf("missing log field %s: %s", part, logs.String())
					}
				}
			} else if strings.Contains(logs.String(), "level=ERROR") {
				t.Fatal(logs.String())
			}
			if mode == "replacement_missing" && len(f.gets) != 2 {
				t.Fatalf("retry count=%d", len(f.gets))
			}
		})
	}
}

type failedLogoAudit struct{}

func (failedLogoAudit) Record(context.Context, db.Tx, audit.Entry) error {
	return errors.New("test: audit failure")
}
func TestLogoSafeRollbackAndDeleteLogs(t *testing.T) {
	for _, reason := range []string{"compensation", "replaced"} {
		t.Run(reason, func(t *testing.T) {
			_, _, real := newRegistrationServices(t)
			f := newLogoStorage()
			var logs bytes.Buffer
			svc := logoService(t, real, audit.NewRecorder(), f, &logs)
			p := companyAdmin(t, svc)
			if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
				t.Fatal(err)
			}
			old, _ := companyRow(t, real, p)
			f.deleteErr = objectstore.ErrUnavailable
			r := &uncertainLogoRunner{TxRunner: real, mode: "committed", cancel: func() {}}
			if reason == "compensation" {
				svc = logoService(t, r, failedLogoAudit{}, f, &logs)
				if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 11, 10, 0), identity.RequestMeta{}); err == nil {
					t.Fatal("audit failure not returned")
				}
				if r.calls != 1 {
					t.Fatal("safe rollback re-read unnecessarily")
				}
				current, _ := companyRow(t, real, p)
				if *current != *old {
					t.Fatal("safe rollback changed reference")
				}
			} else {
				if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 11, 10, 0), identity.RequestMeta{}); err != nil {
					t.Fatal(err)
				}
			}
			attempted := f.deletes[len(f.deletes)-1]
			for _, part := range []string{"level=WARN", "event=logo_delete_failed", "reason=" + reason, "object_key=" + attempted} {
				if !strings.Contains(logs.String(), part) {
					t.Fatalf("delete log lacks %s: %s", part, logs.String())
				}
			}
		})
	}
}
