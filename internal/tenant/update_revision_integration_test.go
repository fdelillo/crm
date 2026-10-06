//go:build integration

package tenant_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
)

func updateAuditState(t *testing.T, runner db.TxRunner, p authz.Principal) (count int, xid string) {
	t.Helper()
	err := runner.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND action='tenant.updated'), xmin::text FROM app.tenants WHERE id=$1`, p.TenantID).Scan(&count, &xid)
	})
	if err != nil {
		t.Fatal(err)
	}
	return
}
func TestUpdateEmptyOptionalsBecomeNull(t *testing.T) {
	for _, field := range []string{"legal_name", "tax_id", "address", "phone", "email"} {
		for _, value := range []string{"", "   "} {
			t.Run(field+"/"+value, func(t *testing.T) {
				svc, _, runner := newRegistrationServices(t)
				p := companyAdmin(t, svc)
				initial := "text"
				if field == "tax_id" {
					initial = "30123456781"
				}
				if field == "email" {
					initial = "company@example.com"
				}
				body, _ := json.Marshal(map[string]string{field: initial})
				if _, err := svc.Update(context.Background(), p, patch(t, string(body)), identity.RequestMeta{}); err != nil {
					t.Fatal(err)
				}
				body, _ = json.Marshal(map[string]string{field: value})
				out, err := svc.Update(context.Background(), p, patch(t, string(body)), identity.RequestMeta{})
				if err != nil {
					t.Fatal(err)
				}
				data, _ := json.Marshal(out)
				var fields map[string]any
				_ = json.Unmarshal(data, &fields)
				if fields[field] != nil {
					t.Fatalf("DD-43: %s was not NULL: %v", field, fields[field])
				}
				if n, _ := updateAuditState(t, runner, p); n != 2 {
					t.Fatalf("audits=%d", n)
				}
			})
		}
	}
}
func TestUpdateNoChangeAndChangedFields(t *testing.T) {
	svc, _, runner := newRegistrationServices(t)
	p := companyAdmin(t, svc)
	before, err := svc.Update(context.Background(), p, patch(t, `{"tax_id":"30123456781","phone":"123"}`), identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	n, xid := updateAuditState(t, runner, p)
	after, err := svc.Update(context.Background(), p, patch(t, `{"tax_id":"30-12345678-1","phone":" 123 "}`), identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	n2, xid2 := updateAuditState(t, runner, p)
	if !reflect.DeepEqual(before, after) || n != n2 || xid != xid2 {
		t.Fatalf("DD-43 no-op wrote: audits %d -> %d xmin %s -> %s updated_at %v -> %v", n, n2, xid, xid2, before.UpdatedAt, after.UpdatedAt)
	}
	_, err = svc.Update(context.Background(), p, patch(t, `{"tax_id":"30-12345678-1","phone":"456"}`), identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	var fields []string
	if err = runner.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		var data []byte
		if err := tx.QueryRow(ctx, `SELECT data->'fields' FROM app.audit_log WHERE tenant_id=$1 AND action='tenant.updated' ORDER BY occurred_at DESC LIMIT 1`, p.TenantID).Scan(&data); err != nil {
			return err
		}
		return json.Unmarshal(data, &fields)
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fields, []string{"phone"}) {
		t.Fatalf("only changed fields expected, got %v", fields)
	}
}
func TestUpdateChangedFieldsFollowModelOrder(t *testing.T) {
	svc, _, r := newRegistrationServices(t)
	p := companyAdmin(t, svc)
	_, err := svc.Update(context.Background(), p, patch(t, `{"name":"New","legal_name":"Legal","tax_id":"30123456781","address":"Road","phone":"123","email":"contact@example.com","timezone":"Asia/Kathmandu"}`), identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	var fields []string
	err = r.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		var data []byte
		if err := tx.QueryRow(ctx, `SELECT data->'fields' FROM app.audit_log WHERE tenant_id=$1 AND action='tenant.updated'`, p.TenantID).Scan(&data); err != nil {
			return err
		}
		return json.Unmarshal(data, &fields)
	})
	if err != nil || !reflect.DeepEqual(fields, []string{"name", "legal_name", "tax_id", "address", "phone", "email", "timezone"}) {
		t.Fatalf("DD-43 fields order=%v err=%v", fields, err)
	}
}
func TestConcurrentUpdatesKeepBothFields(t *testing.T) {
	svc, _, r := newRegistrationServices(t)
	p := companyAdmin(t, svc)
	phone := patch(t, `{"phone":"123"}`)
	address := patch(t, `{"address":"Road"}`)
	runTenantQueue(t, r, p, false, func(ctx context.Context) error {
		_, err := svc.Update(ctx, p, phone, identity.RequestMeta{})
		return err
	}, func(ctx context.Context) error {
		_, err := svc.Update(ctx, p, address, identity.RequestMeta{})
		return err
	})
	out, err := svc.Get(context.Background(), p)
	if err != nil || out.Phone == nil || *out.Phone != "123" || out.Address == nil || *out.Address != "Road" {
		t.Fatalf("INV-34 lost fields: %+v err=%v", out, err)
	}
	var auditFields []string
	err = r.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT data->>'fields' FROM app.audit_log WHERE tenant_id=$1 AND action='tenant.updated' ORDER BY data->>'fields'`, p.TenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var fields string
			if err := rows.Scan(&fields); err != nil {
				return err
			}
			auditFields = append(auditFields, fields)
		}
		return rows.Err()
	})
	if err != nil || !reflect.DeepEqual(auditFields, []string{`["address"]`, `["phone"]`}) {
		t.Fatalf("concurrent audit=%v err=%v", auditFields, err)
	}
}

func TestUpdateAuditIncludesOnlyActualChanges(t *testing.T) {
	svc, _, runner := newRegistrationServices(t)
	p := companyAdmin(t, svc)
	if _, err := svc.Update(context.Background(), p, patch(t, `{"tax_id":"30123456781"}`), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(context.Background(), p, patch(t, `{"tax_id":"30-12345678-1","phone":"456"}`), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	var fields []string
	err := runner.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		var data []byte
		if err := tx.QueryRow(ctx, `SELECT data->'fields' FROM app.audit_log WHERE tenant_id=$1 AND action='tenant.updated' ORDER BY occurred_at DESC LIMIT 1`, p.TenantID).Scan(&data); err != nil {
			return err
		}
		return json.Unmarshal(data, &fields)
	})
	if err != nil || !reflect.DeepEqual(fields, []string{"phone"}) {
		t.Fatalf("DD-43 unchanged CUIT must not be audited: fields=%v err=%v", fields, err)
	}
}
