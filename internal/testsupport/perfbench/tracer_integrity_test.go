//go:build bench

package perfbench_test

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
)

func completeRegistrationSample() *sample {
	now := time.Now()
	s := &sample{ID: "complete", Start: now, End: now.Add(20 * time.Millisecond), CallbackStart: now.Add(3 * time.Millisecond), CallbackEnd: now.Add(19 * time.Millisecond)}
	for i, label := range []string{"acquire", "begin", "set_role:crm_signup", "SetSignupLockTimeout", "ProvisionTenantRole", "set_role:tenant", "GetUserEmail", "InsertTenant", "InsertFirstAdmin", "InsertVerificationToken", "InsertMessage", "InsertSession", "InsertAudit", "commit"} {
		s.Events = append(s.Events, event{ID: s.ID, PID: 42, Label: label, Start: now.Add(time.Duration(i) * time.Millisecond), End: now.Add(time.Duration(i+1) * time.Millisecond)})
	}
	return s
}

// seededRegistrationSample is a successful registration whose trace carries the four seed statements between
// InsertAudit and COMMIT, one millisecond each.
func seededRegistrationSample() *sample {
	base := completeRegistrationSample()
	base.Seeded = true
	commit := base.Events[len(base.Events)-1]
	events := append([]event(nil), base.Events[:len(base.Events)-1]...)
	at := commit.Start
	for _, label := range seedLabels {
		events = append(events, event{ID: base.ID, PID: 42, Label: label, Start: at, End: at.Add(time.Millisecond)})
		at = at.Add(time.Millisecond)
	}
	commit.Start, commit.End = at, at.Add(time.Millisecond)
	base.Events = append(events, commit)
	base.End = commit.End.Add(time.Millisecond)
	base.CallbackEnd = base.End
	return base
}

// T-002-B307 (DD-3, R-1): the seed interval spans the four Seed statements, and a seeded trace missing,
// duplicating, reordering or misplacing any of them is rejected.
func TestBenchTracerSeedControls(t *testing.T) {
	good := seededRegistrationSample()
	intervals, err := sampleIntervals(good)
	if err != nil {
		t.Fatal(err)
	}
	if intervals["seed"] != 4*time.Millisecond {
		t.Fatalf("seed=%s, want 4ms", intervals["seed"])
	}
	if _, ok := intervals["statement:SeedStages"]; !ok {
		t.Fatal("seed statements are not counted in the per-label breakdown")
	}
	plain, err := sampleIntervals(completeRegistrationSample())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plain["seed"]; ok {
		t.Fatal("a registration without the real seeder reports a seed interval")
	}
	indexOf := func(s *sample, label string) int {
		for i, e := range s.Events {
			if e.Label == label {
				return i
			}
		}
		t.Fatal("no event " + label)
		return -1
	}
	for _, label := range seedLabels {
		t.Run("missing/"+label, func(t *testing.T) {
			bad := seededRegistrationSample()
			i := indexOf(bad, label)
			bad.Events = append(bad.Events[:i], bad.Events[i+1:]...)
			if _, err := sampleIntervals(bad); err == nil {
				t.Fatal("control accepted missing seed statement: " + label)
			}
		})
		t.Run("duplicate/"+label, func(t *testing.T) {
			bad := seededRegistrationSample()
			bad.Events = append(bad.Events, bad.Events[indexOf(bad, label)])
			if _, err := sampleIntervals(bad); err == nil {
				t.Fatal("control accepted duplicate seed statement: " + label)
			}
		})
		t.Run("unflagged/"+label, func(t *testing.T) {
			bad := seededRegistrationSample()
			bad.Seeded = false
			if _, err := sampleIntervals(bad); err == nil {
				t.Fatal("control accepted seed statements in a registration that did not use the real seeder")
			}
		})
	}
	t.Run("reordered", func(t *testing.T) {
		bad := seededRegistrationSample()
		a, b := indexOf(bad, "SeedStages"), indexOf(bad, "SeedCategories")
		bad.Events[a].Start, bad.Events[b].Start = bad.Events[b].Start, bad.Events[a].Start
		bad.Events[a].End, bad.Events[b].End = bad.Events[b].End, bad.Events[a].End
		if _, err := sampleIntervals(bad); err == nil {
			t.Fatal("control accepted reordered seed statements")
		}
	})
	t.Run("after-commit", func(t *testing.T) {
		bad := seededRegistrationSample()
		i, c := indexOf(bad, "SeedCatalogItems"), indexOf(bad, "commit")
		bad.Events[i].Start, bad.Events[i].End = bad.Events[c].End, bad.Events[c].End.Add(time.Millisecond)
		if _, err := sampleIntervals(bad); err == nil {
			t.Fatal("control accepted a seed statement after COMMIT")
		}
	})
	t.Run("before-tenant-role", func(t *testing.T) {
		bad := seededRegistrationSample()
		i, r := indexOf(bad, "SeedTenantSettings"), indexOf(bad, "set_role:tenant")
		bad.Events[i].Start, bad.Events[i].End = bad.Events[r].Start.Add(-2*time.Millisecond), bad.Events[r].Start.Add(-time.Millisecond)
		if _, err := sampleIntervals(bad); err == nil {
			t.Fatal("control accepted a seed statement before the tenant role switch")
		}
	})
	t.Run("before-hold", func(t *testing.T) {
		// The hold starts at ProvisionTenantRole: a seed statement that starts before it would be
		// measured partly outside the hold.
		bad := seededRegistrationSample()
		p, s := indexOf(bad, "ProvisionTenantRole"), indexOf(bad, "SeedTenantSettings")
		bad.Events[p].Start = bad.Events[s].Start.Add(time.Microsecond)
		bad.Events[p].End = bad.Events[p].Start.Add(time.Microsecond)
		if _, err := sampleIntervals(bad); err == nil {
			t.Fatal("control accepted a seed statement that starts before the hold")
		}
	})
	for _, tc := range []struct{ sql, label string }{{"-- name: SeedTenantSettings :exec\nINSERT", "SeedTenantSettings"}, {"-- name: SeedStages :exec\nINSERT", "SeedStages"}, {"-- name: SeedCategories :exec\nINSERT", "SeedCategories"}, {"-- name: SeedCatalogItems :exec\nINSERT", "SeedCatalogItems"}, {"-- name: SeedOther :exec\nINSERT", "unknown"}} {
		if got := labelSQL(tc.sql); got != tc.label {
			t.Errorf("label=%s want=%s", got, tc.label)
		}
	}
}

