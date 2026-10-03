package audit

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateDataRejectsEmbeddedCredentials(t *testing.T) {
	raw := strings.Repeat("A", 43)
	cases := []struct {
		name, value string
	}{
		{"token assignment", "token=valor-secreto-de-prueba-1"},
		{"spaced reset token", "x reset_token = valor-secreto-de-prueba-2"},
		{"authorization", "Authorization: Bearer valor-secreto-de-prueba-3"},
		{"cookie", "Cookie: __Host-crm_session=valor-secreto-de-prueba-4"},
		{"API key", "X-API-Key:valor-secreto-de-prueba-5"},
		{"pin", "pin: 1234"},
		{"link", "see https://crm.example/reset-password#token=" + raw},
		{"raw in prose", "reintentar con " + raw + " más tarde"},
		{"raw in parentheses", "(" + raw + ")"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateData("", map[string]any{"note": tc.value})
			if !errors.Is(err, ErrSecretInData) {
				t.Fatalf("error = %v", err)
			}
			if !strings.Contains(err.Error(), "data.note") || strings.Contains(err.Error(), raw) || strings.Contains(err.Error(), "valor-secreto-de-prueba") || strings.Contains(err.Error(), "token=") {
				t.Fatalf("error leaked value or omitted path: %v", err)
			}
		})
	}
	err := validateData("", map[string]any{"fields": []string{"name", "ver " + raw}})
	if !errors.Is(err, ErrSecretInData) || !strings.Contains(err.Error(), "fields[1]") {
		t.Fatalf("nested list error = %v", err)
	}
}

func TestValidateDataAcceptsNonCredentialText(t *testing.T) {
	cases := []string{
		"password_reset_request", "Motivo: el cliente pidió anular el cobro de las 10:30",
		"contraseña: cambiada", "https://crm.example/settings?tab=users", "sessions_revoked=2",
		"12345678-1234-1234-1234-123456789abc", strings.Repeat("1", 22),
		strings.Repeat("a", 64), strings.Repeat("A", 42), strings.Repeat("A", 44),
	}
	for _, value := range cases {
		if err := validateData("", map[string]any{"note": value}); err != nil {
			t.Errorf("unexpected rejection of %q: %v", value, err)
		}
	}
}
