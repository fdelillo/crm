package config_test

import (
	"encoding/base64"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/config"
)

const (
	secretDBPassword = "s3cr3t-db-password"
	secretDatabase   = "postgres://crm_app:" + secretDBPassword + "@db.internal:5432/crm"
	secretMigration  = "postgres://crm_owner:s3cr3t-owner-password@db.internal:5432/crm"
)

// secretKey is a valid AUTH_HMAC_KEY (32 bytes, standard base64).
// badKey is an invalid AUTH_HMAC_KEY (not base64); it must not leak either.
const badKey = "!!invalid-key-value!!"

var secretKey = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))

// env builds a getenv from the minimal valid environment plus overrides.
// An override with the empty string removes the variable.
func env(overrides map[string]string) func(string) string {
	m := map[string]string{
		"DATABASE_URL":           secretDatabase,
		"DATABASE_MIGRATION_URL": secretMigration,
		"AUTH_HMAC_KEY":          secretKey,
		"APP_BASE_URL":           "https://crm.example",
	}
	for k, v := range overrides {
		if v == "" {
			delete(m, k)
			continue
		}
		m[k] = v
	}
	return func(k string) string { return m[k] }
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TLS != nil {
		t.Errorf("TLS = %+v, want nil", cfg.TLS)
	}
	if cfg.IsLocal() {
		t.Error("IsLocal() = true, want false for https://crm.example")
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"HTTPAddr", cfg.HTTPAddr, ":8080"},
		{"MetricsAddr", cfg.MetricsAddr, "127.0.0.1:9090"},
		{"SessionIdle", cfg.SessionIdle, 24 * time.Hour},
		{"SessionAbsolute", cfg.SessionAbsolute, 168 * time.Hour},
		{"AppLinkReset", cfg.AppLinkReset, "/reset-password"},
		{"AppLinkVerify", cfg.AppLinkVerify, "/verify-email"},
		{"AppLinkInvitation", cfg.AppLinkInvitation, "/accept-invitation"},
		{"DatabaseURL", cfg.DatabaseURL, secretDatabase},
		{"DatabaseMigrationURL", cfg.DatabaseMigrationURL, secretMigration},
		{"AppBaseURL", cfg.AppBaseURL.String(), "https://crm.example"},
		{"len(AuthHMACKey)", len(cfg.AuthHMACKey), 32},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{
		"HTTP_ADDR":           ":8443",
		"METRICS_ADDR":        "127.0.0.1:9191",
		"SESSION_IDLE":        "1h",
		"SESSION_ABSOLUTE":    "2h",
		"APP_LINK_RESET":      "/r",
		"APP_LINK_VERIFY":     "/v",
		"APP_LINK_INVITATION": "/i",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8443" || cfg.MetricsAddr != "127.0.0.1:9191" ||
		cfg.SessionIdle != time.Hour || cfg.SessionAbsolute != 2*time.Hour ||
		cfg.AppLinkReset != "/r" || cfg.AppLinkVerify != "/v" || cfg.AppLinkInvitation != "/i" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

// Email links are built as APP_BASE_URL + APP_LINK_*, so a trailing slash is normalized away.
func TestLoad_BaseURLIsNormalized(t *testing.T) {
	for in, want := range map[string]string{
		"https://crm.example":      "https://crm.example",
		"https://crm.example/":     "https://crm.example",
		"https://localhost:8443/":  "https://localhost:8443",
		"https://crm.example:8443": "https://crm.example:8443",
	} {
		cfg, err := config.Load(env(map[string]string{"APP_BASE_URL": in}))
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got := cfg.AppBaseURL.String(); got != want {
			t.Errorf("APP_BASE_URL %q -> %q, want %q", in, got, want)
		}
	}
}

func TestLoad_MigrationURLIsOptional(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"DATABASE_MIGRATION_URL": ""}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DatabaseMigrationURL != "" {
		t.Errorf("DatabaseMigrationURL = %q, want empty", cfg.DatabaseMigrationURL)
	}
}

func TestLoad_LocalMode(t *testing.T) {
	tls := map[string]string{"TLS_CERT_FILE": ".certs/localhost.pem", "TLS_KEY_FILE": ".certs/localhost-key.pem"}
	merge := func(base string, extra map[string]string) map[string]string {
		m := map[string]string{"APP_BASE_URL": base}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	tests := []struct {
		name      string
		env       map[string]string
		wantLocal bool
		wantTLS   *config.TLSFiles
	}{
		{"localhost:8443 without TLS", merge("https://localhost:8443", nil), true, nil},
		{"localhost:5173 without TLS", merge("https://localhost:5173", nil), true, nil},
		{"127.0.0.1:8443 without TLS", merge("https://127.0.0.1:8443", nil), true, nil},
		{"localhost without port", merge("https://localhost", nil), true, nil},
		{"localhost:8443 with TLS", merge("https://localhost:8443", tls), true,
			&config.TLSFiles{CertFile: ".certs/localhost.pem", KeyFile: ".certs/localhost-key.pem"}},
		{"localhost:5173 with TLS", merge("https://localhost:5173", tls), true,
			&config.TLSFiles{CertFile: ".certs/localhost.pem", KeyFile: ".certs/localhost-key.pem"}},
		{"127.0.0.1:8443 with TLS", merge("https://127.0.0.1:8443", tls), true,
			&config.TLSFiles{CertFile: ".certs/localhost.pem", KeyFile: ".certs/localhost-key.pem"}},
		{"subdomain of a real domain is not local", merge("https://localhost.crm.example", nil), false, nil},
		{"private IP is not local", merge("https://192.168.0.10:8443", nil), false, nil},
		{"real domain", merge("https://crm.example", nil), false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(env(tt.env))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.IsLocal(); got != tt.wantLocal {
				t.Errorf("IsLocal() = %v, want %v", got, tt.wantLocal)
			}
			if !reflect.DeepEqual(cfg.TLS, tt.wantTLS) {
				t.Errorf("TLS = %+v, want %+v", cfg.TLS, tt.wantTLS)
			}
		})
	}
}

func TestLoad_Errors(t *testing.T) {
	certOnly := map[string]string{"APP_BASE_URL": "https://localhost:8443", "TLS_CERT_FILE": "c.pem"}
	keyOnly := map[string]string{"APP_BASE_URL": "https://localhost:8443", "TLS_KEY_FILE": "k.pem"}
	tests := []struct {
		name    string
		env     map[string]string
		wantVar string // variable name the error must mention
	}{
		{"missing DATABASE_URL", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL"},
		{"missing AUTH_HMAC_KEY", map[string]string{"AUTH_HMAC_KEY": ""}, "AUTH_HMAC_KEY"},
		{"missing APP_BASE_URL", map[string]string{"APP_BASE_URL": ""}, "APP_BASE_URL"},
		{"AUTH_HMAC_KEY of 31 bytes", map[string]string{
			"AUTH_HMAC_KEY": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 31)))}, "AUTH_HMAC_KEY"},
		{"AUTH_HMAC_KEY not base64", map[string]string{"AUTH_HMAC_KEY": badKey}, "AUTH_HMAC_KEY"},
		{"TLS in production mode", map[string]string{"TLS_CERT_FILE": "c.pem", "TLS_KEY_FILE": "k.pem"}, "TLS"},
		{"only TLS_CERT_FILE (local)", certOnly, "TLS_KEY_FILE"},
		{"only TLS_KEY_FILE (local)", keyOnly, "TLS_CERT_FILE"},
		{"only TLS_CERT_FILE (production)", map[string]string{"TLS_CERT_FILE": "c.pem"}, "TLS"},
		{"http://localhost:8080", map[string]string{"APP_BASE_URL": "http://localhost:8080"}, "APP_BASE_URL"},
		{"http://127.0.0.1:8080", map[string]string{"APP_BASE_URL": "http://127.0.0.1:8080"}, "APP_BASE_URL"},
		{"http://crm.example", map[string]string{"APP_BASE_URL": "http://crm.example"}, "APP_BASE_URL"},
		{"no scheme", map[string]string{"APP_BASE_URL": "crm.example"}, "APP_BASE_URL"},
		{"host:port looks like a scheme", map[string]string{"APP_BASE_URL": "localhost:8443"}, "APP_BASE_URL"},
		{"ftp scheme", map[string]string{"APP_BASE_URL": "ftp://crm.example"}, "APP_BASE_URL"},
		{"https without host", map[string]string{"APP_BASE_URL": "https://"}, "APP_BASE_URL"},
		{"with user info", map[string]string{"APP_BASE_URL": "https://admin:s3cr3t-userinfo@crm.example"}, "APP_BASE_URL"},
		{"with query", map[string]string{"APP_BASE_URL": "https://crm.example?x=1"}, "APP_BASE_URL"},
		{"with fragment", map[string]string{"APP_BASE_URL": "https://crm.example#x"}, "APP_BASE_URL"},
		{"with a path", map[string]string{"APP_BASE_URL": "https://crm.example/app"}, "APP_BASE_URL"},
		{"with an empty query marker", map[string]string{"APP_BASE_URL": "https://crm.example/?"}, "APP_BASE_URL"},
		{"APP_LINK_RESET without slash", map[string]string{"APP_LINK_RESET": "reset"}, "APP_LINK_RESET"},
		{"APP_LINK_VERIFY with fragment", map[string]string{"APP_LINK_VERIFY": "/verify#x"}, "APP_LINK_VERIFY"},
		{"APP_LINK_INVITATION with query", map[string]string{"APP_LINK_INVITATION": "/i?x=1"}, "APP_LINK_INVITATION"},
		{"SESSION_IDLE > SESSION_ABSOLUTE", map[string]string{"SESSION_IDLE": "48h", "SESSION_ABSOLUTE": "24h"}, "SESSION_IDLE"},
		{"SESSION_IDLE malformed", map[string]string{"SESSION_IDLE": "one day"}, "SESSION_IDLE"},
		{"SESSION_ABSOLUTE malformed", map[string]string{"SESSION_ABSOLUTE": "7d"}, "SESSION_ABSOLUTE"},
		{"SESSION_IDLE zero", map[string]string{"SESSION_IDLE": "0s"}, "SESSION_IDLE"},
		{"SESSION_ABSOLUTE negative", map[string]string{"SESSION_ABSOLUTE": "-1h"}, "SESSION_ABSOLUTE"},
	}
	secrets := []string{"s3cr3t-userinfo", secretDBPassword, secretDatabase, secretMigration, secretKey, badKey}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.Load(env(tt.env))
			if err == nil {
				t.Fatal("Load returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantVar) {
				t.Errorf("error %q does not name %s", err, tt.wantVar)
			}
			// The error must never carry the value of a secret variable.
			for _, s := range secrets {
				if strings.Contains(err.Error(), s) {
					t.Errorf("error %q leaks the secret value %q", err, s)
				}
			}
		})
	}
}

