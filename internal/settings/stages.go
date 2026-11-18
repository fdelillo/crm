package settings

import (
	"context"
	"errors"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/google/uuid"
)

var errStageUseCasesPending = errors.New("settings: stage use cases not implemented")

// ListStages returns the active stages of the company in pipeline order. Any role may read (002/DD-11).
func (s *Service) ListStages(ctx context.Context, p authz.Principal) ([]Stage, error) {
	return nil, errStageUseCasesPending
}

// CreateStage adds a stage at in.Position (nil = at the end) and returns it.
func (s *Service) CreateStage(ctx context.Context, p authz.Principal, in NewStage, meta RequestMeta) (Stage, error) {
	return Stage{}, errStageUseCasesPending
}

// UpdateStage changes the name, type or color of an active stage.
func (s *Service) UpdateStage(ctx context.Context, p authz.Principal, id uuid.UUID, ch StageChanges, meta RequestMeta) (Stage, error) {
	return Stage{}, errStageUseCasesPending
}

// DeleteStage removes an active stage from the pipeline (soft delete).
func (s *Service) DeleteStage(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) error {
	return errStageUseCasesPending
}

// MoveStage puts a stage at position and returns the whole pipeline.
func (s *Service) MoveStage(ctx context.Context, p authz.Principal, id uuid.UUID, position int, meta RequestMeta) ([]Stage, error) {
	return nil, errStageUseCasesPending
}
