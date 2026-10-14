//go:build bench

package phase9bench_test

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type sampleKey struct{}
type spanKey struct{}
type acquireKey struct{}

type event struct {
	ID         string
	PID        uint32
	Label      string
	Start, End time.Time
	Err        error
}

type sample struct {
	ID                                     string
	Start, End, CallbackStart, CallbackEnd time.Time
	Err                                    error
	mu                                     sync.Mutex
	Events                                 []event
}

func currentSample(ctx context.Context) *sample { s, _ := ctx.Value(sampleKey{}).(*sample); return s }

var sqlcName = regexp.MustCompile(`^-- name: ([A-Za-z][A-Za-z0-9_]*) :`)
var tenantRole = regexp.MustCompile(`^crm_t_[0-9a-f]{32}$`)

var knownLabels = map[string]bool{
	"begin": true, "commit": true, "rollback": true, "acquire": true,
	"set_role:crm_signup": true, "set_role:crm_auth": true, "set_role:tenant": true,
	"SetSignupLockTimeout": true, "ProvisionTenantRole": true, "InsertTenant": true,
	"InsertFirstAdmin": true, "GetUserEmail": true, "InsertVerificationToken": true,
	"InsertMessage": true, "InsertSession": true, "InsertAudit": true,
}

// This is the exact catalog read + SET text of platform/db.setRole, without changing production.
func roleSQL(role string) string {
	return "SELECT 1 FROM pg_catalog.pg_auth_members LIMIT 0; SET LOCAL ROLE " + pgx.Identifier{role}.Sanitize()
}

func labelSQL(sql string) string {
	sql = strings.TrimSpace(sql)
	if match := sqlcName.FindStringSubmatch(sql); len(match) == 2 {
		if knownLabels[match[1]] {
			return match[1]
		}
		return "unknown"
	}
	switch strings.ToLower(sql) {
	case "begin", "commit", "rollback":
		return strings.ToLower(sql)
	}
	const prefix = "SELECT 1 FROM pg_catalog.pg_auth_members LIMIT 0; SET LOCAL ROLE "
	if strings.HasPrefix(sql, prefix) {
		role := strings.Trim(strings.TrimPrefix(sql, prefix), `"`)
		if role == "crm_signup" || role == "crm_auth" {
			return "set_role:" + role
		}
		if tenantRole.MatchString(role) {
			return "set_role:tenant"
		}
	}
	return "unknown"
}

type benchTracer struct {
	mu     sync.Mutex
	origin time.Time
	file   *os.File
	csv    *csv.Writer
	err    error
}

var _ pgx.QueryTracer = (*benchTracer)(nil)
var _ pgxpool.AcquireTracer = (*benchTracer)(nil)

func (tr *benchTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if s := currentSample(ctx); s != nil {
		return context.WithValue(ctx, spanKey{}, event{ID: s.ID, PID: conn.PgConn().PID(), Label: labelSQL(data.SQL), Start: time.Now()})
	}
	return ctx
}
func (tr *benchTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if e, ok := ctx.Value(spanKey{}).(event); ok {
		e.End = time.Now()
		e.Err = data.Err
		tr.record(ctx, e)
	}
}
func (tr *benchTracer) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	if s := currentSample(ctx); s != nil {
		return context.WithValue(ctx, acquireKey{}, event{ID: s.ID, Label: "acquire", Start: time.Now()})
	}
	return ctx
}
func (tr *benchTracer) TraceAcquireEnd(ctx context.Context, _ *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	if e, ok := ctx.Value(acquireKey{}).(event); ok {
		e.End = time.Now()
		e.Err = data.Err
		if data.Conn != nil {
			e.PID = data.Conn.PgConn().PID()
		}
		tr.record(ctx, e)
	}
}
func (tr *benchTracer) record(ctx context.Context, e event) {
	s := currentSample(ctx)
	if s == nil {
		tr.mu.Lock()
		tr.err = fmt.Errorf("trace event has no sample context")
		tr.mu.Unlock()
		return
	}
	s.mu.Lock()
	s.Events = append(s.Events, e)
	s.mu.Unlock()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.csv == nil {
		return
	}
	errText := ""
	if e.Err != nil {
		errText = e.Err.Error()
	}
	if err := tr.csv.Write([]string{e.ID, strconv.FormatUint(uint64(e.PID), 10), e.Label, strconv.FormatInt(e.Start.Sub(tr.origin).Nanoseconds(), 10), strconv.FormatInt(e.End.Sub(tr.origin).Nanoseconds(), 10), errText}); err != nil {
		tr.err = err
	}
}
func (tr *benchTracer) close() error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.csv != nil {
		tr.csv.Flush()
		if err := tr.csv.Error(); err != nil {
			tr.err = err
		}
	}
	if tr.file != nil {
		if err := tr.file.Close(); err != nil {
			tr.err = err
		}
	}
	return tr.err
}

