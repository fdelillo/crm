package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/fdelillo/crm/internal/platform/audit/store"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Entry is one audit_log row to write (data-model.md §2.6, plan FR-008): who (ActorUserID, IP,
// UserAgent), when (the database's own now(), not set here), and what (Action, Target*, Data).
type Entry struct {
	TenantID    uuid.UUID
	ActorUserID *uuid.UUID
	Action      string
	TargetType  string
	TargetID    *uuid.UUID
	Data        map[string]any
	IP          netip.Addr
	UserAgent   string
}

// Recorder writes one Entry to audit_log in the caller's own transaction (tx), so the audit trail
// commits or rolls back with the operation it documents (plan FR-008).
type Recorder interface {
	Record(ctx context.Context, tx db.Tx, e Entry) error
}

// Sentinels of Record (DD-37, INV-32). Errors wrap them with the path of the offending key, never
// with the value.
var (
	ErrSecretInData    = errors.New("audit: data contains a credential")
	ErrUnsupportedData = errors.New("audit: data contains an unsupported type")
	ErrTxRequired      = errors.New("audit: transaction is required")
)

type recorder struct{}

// NewRecorder returns the Recorder. It has no state: every call is independent.
func NewRecorder() Recorder { return recorder{} }

// Record validates Data (DD-37), normalizes UserAgent (DD-38) and inserts the row in tx.
func (recorder) Record(ctx context.Context, tx db.Tx, e Entry) error {
	// Validation runs before looking at tx: a credential must be reported as such even when the
	// caller also forgot the transaction (DD-37).
	if err := validateData("", e.Data); err != nil {
		return err
	}
	if tx == nil {
		return ErrTxRequired
	}
	data := e.Data
	if data == nil {
		data = map[string]any{}
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("audit: encode data: %w", err)
	}
	var ip *netip.Addr
	if e.IP.IsValid() {
		value := e.IP.Unmap()
		ip = &value
	}
	userAgent := httpx.NormalizeUserAgent(e.UserAgent) // DD-38, INV-33: never fail the audited operation
	args := store.InsertAuditParams{
		TenantID:   e.TenantID,
		Action:     e.Action,
		Data:       encoded,
		Ip:         ip,
		TargetType: pgtype.Text{String: e.TargetType, Valid: e.TargetType != ""},
		UserAgent:  pgtype.Text{String: userAgent, Valid: userAgent != ""},
	}
	if e.ActorUserID != nil {
		args.ActorUserID = uuid.NullUUID{UUID: *e.ActorUserID, Valid: true}
	}
	if e.TargetID != nil {
		args.TargetID = uuid.NullUUID{UUID: *e.TargetID, Valid: true}
	}
	if id := httpx.RequestIDFrom(ctx); id != "" {
		args.RequestID = pgtype.Text{String: id, Valid: true}
	}
	if err := store.New(tx).InsertAudit(ctx, args); err != nil {
		return fmt.Errorf("audit: insert: %w", db.MapError(err))
	}
	return nil
}

// credentialKeyParts and credentialKeyExact are the key denylist of DD-37, compared against the
// normalized key (lower case, letters and digits only: "Set-Cookie" -> "setcookie", "X-API-Key" ->
// "xapikey"). "session" is exact because the catalog has "sessions_revoked".
var (
	credentialKeyParts = []string{"password", "passwd", "passphrase", "secret", "token", "authorization",
		"cookie", "apikey", "privatekey", "credential", "signature", "csrf", "xsrf"}
	credentialKeyExact = []string{"auth", "session", "sessionid", "sid", "otp", "pin"}
)

func isCredentialKey(key string) bool {
	var b strings.Builder
	for _, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	normalized := b.String()
	for _, part := range credentialKeyParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return slices.Contains(credentialKeyExact, normalized)
}

// isCredentialValue recognizes values that look like a credential whatever their key (DD-37 (3)):
// an Authorization-style scheme, a JWT, or the 43-character base64url form of securetoken.New.
func isCredentialValue(value string) bool {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	for _, scheme := range []string{"bearer ", "basic ", "digest "} {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	if strings.HasPrefix(value, "eyJ") {
		segments := strings.Split(value, ".")
		if len(segments) == 3 && slices.IndexFunc(segments, func(s string) bool { return !isBase64URL(s) }) < 0 {
			return true
		}
	}
	return len(value) == 43 && isBase64URL(value)
}

func isBase64URL(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// validateData walks data at every depth and rejects what DD-37 forbids. It closes the set of
// admitted types on purpose: a struct, a pointer or a []byte would be serialized without being
// inspected, which would make the credential check incomplete. Errors carry the path of the
// offending key and never the value, so the error itself cannot leak a secret (INV-32).
func validateData(path string, value any) error {
	switch v := value.(type) {
	case nil, bool, int, int32, int64, uuid.UUID, time.Time:
		return nil
	case string:
		if isCredentialValue(v) {
			return fmt.Errorf("audit: data.%s: %w", path, ErrSecretInData)
		}
		return nil
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(v)) {
			if err := validateEntry(path, key, v[key]); err != nil {
				return err
			}
		}
		return nil
	case map[string]string:
		for _, key := range slices.Sorted(maps.Keys(v)) {
			if err := validateEntry(path, key, v[key]); err != nil {
				return err
			}
		}
		return nil
	case []any:
		for i, item := range v {
			if err := validateData(fmt.Sprintf("%s[%d]", path, i), item); err != nil {
				return err
			}
		}
		return nil
	case []string:
		for i, item := range v {
			if err := validateData(fmt.Sprintf("%s[%d]", path, i), item); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("audit: data.%s: %T: %w", path, value, ErrUnsupportedData)
	}
}

func validateEntry(parent, key string, value any) error {
	path := key
	if parent != "" {
		path = parent + "." + key
	}
	if isCredentialKey(key) {
		return fmt.Errorf("audit: data.%s: %w", path, ErrSecretInData)
	}
	return validateData(path, value)
}
