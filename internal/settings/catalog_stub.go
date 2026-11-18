package settings

import (
	"context"
	"errors"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/google/uuid"
)

var errCatalogNotImplemented = errors.New("settings: catalog use cases not implemented")

func (s *Service) ListCatalogItems(ctx context.Context, p authz.Principal, includeArchived bool) ([]CatalogItem, error) {
	return nil, errCatalogNotImplemented
}

func (s *Service) GetCatalogItem(ctx context.Context, p authz.Principal, id uuid.UUID) (CatalogItem, error) {
	return CatalogItem{}, errCatalogNotImplemented
}

func (s *Service) CreateCatalogItem(ctx context.Context, p authz.Principal, in NewCatalogItem, meta RequestMeta) (CatalogItem, error) {
	return CatalogItem{}, errCatalogNotImplemented
}

func (s *Service) UpdateCatalogItem(ctx context.Context, p authz.Principal, id uuid.UUID, ch CatalogItemChanges, meta RequestMeta) (CatalogItem, error) {
	return CatalogItem{}, errCatalogNotImplemented
}

func (s *Service) ArchiveCatalogItem(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) (CatalogItem, error) {
	return CatalogItem{}, errCatalogNotImplemented
}

func (s *Service) RestoreCatalogItem(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) (CatalogItem, error) {
	return CatalogItem{}, errCatalogNotImplemented
}
