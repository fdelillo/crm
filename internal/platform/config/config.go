package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const minHMACKeyBytes = 32

// TLSFiles are the paths of the local development certificate (TLS_CERT_FILE, TLS_KEY_FILE).
// Load does not read the files; `crm serve` checks them before listening (DD-24).
type TLSFiles struct {
	CertFile string // TLS_CERT_FILE
	KeyFile  string // TLS_KEY_FILE
}

// Config is the typed configuration of the binary (plan §10.5, §11.1).
//
// There is deliberately no field that could weaken the session cookie: it is always Secure
// (INV-23, DD-24).
type Config struct {
	DatabaseURL          string // DATABASE_URL (role crm_app). Secret.
	DatabaseMigrationURL string // DATABASE_MIGRATION_URL (role crm_owner); only `crm migrate` needs it. Secret.
	AuthHMACKey          []byte // AUTH_HMAC_KEY, standard base64, at least 32 bytes decoded. Secret.
	AppBaseURL           *url.URL
	AppLinkReset         string
	AppLinkVerify        string
	AppLinkInvitation    string
	SessionIdle          time.Duration
	SessionAbsolute      time.Duration
	HTTPAddr             string
	MetricsAddr          string
	TLS                  *TLSFiles // nil except in local mode with TLS_* set
}

// Load reads the environment through getenv and validates the rules of plan §10.5.
// It never reads files. Errors name the offending variable and never include the value of a
// secret one.
func Load(getenv func(string) string) (Config, error) {
	var c Config
	var err error

	if c.DatabaseURL, err = required(getenv, "DATABASE_URL"); err != nil {
		return Config{}, err
	}
	c.DatabaseMigrationURL = getenv("DATABASE_MIGRATION_URL")
	if c.AuthHMACKey, err = hmacKey(getenv("AUTH_HMAC_KEY")); err != nil {
		return Config{}, err
	}
	if c.AppBaseURL, err = baseURL(getenv("APP_BASE_URL")); err != nil {
		return Config{}, err
	}
	if c.TLS, err = tlsFiles(getenv, c.AppBaseURL); err != nil {
		return Config{}, err
	}
	links := []struct {
		dst      *string
		name     string
		fallback string
	}{
		{&c.AppLinkReset, "APP_LINK_RESET", "/reset-password"},
		{&c.AppLinkVerify, "APP_LINK_VERIFY", "/verify-email"},
		{&c.AppLinkInvitation, "APP_LINK_INVITATION", "/accept-invitation"},
	}
	for _, l := range links {
		if *l.dst, err = appLink(l.name, getenv(l.name), l.fallback); err != nil {
			return Config{}, err
		}
	}
	if c.SessionIdle, err = duration(getenv, "SESSION_IDLE", 24*time.Hour); err != nil {
		return Config{}, err
	}
	if c.SessionAbsolute, err = duration(getenv, "SESSION_ABSOLUTE", 168*time.Hour); err != nil {
		return Config{}, err
	}
	if c.SessionIdle > c.SessionAbsolute {
		return Config{}, errors.New("config: SESSION_IDLE must not be greater than SESSION_ABSOLUTE")
	}
	c.HTTPAddr = orDefault(getenv("HTTP_ADDR"), ":8080")
	c.MetricsAddr = orDefault(getenv("METRICS_ADDR"), "127.0.0.1:9090")
	return c, nil
}

// IsLocal reports whether APP_BASE_URL points at this machine: the host is exactly localhost
// or 127.0.0.1 (any port). It is the only place that decides "local mode", used for TLS and
// HSTS (DD-24).
func (c Config) IsLocal() bool {
	if c.AppBaseURL == nil {
		return false
	}
	switch strings.ToLower(c.AppBaseURL.Hostname()) {
	case "localhost", "127.0.0.1":
		return true
	}
	return false
}

func required(getenv func(string) string, name string) (string, error) {
	v := getenv(name)
	if v == "" {
		return "", fmt.Errorf("config: %s is required", name)
	}
	return v, nil
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// hmacKey decodes AUTH_HMAC_KEY. The error never includes the value.
func hmacKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, errors.New("config: AUTH_HMAC_KEY is required")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New("config: AUTH_HMAC_KEY must be standard base64")
	}
	if len(key) < minHMACKeyBytes {
		return nil, fmt.Errorf("config: AUTH_HMAC_KEY must decode to at least %d bytes", minHMACKeyBytes)
	}
	return key, nil
}

// baseURL validates APP_BASE_URL: always https (H-10, DD-24).
func baseURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("config: APP_BASE_URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return nil, errors.New("config: APP_BASE_URL must be an absolute https:// URL")
	}
	return u, nil
}

// tlsFiles validates TLS_CERT_FILE / TLS_KEY_FILE: both or none, and only in local mode.
func tlsFiles(getenv func(string) string, base *url.URL) (*TLSFiles, error) {
	cert, key := getenv("TLS_CERT_FILE"), getenv("TLS_KEY_FILE")
	if cert == "" && key == "" {
		return nil, nil
	}
	if !(Config{AppBaseURL: base}).IsLocal() {
		return nil, errors.New("config: TLS_CERT_FILE/TLS_KEY_FILE are only accepted in local mode (APP_BASE_URL host localhost or 127.0.0.1): in production TLS is terminated by the hosting")
	}
	if cert == "" {
		return nil, errors.New("config: TLS_CERT_FILE is required when TLS_KEY_FILE is set")
	}
	if key == "" {
		return nil, errors.New("config: TLS_KEY_FILE is required when TLS_CERT_FILE is set")
	}
	return &TLSFiles{CertFile: cert, KeyFile: key}, nil
}

// appLink validates a path used to build email links: it starts with "/" and has no query or
// fragment (the token goes in the fragment, appended later, DD-14).
func appLink(name, raw, fallback string) (string, error) {
	v := orDefault(raw, fallback)
	if !strings.HasPrefix(v, "/") || strings.ContainsAny(v, "#?") {
		return "", fmt.Errorf("config: %s must start with '/' and contain no '#' or '?'", name)
	}
	return v, nil
}

func duration(getenv func(string) string, name string, fallback time.Duration) (time.Duration, error) {
	raw := getenv(name)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("config: %s must be a positive duration such as 24h", name)
	}
	return d, nil
}
