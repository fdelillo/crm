package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox/store"
	"github.com/google/uuid"
)

type Message struct {
	TenantID  uuid.UUID
	Kind      string
	Template  string
	Recipient string
	Payload   map[string]string
}
type Enqueuer interface {
	Enqueue(ctx context.Context, tx db.Tx, m Message) error
}
type enqueuer struct{}

func NewEnqueuer() Enqueuer { return enqueuer{} }
func (enqueuer) Enqueue(ctx context.Context, tx db.Tx, m Message) error {
	if tx == nil {
		return errors.New("outbox: transaction required")
	}
	tenant, bound := tx.TenantID()
	if !bound || tenant != m.TenantID {
		return errors.New("outbox: transaction tenant mismatch")
	}
	payload, err := json.Marshal(m.Payload)
	if err != nil {
		return fmt.Errorf("outbox: encode payload: %w", err)
	}
	if err := store.New(tx).InsertMessage(ctx, store.InsertMessageParams{
		TenantID: m.TenantID, Kind: m.Kind, Template: m.Template, Recipient: m.Recipient, Payload: payload,
	}); err != nil {
		return fmt.Errorf("outbox: enqueue: %w", db.MapError(err))
	}
	return nil
}

type Handler interface {
	Handle(ctx context.Context, m Message) error
}
type PeriodicTask interface {
	Name() string
	Every() time.Duration
	Run(ctx context.Context) error
}
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return "permanent delivery failure" }
func (e *PermanentError) Unwrap() error { return e.Err }
