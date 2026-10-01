package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"

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
type recorder struct{}

// NewRecorder returns the Recorder. It has no state: every call is independent.
func NewRecorder() Recorder { return recorder{} }

func (recorder) Record(ctx context.Context, tx db.Tx, e Entry) error {
	if containsSecret(e.Data) {
		return errors.New("audit: data contains a secret key")
	}
	if tx == nil {
		return errors.New("audit: transaction is required")
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
	args := store.InsertAuditParams{
		TenantID:   e.TenantID,
		Action:     e.Action,
		Data:       encoded,
		Ip:         ip,
		TargetType: pgtype.Text{String: e.TargetType, Valid: e.TargetType != ""},
		UserAgent:  pgtype.Text{String: e.UserAgent, Valid: e.UserAgent != ""},
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

// containsSecret rejects an Entry.Data whose keys look like a credential, at any depth: a caller
// that builds Data from, say, request headers could otherwise write a password or token to
// audit_log (append-only, never scrubbed). map[string]string is checked explicitly and not only
// through the map[string]any case: a type switch does not see through a concrete map type, so a
// caller that passes one directly (headers, form values) would otherwise skip the check entirely.
func containsSecret(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			normalized := strings.ToLower(key)
			if strings.Contains(normalized, "password") || strings.Contains(normalized, "token") || containsSecret(item) {
				return true
			}
		}
	case map[string]string:
		for key := range v {
			normalized := strings.ToLower(key)
			if strings.Contains(normalized, "password") || strings.Contains(normalized, "token") {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if containsSecret(item) {
				return true
			}
		}
	}
	return false
}
