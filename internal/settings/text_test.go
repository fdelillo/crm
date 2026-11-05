package settings

import (
	"strings"
	"testing"
)

// T-002-B301 (DD-10, DD-43 (2) of 001): required text is trimmed and its limit is measured in runes
// over the received value.
func TestNormalizeRequiredText(t *testing.T) {
	const max = 10
	tests := []struct {
		name  string
		input string
		want  string
		code  string
	}{
		{"trimmed", "  Obra  ", "Obra", ""},
		{"empty", "", "", codeRequired},
		{"only spaces", "   ", "", codeRequired},
		{"one rune over, counting runes", strings.Repeat("ñ", max+1), "", codeTooLong},
		{"limit in runes is not bytes", strings.Repeat("ñ", max), strings.Repeat("ñ", max), ""},
		{"trailing spaces count", "Obra" + strings.Repeat(" ", max), "", codeTooLong},
		{"NUL", "Ob\x00ra", "", codeInvalidValue},
		{"malformed UTF-8", "Ob\xffra", "", codeInvalidValue},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, code := normalizeRequiredText(tc.input, max)
			if got != tc.want || code != tc.code {
				t.Fatalf("normalizeRequiredText(%q, %d) = (%q, %q), want (%q, %q)", tc.input, max, got, code, tc.want, tc.code)
			}
		})
	}
}

// T-002-B301 (DD-9, DD-10): optional text with no content is "no value", not an error.
func TestNormalizeOptionalText(t *testing.T) {
	for _, input := range []string{"", "   ", "\t\n"} {
		got, code := normalizeOptionalText(input, 10)
		if got != nil || code != "" {
			t.Errorf("normalizeOptionalText(%q) = (%v, %q), want (nil, \"\")", input, got, code)
		}
	}
	got, code := normalizeOptionalText("  Obra ", 10)
	if code != "" || got == nil || *got != "Obra" {
		t.Errorf("normalizeOptionalText(\"  Obra \") = (%v, %q), want (\"Obra\", \"\")", got, code)
	}
	for _, tc := range []struct{ input, code string }{
		{"Obra\x00", codeInvalidValue},
		{strings.Repeat("ñ", 11), codeTooLong},
		{"Obra" + strings.Repeat(" ", 10), codeTooLong},
	} {
		got, code := normalizeOptionalText(tc.input, 10)
		if got != nil || code != tc.code {
			t.Errorf("normalizeOptionalText(%q) = (%v, %q), want (nil, %q)", tc.input, got, code, tc.code)
		}
	}
}

// T-002-B301 (DD-7, DD-8, DD-9, DD-10, P-3): every enumerated type accepts its values and rejects
// the rest, including a different case and the empty string.
func TestEnumeratedValues(t *testing.T) {
	invalid := []string{"Open", "pink", "kg", "neutral", ""}
	check := func(name string, validate func(string) string, valid []string) {
		t.Run(name, func(t *testing.T) {
			for _, v := range valid {
				if code := validate(v); code != "" {
					t.Errorf("%q: code %q, want ok", v, code)
				}
			}
			for _, v := range invalid {
				if code := validate(v); code != codeInvalidValue {
					t.Errorf("%q: code %q, want %q", v, code, codeInvalidValue)
				}
			}
		})
	}
	check("StageType", func(v string) string { return validateStageType(StageType(v)) }, []string{"open", "won", "lost"})
	check("Color", func(v string) string { return validateColor(Color(v)) },
		[]string{"gray", "blue", "teal", "green", "yellow", "orange", "red", "purple"})
	check("CategoryType", func(v string) string { return validateCategoryType(CategoryType(v)) }, []string{"income", "expense"})
	check("Unit", func(v string) string { return validateUnit(Unit(v)) },
		[]string{"unit", "square_meter", "linear_meter", "hour", "other"})
	check("Gender", func(v string) string { return validateGender(Gender(v)) }, []string{"masculine", "feminine"})
	check("SystemKey", func(v string) string { return validateSystemKey(SystemKey(v)) },
		[]string{"customer_receipts", "partner_contributions", "supplier_payments", "employee_payments", "partner_withdrawals"})
}

// T-002-B301 (DD-8, FR-001): stage and category names are required text of at most 60 characters.
func TestNormalizeStageAndCategoryNames(t *testing.T) {
	for name, normalize := range map[string]func(string) (string, string){
		"stage": normalizeStageName, "category": normalizeCategoryName,
	} {
		t.Run(name, func(t *testing.T) {
			if got, code := normalize("  Cobrado "); got != "Cobrado" || code != "" {
				t.Errorf("trim: (%q, %q)", got, code)
			}
			if _, code := normalize("  "); code != codeRequired {
				t.Errorf("blank: code %q", code)
			}
			if _, code := normalize(strings.Repeat("ñ", 60)); code != "" {
				t.Errorf("60 runes: code %q", code)
			}
			if _, code := normalize(strings.Repeat("ñ", 61)); code != codeTooLong {
				t.Errorf("61 runes: code %q", code)
			}
		})
	}
}
