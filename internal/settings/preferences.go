package settings

// Gender is the grammatical gender of a custom name: "masculine" or "feminine" (P-3).
type Gender string

const (
	genderMasculine Gender = "masculine"
	genderFeminine  Gender = "feminine"
)

var genders = []Gender{genderMasculine, genderFeminine}

// EntityLabel is the singular, plural and gender of a custom name.
type EntityLabel struct {
	Singular string
	Plural   string
	Gender   Gender
}

// Labels holds the three custom names of the company.
type Labels struct{ Project, Customer, Quote EntityLabel }

// QuoteDefaults are the default commercial conditions of a quote.
type QuoteDefaults struct {
	ValidityDays int     // 1..365
	Terms        *string // nil = no terms; free text
}

func validateGender(g Gender) string { return validateEnum(g, genders) }

// normalizeLabels validates and normalizes the three names, returning every field error at once. The
// keys are the contract paths ("project.singular").
// T-002-B302 (002/DD-10, P-3): texts up to 40 characters, gender masculine or feminine.
func normalizeLabels(l Labels) (Labels, FieldErrors) {
	errs := FieldErrors{}
	normalize := func(prefix string, in EntityLabel) EntityLabel {
		out := EntityLabel{Gender: in.Gender}
		var code string
		out.Singular, code = normalizeRequiredText(in.Singular, maxEntityLabelLength)
		errs.add(prefix+".singular", code)
		out.Plural, code = normalizeRequiredText(in.Plural, maxEntityLabelLength)
		errs.add(prefix+".plural", code)
		if in.Gender == "" {
			errs.add(prefix+".gender", codeRequired)
		} else {
			errs.add(prefix+".gender", validateGender(in.Gender))
		}
		return out
	}
	out := Labels{
		Project:  normalize("project", l.Project),
		Customer: normalize("customer", l.Customer),
		Quote:    normalize("quote", l.Quote),
	}
	if len(errs) > 0 {
		return Labels{}, errs
	}
	return out, nil
}

// normalizeQuoteDefaults validates validity_days (1..365) and terms (optional, up to 2000 characters).
// T-002-B302 (002/DD-10, P-10): empty or only spaces terms become nil.
func normalizeQuoteDefaults(q QuoteDefaults) (QuoteDefaults, FieldErrors) {
	errs := FieldErrors{}
	if q.ValidityDays < minQuoteValidityDays || q.ValidityDays > maxQuoteValidityDays {
		errs.add("validity_days", codeInvalidValue)
	}
	out := QuoteDefaults{ValidityDays: q.ValidityDays}
	if q.Terms != nil {
		terms, code := normalizeOptionalText(*q.Terms, maxQuoteTermsLength)
		errs.add("terms", code)
		out.Terms = terms
	}
	if len(errs) > 0 {
		return QuoteDefaults{}, errs
	}
	return out, nil
}