// COOKIE_SECURE no longer exists (DD-24, INV-23): setting it changes nothing and the Config
// has no field that could weaken the session cookie.
func TestLoad_CookieSecureIsIgnored(t *testing.T) {
	base, err := config.Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, v := range []string{"false", "0", "true", "anything"} {
		got, err := config.Load(env(map[string]string{"COOKIE_SECURE": v}))
		if err != nil {
			t.Fatalf("COOKIE_SECURE=%q: Load: %v", v, err)
		}
		if !reflect.DeepEqual(got, base) {
			t.Errorf("COOKIE_SECURE=%q changed the config", v)
		}
	}
	typ := reflect.TypeOf(config.Config{})
	for i := range typ.NumField() {
		name := strings.ToLower(typ.Field(i).Name)
		if strings.Contains(name, "cookie") || strings.Contains(name, "secure") || strings.Contains(name, "insecure") {
			t.Errorf("Config.%s must not exist (INV-23)", typ.Field(i).Name)
		}
	}
}

// TRUSTED_PROXIES (DD-32, plan §10.5): the proxies whose X-Forwarded-For is believed. Empty by default.
func TestLoad_TrustedProxies(t *testing.T) {
	pfx := func(ss ...string) []netip.Prefix {
		var out []netip.Prefix
		for _, s := range ss {
			out = append(out, netip.MustParsePrefix(s))
		}
		return out
	}
	tests := []struct {
		name, value string
		want        []netip.Prefix
	}{
		{"unset", "", nil},
		{"one IPv4 CIDR", "10.0.0.0/8", pfx("10.0.0.0/8")},
		{"IPv4 and IPv6 with spaces around the commas", "10.0.0.0/8 , 2001:db8::/32", pfx("10.0.0.0/8", "2001:db8::/32")},
		{"a bare IPv4 address is a /32", "10.1.2.3", pfx("10.1.2.3/32")},
		{"a bare IPv6 address is a /128", "2001:db8::1", pfx("2001:db8::1/128")},
		{"host bits are masked", "10.1.2.3/8", pfx("10.0.0.0/8")},
		{"trailing comma and blanks are ignored", "10.0.0.0/8, ,", pfx("10.0.0.0/8")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(env(map[string]string{"TRUSTED_PROXIES": tt.value}))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !reflect.DeepEqual(cfg.TrustedProxies, tt.want) {
				t.Errorf("TrustedProxies = %v, want %v", cfg.TrustedProxies, tt.want)
			}
		})
	}
}