func TestBenchTracerFullSetControls(t *testing.T) {
	s := completeRegistrationSample()
	if _, err := sampleIntervals(s); err != nil {
		t.Fatal(err)
	}
	for index, e := range s.Events {
		t.Run("missing/"+e.Label, func(t *testing.T) {
			bad := completeRegistrationSample()
			bad.Events = append(bad.Events[:index], bad.Events[index+1:]...)
			if _, err := sampleIntervals(bad); err == nil {
				t.Fatal("control accepted missing label: " + e.Label)
			}
		})
		t.Run("duplicate/"+e.Label, func(t *testing.T) {
			bad := completeRegistrationSample()
			bad.Events = append(bad.Events, bad.Events[index])
			if _, err := sampleIntervals(bad); err == nil {
				t.Fatal("control accepted duplicate label: " + e.Label)
			}
		})
	}
	failed := completeRegistrationSample()
	failed.Err = db.ErrUnavailable
	failed.Events = failed.Events[:5]
	rollback := failed.Events[4]
	rollback.Label = "rollback"
	rollback.Start, rollback.End = rollback.End, rollback.End.Add(time.Millisecond)
	failed.Events = append(failed.Events, rollback)
	if _, err := sampleIntervals(failed); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"commit", "set_role:tenant"} {
		t.Run("rejected-with/"+label, func(t *testing.T) {
			bad := &sample{ID: failed.ID, Start: failed.Start, End: failed.End, CallbackStart: failed.CallbackStart, CallbackEnd: failed.CallbackEnd, Err: failed.Err}
			bad.Events = append(append([]event(nil), failed.Events...), event{ID: bad.ID, PID: 42, Label: label, Start: rollback.End, End: rollback.End})
			if _, err := sampleIntervals(bad); err == nil {
				t.Fatal("rejected sample accepted extra label: " + label)
			}
		})
	}
	copyRejected := func() *sample {
		return &sample{ID: failed.ID, Start: failed.Start, End: failed.End, CallbackStart: failed.CallbackStart, CallbackEnd: failed.CallbackEnd, Err: failed.Err, Events: append([]event(nil), failed.Events...)}
	}
	t.Run("rejected-without-rollback", func(t *testing.T) {
		bad := copyRejected()
		bad.Events = bad.Events[:len(bad.Events)-1]
		if _, err := sampleIntervals(bad); err == nil {
			t.Fatal("rejected sample accepted without rollback")
		}
	})
	t.Run("rejected-commit-instead-of-rollback", func(t *testing.T) {
		bad := copyRejected()
		bad.Events[len(bad.Events)-1].Label = "commit"
		if _, err := sampleIntervals(bad); err == nil {
			t.Fatal("rejected sample accepted commit instead of rollback")
		}
	})
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"canceled", db.ErrCanceled},
		{"privilege", db.ErrPrivilege},
		{"plain", errors.New("unrelated rejection")},
	} {
		t.Run("rejected-error/"+tc.name, func(t *testing.T) {
			bad := copyRejected()
			bad.Err = tc.err
			if _, err := sampleIntervals(bad); err == nil {
				t.Fatal("rejected sample accepted non-503 error: " + tc.name)
			}
		})
	}
	t.Run("wrapped-unavailable", func(t *testing.T) {
		wrapped := copyRejected()
		wrapped.Err = fmt.Errorf("registration: %w", db.ErrUnavailable)
		if _, err := sampleIntervals(wrapped); err != nil {
			t.Fatalf("control rejected wrapped 503 error: %v", err)
		}
	})
}

