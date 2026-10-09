//go:build bench

package phase9bench_test

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
}

// Recheck preserved traces without creating a container or remeasuring performance.
// CSV lacks Register/callback boundaries; the reconstruction validates the recorded
// event set, IDs, PID and intervals only, and never recomputes the reported targets.
func TestBenchTracerCSVControls(t *testing.T) {
	paths := filepath.SplitList(os.Getenv("PHASE9_BENCH_TRACE_FILES"))
	if len(paths) == 0 {
		t.Skip("set PHASE9_BENCH_TRACE_FILES to preserved benchmark CSV paths")
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
					s = &sample{ID: row[0]}
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
				if strings.HasPrefix(id, "M2/") {
					isolated = append(isolated, s)
				}
			}
			sort.Slice(isolated, func(i, j int) bool { return isolated[i].Start.Before(isolated[j].Start) })
			if len(samples) == 0 || len(isolated) != 100 {
				t.Fatalf("incomplete CSV: samples=%d M2=%d", len(samples), len(isolated))
			}
			if err := noOverlap(isolated); err != nil {
				t.Fatal(err)
			}
			t.Logf("CSV controls path=%s confirmed=%d rejected=%d events=%d M2=%d: complete labels/IDs/PID and M2 non-overlap", path, confirmed, rejected, events, len(isolated))
		})
	}
}
