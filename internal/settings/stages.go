package settings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings/store"
	"github.com/google/uuid"
)

// stageNameConstraint is the partial unique index of active names (002/INV-6): the database, not the service,
// decides that a name is taken.
const stageNameConstraint = "stages_active_name_key"

// ListStages returns the active stages of the company in pipeline order. Any role may read (002/DD-11).
func (s *Service) ListStages(ctx context.Context, p authz.Principal) ([]Stage, error) {
	var out []Stage
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		stages, err := listActiveStages(ctx, store.New(tx), p.TenantID)
		out = stages
		return err
	})
	return out, err
}

// CreateStage adds a stage at in.Position (nil = at the end) and returns it. Invalid fields are FieldErrors
// (name, type, color, position); a pipeline that would break is ErrPipelineRule; a taken name is ErrNameTaken.
// T-002-B402 (002/DD-5, DD-18, INV-2, INV-6)
func (s *Service) CreateStage(ctx context.Context, p authz.Principal, in NewStage, meta RequestMeta) (Stage, error) {
	if err := requireSettingsManage(p); err != nil {
		return Stage{}, err
	}
	fields := FieldErrors{}
	name, code := normalizeStageName(in.Name)
	setFieldCode(fields, "name", code)
	setFieldCode(fields, "type", validateStageType(in.Type))
	setFieldCode(fields, "color", validateColor(in.Color))
	if len(fields) > 0 {
		return Stage{}, fields
	}

	var created Stage
	err := s.writeStages(ctx, p, meta, func(current []Stage, now time.Time) (stageWrite, error) {
		position := len(current) + 1
		if in.Position != nil {
			position = *in.Position
		}
		created = Stage{ID: uuid.Must(uuid.NewV7()), Name: name, Type: in.Type, Color: in.Color}
		result, err := insertStage(current, created, position)
		if err != nil {
			return stageWrite{}, err
		}
		created.Position = position
		return stageWrite{
			result:     result,
			reposition: true,
			write: func(ctx context.Context, q *store.Queries) error {
				return q.InsertStage(ctx, store.InsertStageParams{ID: created.ID, TenantID: p.TenantID, Name: created.Name,
					Type: string(created.Type), Color: string(created.Color), Position: int32(position), Now: now})
			},
			audit: stageAudit{action: "stage.created", target: created.ID, data: map[string]any{"type": string(created.Type), "position": position}},
		}, nil
	})
	if err != nil {
		return Stage{}, err
	}
	return created, nil
}

// UpdateStage changes the name, type or color of an active stage and returns it. Nothing to change
// (every value equals the current one) writes and audits nothing. A stage that is not active, or not the
// company's, is ErrNotFound; no field at all is ErrMalformedChanges.
// T-002-B402 (002/DD-5, DD-15, INV-2, INV-6, INV-13)
func (s *Service) UpdateStage(ctx context.Context, p authz.Principal, id uuid.UUID, ch StageChanges, meta RequestMeta) (Stage, error) {
	if err := requireSettingsManage(p); err != nil {
		return Stage{}, err
	}
	if ch.Name == nil && ch.Type == nil && ch.Color == nil {
		return Stage{}, ErrMalformedChanges
	}
	fields := FieldErrors{}
	var name string
	if ch.Name != nil {
		var code string
		name, code = normalizeStageName(*ch.Name)
		setFieldCode(fields, "name", code)
	}
	if ch.Type != nil {
		setFieldCode(fields, "type", validateStageType(*ch.Type))
	}
	if ch.Color != nil {
		setFieldCode(fields, "color", validateColor(*ch.Color))
	}
	if len(fields) > 0 {
		return Stage{}, fields
	}

	var updated Stage
	err := s.writeStages(ctx, p, meta, func(current []Stage, now time.Time) (stageWrite, error) {
		index := stageIndex(current, id)
		if index < 0 {
			return stageWrite{}, ErrNotFound
		}
		updated = current[index]
		var changed []string // in the order of the audit catalog: name, type, color
		if ch.Name != nil && name != updated.Name {
			updated.Name = name
			changed = append(changed, "name")
		}
		if ch.Type != nil && *ch.Type != updated.Type {
			updated.Type = *ch.Type
			changed = append(changed, "type")
		}
		if ch.Color != nil && *ch.Color != updated.Color {
			updated.Color = *ch.Color
			changed = append(changed, "color")
		}
		if len(changed) == 0 {
			return stageWrite{noop: true}, nil
		}
		result := append([]Stage(nil), current...)
		result[index] = updated
		return stageWrite{
			result: result,
			write: func(ctx context.Context, q *store.Queries) error {
				return q.UpdateStageFields(ctx, store.UpdateStageFieldsParams{TenantID: p.TenantID, ID: id, Name: updated.Name,
					Type: string(updated.Type), Color: string(updated.Color), Now: now})
			},
			audit: stageAudit{action: "stage.updated", target: id, data: map[string]any{"fields": changed}},
		}, nil
	})
	if err != nil {
		return Stage{}, err
	}
	return updated, nil
}

