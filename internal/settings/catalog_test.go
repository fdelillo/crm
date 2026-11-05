package settings

import (
	"reflect"
	"strings"
	"testing"
)

// T-002-B301 (002/INV-7, DD-9, ADR-029): amount in [0, 2^53-1] and currency ARS or USD.
func TestValidateReferencePrice(t *testing.T) {
	tests := []struct {
		name  string
		price Money
		want  FieldErrors
	}{
		{"zero is a valid price", Money{0, "ARS"}, nil},
		{"the maximum", Money{9007199254740991, "USD"}, nil},
		{"over the maximum", Money{9007199254740992, "ARS"}, FieldErrors{"reference_price.amount_cents": codeInvalidValue}},
		{"negative", Money{-1, "ARS"}, FieldErrors{"reference_price.amount_cents": codeInvalidValue}},
		{"lowercase currency", Money{100, "ars"}, FieldErrors{"reference_price.currency": codeInvalidValue}},
		{"other currency", Money{100, "EUR"}, FieldErrors{"reference_price.currency": codeInvalidValue}},
		{"empty currency", Money{100, ""}, FieldErrors{"reference_price.currency": codeRequired}},
		{"both wrong", Money{-1, "EUR"}, FieldErrors{
			"reference_price.amount_cents": codeInvalidValue, "reference_price.currency": codeInvalidValue,
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateReferencePrice(tc.price); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	if MaxAmountCents != 9007199254740991 {
		t.Fatalf("MaxAmountCents = %d", MaxAmountCents)
	}
}

func ptr[T any](v T) *T { return &v }

// T-002-B301 (002/DD-9, P-9): other requires a label of at most 20 characters; the rest discard it.
func TestNormalizeUnitLabel(t *testing.T) {
	label := func(l *string, err FieldErrors, want *string, wantErr FieldErrors, t *testing.T) {
		t.Helper()
		if !reflect.DeepEqual(l, want) || !reflect.DeepEqual(err, wantErr) {
			t.Fatalf("got (%v, %v), want (%v, %v)", deref(l), err, deref(want), wantErr)
		}
	}
	t.Run("other with label", func(t *testing.T) {
		l, err := normalizeUnitLabel(unitOther, ptr(" jornada "))
		label(l, err, ptr("jornada"), nil, t)
	})
	t.Run("other without label", func(t *testing.T) {
		l, err := normalizeUnitLabel(unitOther, nil)
		label(l, err, nil, FieldErrors{"unit_label": codeRequired}, t)
	})
	t.Run("other with blank label", func(t *testing.T) {
		l, err := normalizeUnitLabel(unitOther, ptr("   "))
		label(l, err, nil, FieldErrors{"unit_label": codeRequired}, t)
	})
	t.Run("hour discards the label", func(t *testing.T) {
		l, err := normalizeUnitLabel(unitHour, ptr("x"))
		label(l, err, nil, nil, t)
	})
	t.Run("other with 20 and 21 characters", func(t *testing.T) {
		l, err := normalizeUnitLabel(unitOther, ptr(strings.Repeat("ñ", 20)))
		label(l, err, ptr(strings.Repeat("ñ", 20)), nil, t)
		l, err = normalizeUnitLabel(unitOther, ptr(strings.Repeat("ñ", 21)))
		label(l, err, nil, FieldErrors{"unit_label": codeTooLong}, t)
	})
	t.Run("other with NUL", func(t *testing.T) {
		l, err := normalizeUnitLabel(unitOther, ptr("jor\x00nada"))
		label(l, err, nil, FieldErrors{"unit_label": codeInvalidValue}, t)
	})
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// T-002-B301 (002/DD-9, FR-005): creation fields, codes and paths of createCatalogItem.
func TestNormalizeNewCatalogItem(t *testing.T) {
	t.Run("valid, normalized", func(t *testing.T) {
		got, errs := normalizeNewCatalogItem(NewCatalogItem{
			Name: " Ventana ", Description: ptr("   "), Unit: unitHour, UnitLabel: ptr("x"),
			ReferencePrice: &Money{0, "ARS"},
		})
		want := NewCatalogItem{Name: "Ventana", Unit: unitHour, ReferencePrice: &Money{0, "ARS"}}
		if errs != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got (%+v, %v), want (%+v, nil)", got, errs, want)
		}
	})
	t.Run("without price", func(t *testing.T) {
		if _, errs := normalizeNewCatalogItem(NewCatalogItem{Name: "Ventana", Unit: unitUnit}); errs != nil {
			t.Fatalf("errs = %v", errs)
		}
	})
	t.Run("every error at once", func(t *testing.T) {
		_, errs := normalizeNewCatalogItem(NewCatalogItem{
			Name: " ", Description: ptr(strings.Repeat("a", 1001)), Unit: "kg",
			ReferencePrice: &Money{-1, "ars"},
		})
		want := FieldErrors{
			"name": codeRequired, "description": codeTooLong, "unit": codeInvalidValue,
			"reference_price.amount_cents": codeInvalidValue, "reference_price.currency": codeInvalidValue,
		}
		if !reflect.DeepEqual(errs, want) {
			t.Fatalf("errs = %v, want %v", errs, want)
		}
	})
	t.Run("missing unit is required", func(t *testing.T) {
		_, errs := normalizeNewCatalogItem(NewCatalogItem{Name: "Ventana"})
		if !reflect.DeepEqual(errs, FieldErrors{"unit": codeRequired}) {
			t.Fatalf("errs = %v", errs)
		}
	})
	t.Run("name limit is 120 runes", func(t *testing.T) {
		if _, errs := normalizeNewCatalogItem(NewCatalogItem{Name: strings.Repeat("ñ", 120), Unit: unitUnit}); errs != nil {
			t.Fatalf("120: %v", errs)
		}
		_, errs := normalizeNewCatalogItem(NewCatalogItem{Name: strings.Repeat("ñ", 121), Unit: unitUnit})
		if !reflect.DeepEqual(errs, FieldErrors{"name": codeTooLong}) {
			t.Fatalf("121: %v", errs)
		}
	})
	t.Run("other without label", func(t *testing.T) {
		_, errs := normalizeNewCatalogItem(NewCatalogItem{Name: "Ventana", Unit: unitOther})
		if !reflect.DeepEqual(errs, FieldErrors{"unit_label": codeRequired}) {
			t.Fatalf("errs = %v", errs)
		}
	})
	t.Run("NUL in name and description", func(t *testing.T) {
		_, errs := normalizeNewCatalogItem(NewCatalogItem{Name: "a\x00", Description: ptr("b\x00"), Unit: unitUnit})
		want := FieldErrors{"name": codeInvalidValue, "description": codeInvalidValue}
		if !reflect.DeepEqual(errs, want) {
			t.Fatalf("errs = %v", errs)
		}
	})
}
