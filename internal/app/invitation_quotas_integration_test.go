//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/google/uuid"
)

// Documentation addresses (RFC 5737): the IP of a request is its client address (DD-32).
const (
	ipOne   = "192.0.2.10"
	ipTwo   = "192.0.2.20"
	ipThree = "192.0.2.30"
)

func (e *apiEnv) invite(raw, ip, email string, want int) {
	e.t.Helper()
	e.doFrom(raw, ip, "POST", "/api/v1/users/invitations", `{"email":"`+email+`","role":"operator"}`, want)
}
func newEmail() string { return uuid.NewString() + "@example.test" }

// footprint counts what an accepted invitation leaves behind in a company.
func (e *apiEnv) footprint(tenantID uuid.UUID) [4]int {
	e.t.Helper()
	var n [4]int
	if err := e.runner.TxRunner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.users WHERE tenant_id=$1),
		  (SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1),
		  (SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1),
		  (SELECT count(*) FROM app.audit_log WHERE tenant_id=$1)`, tenantID).Scan(&n[0], &n[1], &n[2], &n[3])
	}); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// requireRateLimitedProblem checks the 429 of the limiters: a problem with code rate_limited and a
// Retry-After in whole seconds.
func requireRateLimitedProblem(t *testing.T, header string, body []byte) {
	t.Helper()
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &problem); err != nil || problem.Code != "rate_limited" {
		t.Fatalf("429 body: %s (%v)", body, err)
	}
	if seconds, err := strconv.Atoi(header); err != nil || seconds <= 0 {
		t.Fatalf("Retry-After=%q", header)
	}
}

func (e *apiEnv) invite429(raw, ip, email string) {
	e.t.Helper()
	rec := e.doFrom(raw, ip, "POST", "/api/v1/users/invitations", `{"email":"`+email+`","role":"operator"}`, 429)
	requireRateLimitedProblem(e.t, rec.Header().Get("Retry-After"), rec.Body.Bytes())
}

// tenSteps sends the ten requests of Q1 through send, alternating the two addresses, with every
// kind of answer: new emails, one reinvitation, an email of another company and an invalid body.
func (e *apiEnv) tenSteps(senders [2]string, otherEmail string) {
	e.t.Helper()
	ips := [2]string{ipOne, ipTwo}
	reinvited := newEmail()
	plan := []struct {
		body string
		want int
	}{}
	for i := 0; i < 5; i++ {
		plan = append(plan, struct {
			body string
			want int
		}{`{"email":"` + newEmail() + `","role":"operator"}`, 201})
	}
	plan = append(plan,
		struct {
			body string
			want int
		}{`{"email":"` + reinvited + `","role":"operator"}`, 201},
		struct {
			body string
			want int
		}{`{"email":"` + reinvited + `","role":"admin"}`, 200},
		struct {
			body string
			want int
		}{`{"email":"` + otherEmail + `","role":"operator"}`, 409},
		struct {
			body string
			want int
		}{`{"email":"not-an-email","role":"operator"}`, 422},
		struct {
			body string
			want int
		}{`{"email":`, 400}) // malformed JSON: it spends quota too
	if len(plan) != 10 {
		e.t.Fatalf("plan has %d steps", len(plan))
	}
	for i, step := range plan {
		e.doFrom(senders[i%2], ips[i%2], "POST", "/api/v1/users/invitations", step.body, step.want)
	}
}

// T-B1005 (Q1-Q8, DD-48): invitation and reactivation share one quota per company and one per IP.
func TestInvitationQuotas(t *testing.T) {
	t.Run("Q1 ten requests of every kind are answered normally and Q2 the eleventh is limited", func(t *testing.T) {
		e := newAPIEnv(t)
		other := e.newCompany()
		var otherEmail string
		if err := e.runner.TxRunner.InTenantTx(context.Background(), other.a.TenantID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, other.a.TenantID, other.a.UserID).Scan(&otherEmail)
		}); err != nil {
			t.Fatal(err)
		}
		e.tenSteps([2]string{e.aRaw, e.aRaw}, otherEmail)
		before := e.footprint(e.a.TenantID)
		for _, ip := range []string{ipOne, ipTwo} {
			e.invite429(e.aRaw, ip, newEmail())
		}
		if after := e.footprint(e.a.TenantID); after != before {
			t.Fatalf("rejected invitations left rows: %v -> %v", before, after)
		}
		// Q3: the quota of another company is independent and its IP still has room.
		e.invite(other.aRaw, ipOne, newEmail(), 201)
	})
	t.Run("Q1b two administrators share the company quota", func(t *testing.T) {
		e := newAPIEnv(t)
		other := e.newCompany()
		var otherEmail string
		if err := e.runner.TxRunner.InTenantTx(context.Background(), other.a.TenantID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, other.a.TenantID, other.a.UserID).Scan(&otherEmail)
		}); err != nil {
			t.Fatal(err)
		}
		e.tenSteps([2]string{e.aRaw, e.bRaw}, otherEmail)
		e.invite429(e.aRaw, ipOne, newEmail())
		e.invite429(e.bRaw, ipTwo, newEmail())
	})
	t.Run("Q4 reactivation and invitation share one company quota", func(t *testing.T) {
		e := newAPIEnv(t)
		member := e.member(e.a, newEmail(), authz.RoleOperator)
		if _, err := e.users.Deactivate(context.Background(), e.a, member.Principal.UserID, identity.RequestMeta{}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 10; i++ {
			e.invite(e.aRaw, ipOne, newEmail(), 201)
		}
		rec := e.doFrom(e.aRaw, ipOne, "POST", "/api/v1/users/"+member.Principal.UserID.String()+"/reactivate", "", 429)
		requireRateLimitedProblem(t, rec.Header().Get("Retry-After"), rec.Body.Bytes())
		if _, status := e.userState(member.Principal.UserID); status != "disabled" {
			t.Fatalf("the user is %s, want disabled", status)
		}
		// The other way around: the quota spent only by reactivations (any outcome counts).
		other := newAPIEnv(t)
		for i := 0; i < 10; i++ {
			other.doFrom(other.aRaw, ipTwo, "POST", "/api/v1/users/"+uuid.NewString()+"/reactivate", "", 404)
		}
		other.invite429(other.aRaw, ipTwo, newEmail())
	})
	t.Run("Q5 a token comes back after six minutes", func(t *testing.T) {
		e := newAPIEnv(t)
		for i := 0; i < 10; i++ {
			e.invite(e.aRaw, ipOne, newEmail(), 201)
		}
		e.invite429(e.aRaw, ipOne, newEmail())
		e.clock.advance(6 * time.Minute)
		e.invite(e.aRaw, ipOne, newEmail(), 201)
		e.invite429(e.aRaw, ipOne, newEmail())
	})
	t.Run("Q6 one address is limited across companies", func(t *testing.T) {
		e := newAPIEnv(t)
		companies := []twoAdminCompany{e.twoAdminCompany, e.newCompany(), e.newCompany()}
		for i := 0; i < 20; i++ {
			e.invite(companies[i%3].aRaw, ipOne, newEmail(), 201)
		}
		// The twenty-first company request has room in its company (7 of 10 used) but not in the IP.
		e.invite429(companies[0].aRaw, ipOne, newEmail())
		// The refusal by IP did not spend company quota: 3 of its 10 are left, from another address.
		for i := 0; i < 3; i++ {
			e.invite(companies[0].aRaw, ipTwo, newEmail(), 201)
		}
		e.invite429(companies[0].aRaw, ipTwo, newEmail())
	})
	t.Run("Q7 a refused permission does not spend quota", func(t *testing.T) {
		e := newAPIEnv(t)
		operator := e.member(e.a, newEmail(), authz.RoleOperator)
		for i := 0; i < 10; i++ {
			e.doFrom(operator.RawToken, ipOne, "POST", "/api/v1/users/invitations", `{"email":"`+newEmail()+`","role":"operator"}`, 403)
		}
		for i := 0; i < 10; i++ {
			e.invite(e.aRaw, ipOne, newEmail(), 201)
		}
	})
	t.Run("Q8 an unauthenticated request does not spend quota", func(t *testing.T) {
		e := newAPIEnv(t)
		second := e.newCompany()
		for i := 0; i < 25; i++ {
			e.doFrom("", ipOne, "POST", "/api/v1/users/invitations", `{"email":"`+newEmail()+`","role":"operator"}`, 401)
		}
		// 20 accepted invitations from the same address: every token of its quota is still there.
		for i := 0; i < 10; i++ {
			e.invite(e.aRaw, ipOne, newEmail(), 201)
			e.invite(second.aRaw, ipOne, newEmail(), 201)
		}
	})
}
