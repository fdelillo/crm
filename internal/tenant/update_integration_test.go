//go:build integration

package tenant_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/google/uuid"
)

func companyAdmin(t *testing.T, svc *tenant.Service) authz.Principal {
	t.Helper()
	reg, err := svc.Register(context.Background(), signupWithEmail(uuid.NewString()+"@example.com"), identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	return reg.Session.Principal
}
func patch(t *testing.T, data string) tenant.Update {
	t.Helper()
	var p tenant.Update
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestUpdatePartialNullableAndAudit(t *testing.T) {
	svc, _, runner := newRegistrationServices(t)
	p := companyAdmin(t, svc)
	before, err := svc.Get(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.Update(context.Background(), p, patch(t, `{"legal_name":"Free text secret","tax_id":"30-12345678-1","address":"Calle","phone":"123","email":"contact@example.com"}`), identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if updated.TaxID == nil || *updated.TaxID != "30123456781" || updated.LegalName == nil || updated.Name != before.Name || updated.BaseCurrency != before.BaseCurrency || !updated.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("update=%+v", updated)
	}
	after, err := svc.Update(context.Background(), p, patch(t, `{"legal_name":null,"timezone":"America/Argentina/Cordoba"}`), identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if after.LegalName != nil || after.TaxID == nil || *after.TaxID != *updated.TaxID || after.Address == nil || *after.Address != "Calle" || after.Timezone != "America/Argentina/Cordoba" || !after.UpdatedAt.After(updated.UpdatedAt) {
		t.Fatalf("partial/null=%+v", after)
	}
	var data []byte
	var auditSameTx bool
	err = runner.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT a.data,a.xmin=t.xmin FROM app.audit_log a JOIN app.tenants t ON t.id=a.tenant_id WHERE a.tenant_id=$1 AND a.action='tenant.updated' ORDER BY a.occurred_at DESC LIMIT 1`, p.TenantID).Scan(&data, &auditSameTx)
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"fields": []any{"legal_name", "timezone"}}
	if !reflect.DeepEqual(got, want) || !auditSameTx {
		t.Fatalf("audit=%s same transaction=%v", data, auditSameTx)
	}
	persisted, err := svc.Get(context.Background(), p)
	if err != nil || !reflect.DeepEqual(persisted, after) {
		t.Fatalf("persisted=%+v err=%v", persisted, err)
	}
}
func TestUpdateValidationAndUnknownFields(t *testing.T) {
	svc, _, _ := newRegistrationServices(t)
	p := companyAdmin(t, svc)
	for _, body := range []string{`{"base_currency":"USD"}`, `{}`, `null`, `{"name":3}`} {
		var input tenant.Update
		if err := json.Unmarshal([]byte(body), &input); err == nil {
			t.Fatalf("accepted malformed update %s", body)
		}
	}
	for _, tc := range []struct{ body, field, code string }{
		{`{"timezone":"Marte/Olympus"}`, "timezone", "invalid_timezone"},
		{`{"name":""}`, "name", "required"}, {`{"name":null}`, "name", "required"},
		{`{"tax_id":"30123456782"}`, "tax_id", "invalid_tax_id"},
		{`{"email":"not-an-email"}`, "email", "invalid_format"},
	} {
		before, _ := svc.Get(context.Background(), p)
		_, err := svc.Update(context.Background(), p, patch(t, tc.body), identity.RequestMeta{})
		var fields tenant.FieldErrors
		if !errors.As(err, &fields) || fields[tc.field] != tc.code {
			t.Fatalf("body=%s err=%v", tc.body, err)
		}
		after, _ := svc.Get(context.Background(), p)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("invalid update changed row")
		}
	}
}
