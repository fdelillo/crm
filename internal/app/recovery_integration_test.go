//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
)

func postRecovery(t *testing.T, client *http.Client, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestRecoveryHTTPContractAndUniformResponses(t *testing.T) {
	server := phase3Server(t)
	runner := db.NewTxRunner(pgtest.AppPool(t))
	var baseline http.Header
	for _, status := range []string{"active", "invited", "disabled", "missing"} {
		email := "not-found@example.com"
		if status != "missing" {
			company := fixture.NewCompany(t, pgtest.AppPool(t))
			err := runner.InTenantTx(context.Background(), company.ID, func(ctx context.Context, tx db.Tx) error {
				if status != "active" {
					if _, err := tx.Exec(ctx, `UPDATE app.users SET status=$1 WHERE tenant_id=$2 AND id=$3`, status, company.ID, company.UserID); err != nil {
						return err
					}
				}
				return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, company.ID, company.UserID).Scan(&email)
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		resp := postRecovery(t, server.Client, server.URL+"/api/v1/auth/password-reset", `{"email":"`+email+`"}`)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusAccepted || len(body) != 0 {
			t.Fatalf("%s: %d %q", status, resp.StatusCode, body)
		}
		relevant := http.Header{"Cache-Control": resp.Header.Values("Cache-Control"), "Content-Type": resp.Header.Values("Content-Type")}
		if baseline == nil {
			baseline = relevant
		} else if baseline.Get("Cache-Control") != relevant.Get("Cache-Control") || baseline.Get("Content-Type") != relevant.Get("Content-Type") {
			t.Fatalf("%s: response headers differ: %v vs %v", status, relevant, baseline)
		}
	}
	for _, tc := range []struct {
		path, body string
		want       int
	}{
		{"/api/v1/auth/password-reset/confirm", `{"token":"invalid","password":"valid-password"}`, 400},
		{"/api/v1/auth/email-verification/confirm", `{"token":"invalid"}`, 400},
		{"/api/v1/auth/email-verification/resend", ``, 401},
	} {
		resp := postRecovery(t, server.Client, server.URL+tc.path, tc.body)
		var problem map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&problem); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatalf("%s: %d: %v", tc.path, resp.StatusCode, problem)
		}
	}
}
