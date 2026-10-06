package tenant

import "testing"

// T-B701. Official ARCA validator (consulted 2026-10-06):
// https://seti.afip.gob.ar/padron-puc-constancia-internet/js/ValidaCuit.js
// Its weighted sum INCLUDING the last digit must be divisible by 11.
// Thus 11 - sum%11 == 11 means digit 0; == 10 has no decimal digit
// (neither 0 nor 9 repairs it). Fixtures are synthetic, not registry lookups.
func TestNormalizeTaxID(t *testing.T) {
	for _, tc := range []struct{ in, want, code string }{
		{"30-12345678-1", "30123456781", ""}, {"30123456781", "30123456781", ""},
		{"30123456782", "", "invalid_tax_id"}, {"3012345678", "", "invalid_format"},
		{"30abcdefgh1", "", "invalid_format"}, {"30--123456781", "", "invalid_format"},
		{"30-00000009-0", "30000000090", ""},  // sum 33: 11 - remainder == 11
		{"30000000049", "", "invalid_tax_id"}, // sum 23: 11 - remainder == 10
		{"30000000040", "", "invalid_tax_id"},
		{"30000000030", "", "invalid_tax_id"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, code := normalizeTaxID(tc.in)
			if got != tc.want || code != tc.code {
				t.Fatalf("got %q/%q want %q/%q", got, code, tc.want, tc.code)
			}
		})
	}
}
func TestTenantNameAndTimezone(t *testing.T) {
	for _, tc := range []struct{ value, code string }{{"", "required"}, {"   ", "required"}, {"Empresa", ""}} {
		if got := validateName(tc.value); got != tc.code {
			t.Fatalf("name code=%s", got)
		}
	}
	for _, tc := range []struct{ value, code string }{{"America/Argentina/Cordoba", ""}, {"Marte/Olympus", "invalid_timezone"}, {"", "invalid_timezone"}} {
		if got := validateTimezone(tc.value); got != tc.code {
			t.Fatalf("timezone code=%s", got)
		}
	}
}