// DeleteStage removes an active stage from the pipeline (logical delete, no undo) and closes the gap in the
// positions. A stage that is not active, or not the company's, is ErrNotFound; a pipeline that would break
// (the only won stage, or the first one when the next is not open) is ErrPipelineRule.
// T-002-B402 (002/DD-5, DD-6, INV-2)
func (s *Service) DeleteStage(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) error {
	if err := requireSettingsManage(p); err != nil {
		return err
	}
	return s.writeStages(ctx, p, meta, func(current []Stage, now time.Time) (stageWrite, error) {
		index := stageIndex(current, id)
		if index < 0 {
			return stageWrite{}, ErrNotFound
		}
		result, err := removeStage(current, index+1)
		if err != nil {
			return stageWrite{}, err
		}
		return stageWrite{
			result:     result,
			reposition: true,
			write: func(ctx context.Context, q *store.Queries) error {
				return q.SoftDeleteStage(ctx, store.SoftDeleteStageParams{TenantID: p.TenantID, ID: id, Now: now})
			},
			audit: stageAudit{action: "stage.deleted", target: id, data: map[string]any{}},
		}, nil
	})
}

// MoveStage puts a stage at position (1..n) and returns the whole pipeline. The current position is a no-op:
// the list comes back unchanged and nothing is audited. A position out of range is FieldErrors{position}.
// T-002-B402 (002/DD-5, DD-18, INV-2, INV-13)
func (s *Service) MoveStage(ctx context.Context, p authz.Principal, id uuid.UUID, position int, meta RequestMeta) ([]Stage, error) {
	if err := requireSettingsManage(p); err != nil {
		return nil, err
	}
	var out []Stage
	err := s.writeStages(ctx, p, meta, func(current []Stage, _ time.Time) (stageWrite, error) {
		index := stageIndex(current, id)
		if index < 0 {
			return stageWrite{}, ErrNotFound
		}
		result, changed, err := moveStage(current, index+1, position)
		if err != nil {
			return stageWrite{}, err
		}
		out = result
		if !changed {
			return stageWrite{noop: true}, nil
		}
		return stageWrite{
			result:     result,
			reposition: true,
			audit:      stageAudit{action: "stage.moved", target: id, data: map[string]any{"from": index + 1, "to": position}},
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// stageWrite is what one stage operation decided, from the pipeline it found under the lock.
type stageWrite struct {
	noop       bool    // nothing changes: no statement and no audit (002/INV-13)
	result     []Stage // the active stages after the operation, in order
	reposition bool    // positions of other stages change: rewrite them all
	// write is the statement of the operation itself (insert, update, soft delete); nil for a move, which is
	// only positions.
	write func(ctx context.Context, q *store.Queries) error
	audit stageAudit
}

type stageAudit struct {
	action string
	target uuid.UUID
	data   map[string]any
}

// writeStages is the one skeleton of the four writes (002/DD-5): lock the company's settings row, read the
// active stages, let decide compute the resulting list with the pure rules, validate that list with
// validatePipeline, write, rewrite the positions in one statement and audit, all in one transaction.
// decide receives the stages as found and the instant to stamp; it returns ErrNotFound, FieldErrors or
// a rule error before anything is written.
func (s *Service) writeStages(ctx context.Context, p authz.Principal, meta RequestMeta,
	decide func(current []Stage, now time.Time) (stageWrite, error)) error {
	now := s.clock.Now()
	return s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		if _, err := q.LockTenantSettings(ctx, p.TenantID); err != nil {
			return fmt.Errorf("settings: lock tenant settings: %w", db.MapError(err))
		}
		current, err := listActiveStages(ctx, q, p.TenantID)
		if err != nil {
			return err
		}
		op, err := decide(current, now)
		if err != nil {
			return err
		}
		if op.noop {
			return nil
		}
		if err := validatePipeline(op.result); err != nil {
			return err
		}
		if op.write != nil {
			if err := op.write(ctx, q); err != nil {
				return mapStageError(err)
			}
		}
		if op.reposition {
			ids := make([]uuid.UUID, len(op.result))
			for i, st := range op.result {
				ids[i] = st.ID
			}
			if err := q.SetStagePositions(ctx, store.SetStagePositionsParams{TenantID: p.TenantID, Ids: ids, Now: now}); err != nil {
				return fmt.Errorf("settings: set stage positions: %w", db.MapError(err))
			}
		}
		target := op.audit.target
		return s.audit.Record(ctx, tx, audit.Entry{TenantID: p.TenantID, ActorUserID: &p.UserID, Action: op.audit.action,
			TargetType: "stage", TargetID: &target, Data: op.audit.data, IP: meta.IP, UserAgent: meta.UserAgent})
	})
}

func listActiveStages(ctx context.Context, q *store.Queries, tenantID uuid.UUID) ([]Stage, error) {
	rows, err := q.ListActiveStages(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("settings: list stages: %w", db.MapError(err))
	}
	out := make([]Stage, len(rows))
	for i, r := range rows {
		out[i] = Stage{ID: r.ID, Name: r.Name, Type: StageType(r.Type), Color: Color(r.Color), Position: int(r.Position)}
	}
	return out, nil
}

// mapStageError turns the violation of the active-name index into ErrNameTaken; the rest is classified as usual.
func mapStageError(err error) error {
	mapped := db.MapError(err)
	var ce *db.ConstraintError
	if errors.As(mapped, &ce) && ce.Constraint == stageNameConstraint {
		return ErrNameTaken
	}
	return fmt.Errorf("settings: write stage: %w", mapped)
}

func requireSettingsManage(p authz.Principal) error {
	if !authz.Can(p.Role, authz.SettingsManage) {
		return authz.ErrForbidden
	}
	return nil
}

// setFieldCode records code under field when it is not empty.
func setFieldCode(fields FieldErrors, field, code string) {
	if code != "" {
		fields[field] = code
	}
}

func stageIndex(stages []Stage, id uuid.UUID) int {
	for i, st := range stages {
		if st.ID == id {
			return i
		}
	}
	return -1
}
