package identity

import (
	"net/mail"
	"strings"
)

// ValidEmail checks a normalized address against the API's Email constraints.
// Callers retain their existing trim/lowercase policy before validation.
func ValidEmail(email string) bool {
	if email == "" || len(email) > 254 || strings.ContainsRune(email, '\x00') {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}
