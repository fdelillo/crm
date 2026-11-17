//go:build bench

package perfbench_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/tenant"
)

const seedingTemplate = "aluminum_carpentry"

// registerTemplated registers one company through svc with the largest industry template.
func (e *environment) registerTemplated(ctx context.Context, svc *tenant.Service, id string, seeded bool) *sample {
	in := signupInput()
	in.IndustryTemplateCode = seedingTemplate
	s := &sample{ID: id, Start: time.Now(), Seeded: seeded}
	_, err := svc.Register(context.WithValue(ctx, sampleKey{}, s), in, identity.RequestMeta{})
	s.End = time.Now()
	s.Err = err
	return s
}

// gateVerdict applies the decision rule of T-002-B307 as written in tasks.md; it adds no tolerance.
func gateVerdict(seedP95, realHoldP95, noopHoldP95 float64) (conditions [3]bool, passed bool) {
	conditions = [3]bool{seedP95 <= 10, realHoldP95-noopHoldP95 <= 10, realHoldP95 < 140}
	return conditions, conditions[0] && conditions[1] && conditions[2]
}

// m7WarmupPerPath is how many unmeasured registrations each path (real seeder and noop) runs before M-7,
// so neither group carries the first execution of its statements.
const m7WarmupPerPath = 5

// m7Schedule returns the order of the measured registrations (true = real seeder) for pairs pairs. Each pair
// holds one of each, and the order inside the pair alternates (real-noop, noop-real, ...), so each group goes
// first in half of the pairs and neither group always follows the other.
func m7Schedule(pairs int) []bool {
	order := make([]bool, 0, 2*pairs)
	for i := 0; i < pairs; i++ {
		realFirst := i%2 == 0
		order = append(order, realFirst, !realFirst)
	}
	return order
}

// T-002-B307 (DD-3, R-1), Codex review of PR #62: M-7 counterbalances the order of real and noop.
func TestSeedingScheduleIsCounterbalanced(t *testing.T) {
	order := m7Schedule(50)
	if len(order) != 100 {
		t.Fatalf("len=%d, want 100", len(order))
	}
	var real, realFirst int
	follows := map[[2]bool]int{}
	for i, isReal := range order {
		if isReal {
			real++
		}
		if i%2 == 0 {
			if order[i] == order[i+1] {
				t.Fatalf("pair %d does not hold one of each", i/2)
			}
			if isReal {
				realFirst++
			}
		}
		if i > 0 {
			follows[[2]bool{order[i-1], isReal}]++
		}
	}
	if real != 50 || realFirst != 25 {
		t.Fatalf("real=%d (want 50), real first in %d pairs (want 25)", real, realFirst)
	}
	// Each group is preceded by itself and by the other about equally often (49 or 50 transitions each).
	if r, n := follows[[2]bool{true, true}]+follows[[2]bool{false, true}], follows[[2]bool{true, true}]; n < r/2-1 || n > r/2+1 {
		t.Fatalf("real preceded by real %d of %d times", n, r)
	}
	if r, n := follows[[2]bool{false, false}]+follows[[2]bool{true, false}], follows[[2]bool{false, false}]; n < r/2-1 || n > r/2+1 {
		t.Fatalf("noop preceded by noop %d of %d times", n, r)
	}
}

// T-002-B307 (DD-3, R-1): the decision rule is the one fixed in tasks.md, including its documented failing example.
func TestSeedingGateDecisionRule(t *testing.T) {
	for _, tc := range []struct {
		name             string
		seed, real, noop float64
		want             [3]bool
		wantPass         bool
	}{
		{"all hold", 8, 130, 125, [3]bool{true, true, true}, true},
		{"documented failing example", 8, 141, 133, [3]bool{true, true, false}, false},
		{"seed at the limit", 10, 100, 90, [3]bool{true, true, true}, true},
		{"seed over the limit", 10.001, 100, 99, [3]bool{false, true, true}, false},
		{"delta over the limit", 5, 120, 109.9, [3]bool{true, false, true}, false},
		{"absolute at 140 fails", 5, 140, 139, [3]bool{true, true, false}, false},
	} {
		got, pass := gateVerdict(tc.seed, tc.real, tc.noop)
		if got != tc.want || pass != tc.wantPass {
			t.Errorf("%s: conditions=%v pass=%t, want %v %t", tc.name, got, pass, tc.want, tc.wantPass)
		}
	}
}

