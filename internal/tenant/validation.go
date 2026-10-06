package tenant

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var taxIDPattern = regexp.MustCompile(`^(?:[0-9]{11}|[0-9]{2}-[0-9]{8}-[0-9])$`)

func normalizeTaxID(value string) (string, string) {
	if !taxIDPattern.MatchString(value) {
		return "", "invalid_format"
	}
	value = strings.ReplaceAll(value, "-", "")
	weights := [...]int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2, 1}
	sum := 0
	for i, weight := range weights {
		sum += int(value[i]-'0') * weight
	}
	if sum%11 != 0 {
		return "", "invalid_tax_id"
	}
	return value, ""
}
func validateName(value string) string {
	if strings.TrimSpace(value) == "" {
		return "required"
	}
	if !validText(value, 120) {
		return "invalid_value"
	}
	return ""
}
func validateTimezone(value string) string {
	if value == "" || !validText(value, 64) {
		return "invalid_timezone"
	}
	if _, err := time.LoadLocation(value); err != nil {
		return "invalid_timezone"
	}
	return ""
}
func validText(value string, max int) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00') && utf8.RuneCountInString(value) <= max
}
