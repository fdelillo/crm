//go:build integration

package app_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTenantHTTPPatchTextErrorCodes(t *testing.T) {
	h, admin, _, _ := phase7Handler(t)
	for _, tc := range []struct{ name, field, value, code string }{
		{"timezone_long", "timezone", strings.Repeat(" ", 62) + "UTC", "invalid_timezone"},
		{"timezone_NUL", "timezone", "UTC\x00", "invalid_timezone"},
		{"email_long", "email", strings.Repeat(" ", 244) + "a@example.com", "invalid_format"},
		{"email_NUL", "email", "a@example.com\x00", "invalid_format"},
		{"tax_id_long", "tax_id", " 30-12345678-1 ", "invalid_format"},
		{"tax_id_NUL", "tax_id", "30123456781\x00", "invalid_format"},
		{"name_long", "name", strings.Repeat("n", 121), "invalid_value"},
		{"name_long_blank", "name", strings.Repeat(" ", 121), "required"},
		{"legal_name_long", "legal_name", strings.Repeat("l", 201), "invalid_value"},
		{"address_long", "address", strings.Repeat("a", 301), "invalid_value"},
		{"phone_long", "phone", strings.Repeat("1", 51), "invalid_value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{tc.field: tc.value})
			if err != nil {
				t.Fatal(err)
			}
			rec := tenantRequest(t, h, admin, "PATCH", "/api/v1/tenant", "application/json", body, "", 422)
			assertTenantProblem(t, rec, tc.field, tc.code)
		})
	}
}