// T-002-B307 (DD-3, R-1): M-7, the cost of the seed inside the registration-lock hold with 10,000 companies.
// Run alone with: go test -tags=bench -run TestSeedingCostGate ./internal/testsupport/perfbench/
func TestSeedingCostGate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()
	e := newEnvironment(t, ctx)
	// The same population as T-B910 M-1: 10,000 tenant roles created by sequential registrations.
	for i := 0; i < 10000; i++ {
		requireSample(t, e.registerTemplated(ctx, e.seed, fmt.Sprintf("M7/load/%05d", i+1), false))
	}
	var count int
	if err := e.super.QueryRow(ctx, "SELECT count(*) FROM app.tenants").Scan(&count); err != nil || count != 10000 {
		t.Fatalf("tenants count=%d err=%v", count, err)
	}
	// Warm both paths outside the samples: the load above only ran the noop one.
	for i := 0; i < m7WarmupPerPath; i++ {
		requireSample(t, e.registerTemplated(ctx, e.templated, fmt.Sprintf("M7/warmup/real/%d", i+1), true))
		requireSample(t, e.registerTemplated(ctx, e.seed, fmt.Sprintf("M7/warmup/noop/%d", i+1), false))
	}
	// M-7: 100 sequential registrations, 50 pairs of real TemplateSeeder and NoopSeeder with the order inside
	// each pair alternating (m7Schedule), no other activity.
	var real, noop, all []*sample
	for i, isReal := range m7Schedule(50) {
		if isReal {
			s := e.registerTemplated(ctx, e.templated, fmt.Sprintf("M7/real/%03d", i+1), true)
			requireSample(t, s)
			real = append(real, s)
			all = append(all, s)
		} else {
			s := e.registerTemplated(ctx, e.seed, fmt.Sprintf("M7/noop/%03d", i+1), false)
			requireSample(t, s)
			noop = append(noop, s)
			all = append(all, s)
		}
	}
	if err := noOverlap(all); err != nil {
		t.Fatal(err)
	}
	// Prove the real seeder ran and the noop one did not, in the database and not only in the trace.
	var seededRows, emptyTenants int
	if err := e.super.QueryRow(ctx, `SELECT count(*) FROM app.tenants t WHERE EXISTS (SELECT 1 FROM app.catalog_items c WHERE c.tenant_id = t.id)`).Scan(&seededRows); err != nil {
		t.Fatal(err)
	}
	if err := e.super.QueryRow(ctx, `SELECT count(*) FROM app.tenants t WHERE NOT EXISTS (SELECT 1 FROM app.tenant_settings s WHERE s.tenant_id = t.id)`).Scan(&emptyTenants); err != nil {
		t.Fatal(err)
	}
	if seededRows != m7WarmupPerPath+len(real) || emptyTenants != 10000+m7WarmupPerPath+len(noop) {
		t.Fatalf("seeded tenants=%d (want %d), unseeded=%d (want %d)", seededRows, m7WarmupPerPath+len(real), emptyTenants, 10000+m7WarmupPerPath+len(noop))
	}
	realStats, noopStats := summarizeSamples(t, real), summarizeSamples(t, noop)
	if _, ok := noopStats["seed"]; ok {
		t.Fatal("noop group reports a seed interval")
	}
	result := map[string]any{
		"seed":      realStats["seed"],
		"hold_real": realStats["hold"],
		"hold_noop": noopStats["hold"],
		"samples":   map[string]int{"real": len(real), "noop": len(noop)},
	}
	conditions, passed := gateVerdict(realStats["seed"].P95MS, realStats["hold"].P95MS, noopStats["hold"].P95MS)
	delta := realStats["hold"].P95MS - noopStats["hold"].P95MS
	result["hold_p95_delta_ms"] = delta
	result["conditions"] = map[string]bool{"seed_p95_le_10ms": conditions[0], "hold_delta_le_10ms": conditions[1], "hold_real_p95_lt_140ms": conditions[2]}
	result["passed"] = passed
	logMeasurement(t, "M-7", result)
	if dir := os.Getenv("PERFBENCH_ARTIFACT_DIR"); dir != "" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "m7.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("BENCH gate T-002-B307 passed=%t conditions=%v", passed, conditions)
	if !passed {
		t.Errorf("BENCH gate T-002-B307 failed; report the numbers and stop: the decision is the user's")
	}
}
