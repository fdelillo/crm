package settings

import "testing"

// T-002-B301 (002/DD-8, FR-004, data-model 4.1): the five system categories, with type and name.
func TestSystemCategories(t *testing.T) {
	want := map[SystemKey]struct {
		typ  CategoryType
		name string
	}{
		"customer_receipts":     {"income", "Cobros de clientes"},
		"partner_contributions": {"income", "Aportes de socios"},
		"supplier_payments":     {"expense", "Pagos a proveedores"},
		"employee_payments":     {"expense", "Pagos a empleados"},
		"partner_withdrawals":   {"expense", "Retiros de socios"},
	}
	if len(systemCategories) != len(want) {
		t.Fatalf("%d system categories, want %d", len(systemCategories), len(want))
	}
	seen := map[SystemKey]bool{}
	for _, c := range systemCategories {
		w, ok := want[c.Key]
		if !ok || seen[c.Key] {
			t.Fatalf("unexpected or repeated key %q", c.Key)
		}
		seen[c.Key] = true
		if c.Type != w.typ || c.Name != w.name {
			t.Errorf("%s = (%s, %q), want (%s, %q)", c.Key, c.Type, c.Name, w.typ, w.name)
		}
	}
}
