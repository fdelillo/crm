package settings

import (
	"errors"
	"log/slog"
	"net/netip"

	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
)

// Sentinels of the use cases. http.go maps each one to its status and code (plan 9.3).
var (
	ErrNotFound         = errors.New("settings: not found")                          // 404, also for another company or a deleted row
	ErrNameTaken        = errors.New("settings: name taken")                         // 409 name_taken
	ErrPipelineRule     = errors.New("settings: pipeline rule violated")             // 409 invalid_state
	ErrSystemCategory   = errors.New("settings: system category cannot be archived") // 409 invalid_state
	ErrMalformedChanges = errors.New("settings: malformed changes")                  // 400 malformed_request
)

// FieldErrors maps a request field to a stable contract code. It never includes submitted values.
type FieldErrors map[string]string

func (FieldErrors) Error() string { return "settings: invalid fields" }

// RequestMeta is declared here so settings does not depend on identity for a type (DD-1).
type RequestMeta struct {
	IP        netip.Addr
	UserAgent string
}

// T-002-B205 (DD-1): skeleton; the use cases arrive with their own tasks.
type Service struct {
	runner db.TxRunner
	audit  audit.Recorder
	clock  clock.Clock
	logger *slog.Logger
}

// NewService panics on a nil dependency, like tenant.NewService.
func NewService(runner db.TxRunner, recorder audit.Recorder, c clock.Clock, logger *slog.Logger) *Service {
	if runner == nil || recorder == nil || c == nil || logger == nil {
		panic("settings: invalid service dependencies")
	}
	return &Service{runner: runner, audit: recorder, clock: c, logger: logger}
}
