//go:build integration

package tenant_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/tenant"
)

// Preserve the per-field codes from c4650c9 while validating the received text
// before DD-43 trims it or turns an empty optional into NULL.
func TestUpdateTextErrorCodes(t *testing.T) {
	svc, _, runner := newRegistrationServices(t)
	p := companyAdmin(t, svc)
	before, err := svc.Get(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	count, xid := updateAuditState(t, runner, p)
	for _, tc := range []struct{ name, field, value, code string }{
		{"timezone_long", "timezone", strings.Repeat(" ", 62) + "UTC", "invalid_timezone"},
		{"timezone_NUL", "timezone", "UTC\x00", "invalid_timezone"},
		{"timezone_UTF8", "timezone", "UTC\xff", "invalid_timezone"},
		{"email_long", "email", strings.Repeat(" ", 244) + "a@example.com", "invalid_format"},
		{"email_NUL", "email", "a@example.com\x00", "invalid_format"},
		{"email_UTF8", "email", "a@example.com\xff", "invalid_format"},
		{"tax_id_long", "tax_id", " 30-12345678-1 ", "invalid_format"},
		{"tax_id_NUL", "tax_id", "30123456781\x00", "invalid_format"},
		{"tax_id_UTF8", "tax_id", "30123456781\xff", "invalid_format"},
		{"name_long", "name", strings.Repeat("n", 121), "invalid_value"},
		{"name_long_blank", "name", strings.Repeat(" ", 121), "required"},
		{"name_NUL", "name", "name\x00", "invalid_value"},
		{"name_UTF8", "name", "name\xff", "invalid_value"},
		{"legal_name_long", "legal_name", strings.Repeat("l", 201), "invalid_value"},
		{"address_long", "address", strings.Repeat("a", 301), "invalid_value"},
		{"phone_long", "phone", strings.Repeat("1", 51), "invalid_value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Update(context.Background(), p, tenant.Update{tc.field: &tc.value}, identity.RequestMeta{})
			var fields tenant.FieldErrors
			if !errors.As(err, &fields) || !reflect.DeepEqual(fields, tenant.FieldErrors{tc.field: tc.code}) {
				t.Fatalf("field errors=%#v err=%v; want %s: %s", fields, err, tc.field, tc.code)
			}
			after, err := svc.Get(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			nextCount, nextXID := updateAuditState(t, runner, p)
			if !reflect.DeepEqual(before, after) || nextCount != count || nextXID != xid {
				t.Fatal("invalid PATCH changed tenant or audit")
			}
		})
	}
}