func TestLoad_TrustedProxiesErrors(t *testing.T) {
	tests := []struct {
		name, value string
		wantInError []string
	}{
		{"trust everybody IPv4", "0.0.0.0/0", []string{"TRUSTED_PROXIES", "0.0.0.0/0"}},
		{"trust everybody IPv6", "::/0", []string{"TRUSTED_PROXIES", "::/0"}},
		{"trust everybody inside a list", "10.0.0.0/8,0.0.0.0/0", []string{"TRUSTED_PROXIES", "#2"}},
		{"invalid CIDR names the position", "10.0.0.0/8,not-a-cidr,192.168.0.0/16", []string{"TRUSTED_PROXIES", "#2"}},
		{"IPv4-mapped trust everybody", "::ffff:0.0.0.0/96", []string{"TRUSTED_PROXIES", "#1", "IPv4-mapped", "every address"}},
		{"IPv4-mapped prefix suggests the IPv4", "10.0.0.0/8,::ffff:10.0.0.0/104", []string{"TRUSTED_PROXIES", "#2", "IPv4-mapped", "write the IPv4 directly", "10.0.0.0/8"}},
		{"IPv4-mapped bare address", "::ffff:10.0.0.1", []string{"TRUSTED_PROXIES", "#1", "IPv4-mapped", "10.0.0.1/32"}},
		{"IPv4-mapped prefix shorter than /96", "::ffff:0:0/80", []string{"TRUSTED_PROXIES", "#1", "IPv4-mapped", "write the IPv4 directly"}},
		{"prefix too long", "10.0.0.0/33", []string{"TRUSTED_PROXIES", "#1"}},
		{"garbage", "???", []string{"TRUSTED_PROXIES", "#1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.Load(env(map[string]string{"TRUSTED_PROXIES": tt.value}))
			if err == nil {
				t.Fatal("Load returned nil error")
			}
			for _, want := range tt.wantInError {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestLoad_ObjectStorage(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"APP_BASE_URL": "https://localhost:8080"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.S3Endpoint != "localhost:9000" || cfg.S3Bucket != "crm-dev" || cfg.S3AccessKey != "minio_dev" || cfg.S3SecretKey != "minio_dev_secret" || cfg.S3UseSSL {
		t.Fatal("local S3 defaults do not match compose")
	}
	cfg, err = config.Load(env(map[string]string{"S3_ENDPOINT": "s3.example", "S3_BUCKET": "private", "S3_ACCESS_KEY": "access", "S3_SECRET_KEY": "secret", "S3_USE_SSL": "true"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.S3Endpoint != "s3.example" || cfg.S3Bucket != "private" || cfg.S3AccessKey != "access" || cfg.S3SecretKey != "secret" || !cfg.S3UseSSL {
		t.Fatal("S3 settings not propagated")
	}
	if _, err = config.Load(env(map[string]string{"S3_USE_SSL": "invalid"})); err == nil || !strings.Contains(err.Error(), "S3_USE_SSL") {
		t.Fatalf("invalid SSL=%v", err)
	}
}
