package settings

import (
	"context"
	"errors"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/google/uuid"
)

var errStub = errors.New("not implemented")

func (s *Service) ListCategories(ctx context.Context, p authz.Principal, f CategoryFilter) ([]Category, error) {
	return nil, errStub
}
func (s *Service) CreateCategory(ctx context.Context, p authz.Principal, name string, t CategoryType, meta RequestMeta) (Category, error) {
	return Category{}, errStub
}
func (s *Service) RenameCategory(ctx context.Context, p authz.Principal, id uuid.UUID, name string, meta RequestMeta) (Category, error) {
	return Category{}, errStub
}
func (s *Service) ArchiveCategory(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) (Category, error) {
	return Category{}, errStub
}
func (s *Service) RestoreCategory(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) (Category, error) {
	return Category{}, errStub
}