type measuredRunner struct{ db.TxRunner }

func (r measuredRunner) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	s := currentSample(ctx)
	err := r.TxRunner.InSystemTx(ctx, role, func(ctx context.Context, tx db.Tx) error {
		if s != nil {
			s.CallbackStart = time.Now()
		}
		return fn(ctx, tx)
	})
	if s != nil {
		s.CallbackEnd = time.Now()
	}
	return err
}

func sampleIntervals(s *sample) (map[string]time.Duration, error) {
	events := s.Events
	byLabel := map[string][]event{}
	var pid uint32
	for _, e := range events {
		if e.ID == "" || e.ID != s.ID || !knownLabels[e.Label] || e.End.Before(e.Start) {
			return nil, fmt.Errorf("invalid trace event: %+v", e)
		}
		if e.PID == 0 {
			return nil, fmt.Errorf("event has no backend PID: %s", e.Label)
		}
		if pid == 0 {
			pid = e.PID
		}
		if pid != e.PID {
			return nil, fmt.Errorf("sample changed backend: %d -> %d", pid, e.PID)
		}
		byLabel[e.Label] = append(byLabel[e.Label], e)
	}
	required := []string{"acquire", "begin", "set_role:crm_signup", "SetSignupLockTimeout", "ProvisionTenantRole"}
	if s.Err == nil {
		required = append(required, "set_role:tenant", "GetUserEmail", "InsertTenant", "InsertFirstAdmin", "InsertVerificationToken", "InsertMessage", "InsertSession", "InsertAudit", "commit")
	} else {
		if !errors.Is(s.Err, db.ErrUnavailable) {
			return nil, fmt.Errorf("sample %s is not a 503 rejection: %w", s.ID, s.Err)
		}
		required = append(required, "rollback")
	}
	if len(events) != len(required) {
		return nil, fmt.Errorf("sample %s: event count=%d, required=%d", s.ID, len(events), len(required))
	}
	for _, label := range required {
		if len(byLabel[label]) != 1 {
			return nil, fmt.Errorf("sample %s: %s count=%d", s.ID, label, len(byLabel[label]))
		}
	}
	result := map[string]time.Duration{"pool_wait": byLabel["acquire"][0].End.Sub(byLabel["acquire"][0].Start), "pre_callback": byLabel["set_role:crm_signup"][0].End.Sub(byLabel["acquire"][0].End), "provision": byLabel["ProvisionTenantRole"][0].End.Sub(byLabel["ProvisionTenantRole"][0].Start), "callback": s.CallbackEnd.Sub(s.CallbackStart), "end_to_end": s.End.Sub(s.Start)}
	if s.Err == nil {
		if len(byLabel["commit"]) != 1 {
			return nil, fmt.Errorf("sample %s: commit count=%d", s.ID, len(byLabel["commit"]))
		}
		if len(byLabel["rollback"]) != 0 {
			return nil, fmt.Errorf("successful sample rolled back")
		}
		result["hold"] = byLabel["commit"][0].End.Sub(byLabel["ProvisionTenantRole"][0].Start)
	} else {
		// A rejected registration cannot have a commit/hold. Preserve its other intervals and
		// the 503 count; do not invent a hold until rollback or discard its end-to-end latency.
		if len(byLabel["commit"]) != 0 || len(byLabel["rollback"]) != 1 {
			return nil, fmt.Errorf("failed sample lacks one rollback")
		}
	}
	for label, spans := range byLabel {
		for _, e := range spans {
			result["statement:"+label] += e.End.Sub(e.Start)
		}
	}
	for label, d := range result {
		if d < 0 {
			return nil, fmt.Errorf("negative interval %s", label)
		}
	}
	return result, nil
}