// Recheck preserved traces without creating a container or remeasuring performance.
// CSV lacks Register/callback boundaries; the reconstruction validates the recorded
// event set, IDs, PID and intervals only, and never recomputes the reported targets.
func TestBenchTracerCSVControls(t *testing.T) {
	paths := filepath.SplitList(os.Getenv("PERFBENCH_TRACE_FILES"))
	if len(paths) == 0 {
		t.Skip("set PERFBENCH_TRACE_FILES to preserved benchmark CSV paths")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			reader := csv.NewReader(file)
			header, err := reader.Read()
			if err != nil || strings.Join(header, ",") != "sample_id,backend_pid,label,start_monotonic_ns,end_monotonic_ns,error" {
				t.Fatalf("CSV header: %v, %v", header, err)
			}
			samples := map[string]*sample{}
			origin := time.Unix(0, 0)
			events := 0
			for {
				row, err := reader.Read()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil || len(row) != 6 {
					t.Fatalf("CSV row: %v", err)
				}
				pid, err := strconv.ParseUint(row[1], 10, 32)
				if err != nil {
					t.Fatal(err)
				}
				start, err := strconv.ParseInt(row[3], 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				end, err := strconv.ParseInt(row[4], 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				s := samples[row[0]]
				if s == nil {
					s = &sample{ID: row[0], Seeded: strings.HasPrefix(row[0], "M7/real/") || strings.HasPrefix(row[0], "M7/warmup/real/")}
					samples[row[0]] = s
				}
				e := event{ID: row[0], PID: uint32(pid), Label: row[2], Start: origin.Add(time.Duration(start)), End: origin.Add(time.Duration(end))}
				if row[5] != "" {
					e.Err = fmt.Errorf("CSV query: %s", row[5])
				}
				s.Events = append(s.Events, e)
				events++
				switch e.Label {
				case "acquire":
					s.Start = e.Start
				case "set_role:crm_signup":
					s.CallbackStart = e.End
				case "commit", "rollback":
					s.End, s.CallbackEnd = e.End, e.End
					if e.Label == "rollback" {
						s.Err = db.ErrUnavailable
					}
				}
			}
			confirmed, rejected := 0, 0
			var isolated []*sample
			for id, s := range samples {
				if _, err := sampleIntervals(s); err != nil {
					t.Fatalf("CSV sample %s: %v", id, err)
				}
				if s.Err == nil {
					confirmed++
				} else {
					rejected++
				}
				if strings.HasPrefix(id, "M2/") || strings.HasPrefix(id, "M7/real/") || strings.HasPrefix(id, "M7/noop/") {
					isolated = append(isolated, s)
				}
			}
			sort.Slice(isolated, func(i, j int) bool { return isolated[i].Start.Before(isolated[j].Start) })
			if len(samples) == 0 || len(isolated) != 100 {
				t.Fatalf("incomplete CSV: samples=%d isolated=%d", len(samples), len(isolated))
			}
			if err := noOverlap(isolated); err != nil {
				t.Fatal(err)
			}
			t.Logf("CSV controls path=%s confirmed=%d rejected=%d events=%d isolated=%d: complete labels/IDs/PID and non-overlap of the 100 isolated registrations", path, confirmed, rejected, events, len(isolated))
		})
	}
}
