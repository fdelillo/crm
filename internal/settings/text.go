package settings

import (
	"strings"
	"unicode/utf8"
)

// Field codes of the contract (FieldErrors values). Never carry the submitted value.
const (
	codeRequired     = "required"
	codeTooLong      = "too_long"
	codeInvalidValue = "invalid_value"
)

// Text limits in characters (runes), the same as the CHECK constraints of data-model.md.
const (
	maxNameLength         = 60   // stage and category names
	maxItemNameLength     = 120  // catalog item name
	maxDescriptionLength  = 1000 // catalog item description
	maxUnitLabelLength    = 20
	maxEntityLabelLength  = 40 // singular and plural of a custom name
	maxQuoteTermsLength   = 2000
	minQuoteValidityDays  = 1
	maxQuoteValidityDays  = 365
	maxStageNameLength    = maxNameLength
	maxCategoryNameLength = maxNameLength
)

// normalizeRequiredText trims the value and returns it with an empty code, or an empty text and the
// contract code that rejects it. The maximum is measured in runes over the received value, before
// trimming, like DD-43 (2) of 001: trailing spaces count.
// T-002-B302 (DD-10): required, then invalid_value (NUL or malformed UTF-8), then too_long.
func normalizeRequiredText(value string, max int) (string, string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", codeRequired
	}
	if !cleanText(value) {
		return "", codeInvalidValue
	}
	if utf8.RuneCountInString(value) > max {
		return "", codeTooLong
	}
	return trimmed, ""
}

// normalizeOptionalText is normalizeRequiredText for an optional field: empty or only spaces is no
// value (nil) and not an error.
// T-002-B302 (DD-9, DD-10): same code order and length rule as the required text.
func normalizeOptionalText(value string, max int) (*string, string) {
	if strings.TrimSpace(value) == "" {
		return nil, ""
	}
	text, code := normalizeRequiredText(value, max)
	if code != "" {
		return nil, code
	}
	return &text, ""
}

func cleanText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

// validateEnum returns invalid_value when the value is not one of the allowed ones, empty included.
func validateEnum[T ~string](value T, allowed []T) string {
	for _, candidate := range allowed {
		if value == candidate {
			return ""
		}
	}
	return codeInvalidValue
}

// add records a code for a field; an empty code is no error.
func (e FieldErrors) add(field, code string) {
	if code != "" {
		e[field] = code
	}
}

func (e FieldErrors) merge(other FieldErrors) {
	for field, code := range other {
		e[field] = code
	}
}

// orNil returns nil for an empty set, so callers compare against "no errors".
func (e FieldErrors) orNil() FieldErrors {
	if len(e) == 0 {
		return nil
	}
	return e
}