func noOverlap(samples []*sample) error {
	var previous time.Time
	for _, s := range samples {
		var start, end time.Time
		for _, e := range s.Events {
			if e.Label == "ProvisionTenantRole" {
				start = e.Start
			}
			if e.Label == "commit" {
				end = e.End
			}
		}
		if start.IsZero() || end.IsZero() || start.Before(previous) {
			return fmt.Errorf("M-2 holds overlap or lack endpoints: %s", s.ID)
		}
		previous = end
	}
	return nil
}

type distribution struct {
	Samples int     `json:"samples"`
	P50MS   float64 `json:"p50_ms"`
	P95MS   float64 `json:"p95_ms"`
	MaxMS   float64 `json:"max_ms"`
}

func summarize(values []time.Duration) distribution {
	if len(values) == 0 {
		return distribution{}
	}
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	ms := func(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }
	return distribution{len(ordered), ms(ordered[(50*len(ordered)+99)/100-1]), ms(ordered[(95*len(ordered)+99)/100-1]), ms(ordered[len(ordered)-1])}
}
func collect(samples []*sample) (map[string][]time.Duration, error) {
	values := map[string][]time.Duration{}
	for _, s := range samples {
		intervals, err := sampleIntervals(s)
		if err != nil {
			return nil, err
		}
		for label, value := range intervals {
			values[label] = append(values[label], value)
		}
	}
	return values, nil
}

func TestBenchTracerSyntheticControls(t *testing.T) {
	now := time.Now()
	valid := func() *sample {
		s := &sample{ID: "control", Start: now, End: now.Add(20 * time.Millisecond), CallbackStart: now.Add(5 * time.Millisecond), CallbackEnd: now.Add(19 * time.Millisecond)}
		for i, label := range []string{"acquire", "begin", "set_role:crm_signup", "SetSignupLockTimeout", "ProvisionTenantRole", "set_role:tenant", "GetUserEmail", "InsertTenant", "InsertFirstAdmin", "InsertVerificationToken", "InsertMessage", "InsertSession", "InsertAudit", "commit"} {
			s.Events = append(s.Events, event{ID: s.ID, PID: 42, Label: label, Start: now.Add(time.Duration(i) * time.Millisecond), End: now.Add(time.Duration(i+1) * time.Millisecond)})
		}
		return s
	}
	if _, err := sampleIntervals(valid()); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"unknown-label", "missing-id", "duplicate-begin", "missing-provision", "missing-commit", "different-pid"} {
		t.Run(bad, func(t *testing.T) {
			s := valid()
			switch bad {
			case "unknown-label":
				s.Events[0].Label = "unknown"
			case "missing-id":
				s.Events[0].ID = ""
			case "duplicate-begin":
				s.Events = append(s.Events, s.Events[1])
			case "missing-provision":
				s.Events = append(s.Events[:4], s.Events[5:]...)
			case "missing-commit":
				s.Events = s.Events[:len(s.Events)-1]
			case "different-pid":
				s.Events[0].PID = 43
			}
			if _, err := sampleIntervals(s); err == nil {
				t.Fatal("self-control accepted corrupt instrumentation")
			}
		})
	}
	a, b := valid(), valid()
	b.ID = "overlap"
	if err := noOverlap([]*sample{a, b}); err == nil {
		t.Fatal("self-control accepted overlapping holds")
	}
	for _, tc := range []struct{ sql, label string }{{"begin", "begin"}, {"COMMIT", "commit"}, {"-- name: InsertTenant :exec\nINSERT...", "InsertTenant"}, {roleSQL("crm_signup"), "set_role:crm_signup"}, {roleSQL("crm_auth"), "set_role:crm_auth"}, {roleSQL("crm_t_0123456789abcdef0123456789abcdef"), "set_role:tenant"}, {"SELECT 42", "unknown"}} {
		if got := labelSQL(tc.sql); got != tc.label {
			t.Errorf("label=%s want=%s", got, tc.label)
		}
	}
}
