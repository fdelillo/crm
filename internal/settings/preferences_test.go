package settings

import (
	"reflect"
	"strings"
	"testing"
)

func validLabels() Labels {
	return Labels{
		Project:  EntityLabel{"Obra", "Obras", genderFeminine},
		Customer: EntityLabel{"Cliente", "Clientes", genderMasculine},
		Quote:    EntityLabel{"Presupuesto", "Presupuestos", genderMasculine},
	}
}

// T-002-B301 (002/DD-10, P-3): names are trimmed; paths are entity.field and every error is reported.
func TestNormalizeLabels(t *testing.T) {
	t.Run("valid, trimmed", func(t *testing.T) {
		in := validLabels()
		in.Project.Singular = "  Obra "
		got, errs := normalizeLabels(in)
		if errs != nil || !reflect.DeepEqual(got, validLabels()) {
			t.Fatalf("got (%+v, %v)", got, errs)
		}
	})
	tests := []struct {
		name   string
		mutate func(*Labels)
		want   FieldErrors
	}{
		{"empty singular", func(l *Labels) { l.Project.Singular = "" }, FieldErrors{"project.singular": codeRequired}},
		{"blank plural", func(l *Labels) { l.Customer.Plural = "  " }, FieldErrors{"customer.plural": codeRequired}},
		{"41 characters", func(l *Labels) { l.Quote.Singular = strings.Repeat("ñ", 41) }, FieldErrors{"quote.singular": codeTooLong}},
		{"40 characters", func(l *Labels) { l.Quote.Plural = strings.Repeat("ñ", 40) }, nil},
		{"NUL", func(l *Labels) { l.Project.Plural = "O\x00" }, FieldErrors{"project.plural": codeInvalidValue}},
		{"invalid gender", func(l *Labels) { l.Project.Gender = "neutral" }, FieldErrors{"project.gender": codeInvalidValue}},
		{"missing gender", func(l *Labels) { l.Customer.Gender = "" }, FieldErrors{"customer.gender": codeRequired}},
		{"several entities", func(l *Labels) {
			l.Project.Singular = ""
			l.Quote.Gender = "x"
		}, FieldErrors{"project.singular": codeRequired, "quote.gender": codeInvalidValue}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := validLabels()
			tc.mutate(&in)
			if _, errs := normalizeLabels(in); !reflect.DeepEqual(errs, tc.want) {
				t.Fatalf("errs = %v, want %v", errs, tc.want)
			}
		})
	}
}

// T-002-B301 (002/DD-10, P-10): validity 1..365 and optional terms of at most 2000 characters.
func TestNormalizeQuoteDefaults(t *testing.T) {
	for _, days := range []int{1, 365} {
		if _, errs := normalizeQuoteDefaults(QuoteDefaults{ValidityDays: days}); errs != nil {
			t.Errorf("validity_days %d: %v", days, errs)
		}
	}
	for _, days := range []int{0, 366, -1} {
		_, errs := normalizeQuoteDefaults(QuoteDefaults{ValidityDays: days})
		if !reflect.DeepEqual(errs, FieldErrors{"validity_days": codeInvalidValue}) {
			t.Errorf("validity_days %d: %v", days, errs)
		}
	}
	for _, terms := range []string{"", "   "} {
		got, errs := normalizeQuoteDefaults(QuoteDefaults{ValidityDays: 15, Terms: ptr(terms)})
		if errs != nil || got.Terms != nil || got.ValidityDays != 15 {
			t.Errorf("terms %q: (%+v, %v), want nil terms", terms, got, errs)
		}
	}
	got, _ := normalizeQuoteDefaults(QuoteDefaults{ValidityDays: 15, Terms: ptr(" Pago contra entrega ")})
	if got.Terms == nil || *got.Terms != "Pago contra entrega" {
		t.Errorf("terms not trimmed: %v", got.Terms)
	}
	if _, errs := normalizeQuoteDefaults(QuoteDefaults{ValidityDays: 15, Terms: ptr(strings.Repeat("ñ", 2000))}); errs != nil {
		t.Errorf("2000 characters: %v", errs)
	}
	_, errs := normalizeQuoteDefaults(QuoteDefaults{ValidityDays: 15, Terms: ptr(strings.Repeat("ñ", 2001))})
	if !reflect.DeepEqual(errs, FieldErrors{"terms": codeTooLong}) {
		t.Errorf("2001 characters: %v", errs)
	}
	_, errs = normalizeQuoteDefaults(QuoteDefaults{ValidityDays: 0, Terms: ptr(strings.Repeat("a", 2001))})
	if !reflect.DeepEqual(errs, FieldErrors{"validity_days": codeInvalidValue, "terms": codeTooLong}) {
		t.Errorf("both: %v", errs)
	}
}
