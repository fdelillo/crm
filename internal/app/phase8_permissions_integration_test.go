//go:build integration

package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

// T-B802: anonymous/operator/admin outcomes are literal, never inferred from chi/authz.
// This table must cover the same complete set as T-B801 and the OpenAPI document.
var isolationPermissions = map[string][3]int{
	"GET /api/v1/industry-templates":               {200, 200, 200},
	"POST /api/v1/auth/signup":                     {201, 201, 201},
	"POST /api/v1/auth/login":                      {200, 200, 200},
	"POST /api/v1/auth/logout":                     {204, 204, 204},
	"POST /api/v1/auth/password-reset":             {202, 202, 202},
	"POST /api/v1/auth/password-reset/confirm":     {204, 204, 204},
	"POST /api/v1/auth/email-verification/confirm": {204, 204, 204},
	"POST /api/v1/auth/email-verification/resend":  {401, 202, 202},
	"POST /api/v1/auth/invitations/preview":        {200, 200, 200},
	"POST /api/v1/auth/invitations/accept":         {201, 201, 201},
	"GET /api/v1/me":                               {401, 200, 200},
	"GET /api/v1/tenant":                           {401, 200, 200},
	"PATCH /api/v1/tenant":                         {401, 403, 200},
	"GET /api/v1/tenant/logo":                      {401, 200, 200},
	"PUT /api/v1/tenant/logo":                      {401, 403, 200},
	"DELETE /api/v1/tenant/logo":                   {401, 403, 204},
	"GET /api/v1/users":                            {401, 403, 200},
	"POST /api/v1/users/invitations":               {401, 403, 201},
	"PUT /api/v1/users/{userId}/role":              {401, 403, 200},
	"POST /api/v1/users/{userId}/deactivate":       {401, 403, 200},
	"POST /api/v1/users/{userId}/reactivate":       {401, 403, 200},
}

func permissionRequest(t *testing.T, f *e2e.Scenario, key string) (path, ct string, body []byte) {
	t.Helper()
	path = strings.SplitN(key, " ", 2)[1]
	ct = "application/json"
	switch isolationCases[key] {
	case "duplicate_signup":
		body = signupBody("signup-" + uuid.NewString() + "@example.test")
	case "login":
		body = jsonBody(map[string]string{"email": f.B.Users["operator"].Email, "password": e2e.Password})
	case "reset_request":
		body = jsonBody(map[string]string{"email": f.B.Users["operator"].Email})
	case "reset_confirm":
		body = jsonBody(map[string]string{"token": f.B.Reset, "password": e2e.NewPassword})
	case "verification_confirm":
		body = jsonBody(map[string]string{"token": f.B.Verification})
	case "invitation_preview":
		body = jsonBody(map[string]string{"token": f.B.Invitation})
	case "invitation_accept":
		body = jsonBody(map[string]string{"token": f.B.Invitation, "name": "Accepted B", "password": e2e.Password})
	case "patch":
		body = []byte(`{"name":"Updated A"}`)
	case "logo_put":
		body, ct = logoMultipart(t, f.A.Logo, "file")
	case "invite":
		body = jsonBody(map[string]string{"email": "new-" + uuid.NewString() + "@example.test", "role": "operator"})
	case "foreign_id":
		target := f.A.Users["operator"].ID
		if strings.HasSuffix(path, "/reactivate") {
			target = f.A.Users["disabled_password"].ID
		}
		path = strings.ReplaceAll(path, "{userId}", target.String())
		if strings.HasSuffix(path, "/role") {
			body = []byte(`{"role":"admin"}`)
		}
	}
	if len(body) == 0 {
		ct = ""
	}
	return
}

func TestIsolationPermissionMatrix(t *testing.T) {
	api := app.BuildAPIRouter(nil, nil, clock.Real{}, discardLogger())
	routeCoverage(t, api)
	for key := range isolationCases {
		if _, ok := isolationPermissions[key]; !ok {
			t.Errorf("missing permission case: %s", key)
		}
	}
	for key := range isolationPermissions {
		if _, ok := isolationCases[key]; !ok {
			t.Errorf("permission case without isolation route: %s", key)
		}
	}
	if t.Failed() {
		return
	}
	for key, wants := range isolationPermissions {
		for i, actor := range []string{"anonymous", "operator", "admin"} {
			t.Run(key+"/"+actor, func(t *testing.T) {
				f := e2e.New(t, pgtest.AppPool(t))
				api := app.BuildAPIRouter(f.Users, f.Companies, f.Clock, f.Logger)
				h := isolationRoot(t, f, api)
				path, ct, body := permissionRequest(t, f, key)
				method := strings.SplitN(key, " ", 2)[0]
				var cookie *http.Cookie
				if actor != "anonymous" {
					cookie = f.A.Users[actor].Cookie
				}
				before := f.Snapshot(t, f.A.ID)
				isolationRequest(t, h, context.Background(), cookie, method, path, ct, body, "", wants[i])
				if wants[i] == 403 || wants[i] == 401 {
					assertUnchanged(t, f, f.A.ID, before)
				}
				if wants[1] == 403 && actor == "operator" {
					// Permission precedes decoding, even when a service also checks it later.
					isolationRequest(t, h, context.Background(), cookie, method, path, "application/json", []byte(`{"broken":`), "", 403)
					if strings.Contains(key, "{userId}") {
						for _, id := range []uuid.UUID{f.B.Users["operator"].ID, uuid.New()} {
							foreign := strings.ReplaceAll(strings.SplitN(key, " ", 2)[1], "{userId}", id.String())
							isolationRequest(t, h, context.Background(), cookie, method, foreign, ct, body, "", 403)
						}
					}
					assertUnchanged(t, f, f.A.ID, before)
				}
				// Each protected route rejects invited/disabled users even with actual session cookies.
				if wants[0] == 401 {
					for _, state := range []string{"invited", "disabled_password", "disabled_no_password"} {
						isolationRequest(t, h, context.Background(), f.B.Users[state].Cookie, method, path, ct, body, "", 401)
					}
				}
			})
		}
	}
}
