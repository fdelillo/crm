package tenant

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path"
	"strings"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/objectstore"
	"github.com/fdelillo/crm/internal/tenant/store"
	"github.com/google/uuid"
)

const (
	LogoMaxBytes     = 2_097_152
	LogoMaxBodyBytes = LogoMaxBytes + 65_536
	LogoMaxSide      = 2000
)

var (
	ErrLogoNotFound        = errors.New("tenant: logo not found")
	ErrLogoTooLarge        = errors.New("tenant: logo too large")
	ErrLogoUnsupportedType = errors.New("tenant: unsupported logo type")
	ErrLogoInvalidImage    = errors.New("tenant: invalid logo image")
)

type ServiceOption func(*Service)

func WithObjectStorage(storage objectstore.ObjectStorage) ServiceOption {
	if storage == nil {
		panic("tenant: nil object storage")
	}
	return func(s *Service) { s.storage = storage }
}

type LogoResult struct {
	ETag        string
	NotModified bool
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

func validateLogo(data []byte) (contentType, ext string, err error) {
	if len(data) > LogoMaxBytes {
		return "", "", ErrLogoTooLarge
	}
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		contentType, ext = "image/png", "png"
	case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		contentType, ext = "image/jpeg", "jpg"
	default:
		return "", "", ErrLogoUnsupportedType
	}
	// DecodeConfig reads only metadata: never allocate or decode the image's pixels.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > LogoMaxSide || cfg.Height > LogoMaxSide {
		return "", "", ErrLogoInvalidImage
	}
	return contentType, ext, nil
}
func (s *Service) SetLogo(ctx context.Context, p authz.Principal, data []byte, meta identity.RequestMeta) (Tenant, error) {
	if !authz.Can(p.Role, authz.SettingsManage) {
		return Tenant{}, authz.ErrForbidden
	}
	ct, ext, err := validateLogo(data)
	if err != nil {
		return Tenant{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Tenant{}, fmt.Errorf("tenant: logo id: %w", err)
	}
	key := "tenants/" + p.TenantID.String() + "/logo/" + id.String() + "." + ext
	if err = s.storage.Put(ctx, key, bytes.NewReader(data), int64(len(data)), ct); err != nil {
		return Tenant{}, fmt.Errorf("tenant: put logo: %w", err)
	}
	var out Tenant
	var oldKey string
	functionCompleted := false
	err = s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		row, err := q.LockTenant(ctx, p.TenantID)
		if err != nil {
			return db.MapError(err)
		}
		oldKey = row.LogoObjectKey.String
		row, err = q.UpdateTenantLogo(ctx, store.UpdateTenantLogoParams{TenantID: p.TenantID, LogoObjectKey: nullableText(&key), LogoContentType: nullableText(&ct), UpdatedAt: time.Now().UTC()})
		if err != nil {
			return db.MapError(err)
		}
		if err = s.recordChange(ctx, tx, p, "tenant.logo_updated", map[string]any{"content_type": ct}, meta); err != nil {
			return err
		}
		out = tenantFrom(row)
		functionCompleted = true
		return nil
	})
	if err != nil {
		if !functionCompleted {
			// fn failed (or never ran): the runner guarantees a safe rollback.
			s.deleteLogo(ctx, key, "compensation")
			return Tenant{}, err
		}
		// A Commit error does not tell whether PostgreSQL committed. A locked
		// reread waits for an in-flight transaction before choosing an object.
		reconcile, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var row store.AppTenant
		readErr := s.runner.InTenantTx(reconcile, p.TenantID, func(ctx context.Context, tx db.Tx) error {
			var err error
			row, err = store.New(tx).LockTenant(ctx, p.TenantID)
			return db.MapError(err)
		})
		outcome := "unknown"
		if readErr == nil {
			outcome = "rolled_back"
			if row.LogoObjectKey.Valid && row.LogoObjectKey.String == key {
				outcome = "committed"
			}
		}
		s.logger.WarnContext(reconcile, "logo commit result uncertain", "event", "logo_commit_uncertain", "tenant_id", p.TenantID.String(), "outcome", outcome)
		if outcome == "committed" {
			s.deleteLogo(ctx, oldKey, "replaced")
			return tenantFrom(row), nil
		}
		if outcome == "rolled_back" {
			s.deleteLogo(ctx, key, "compensation")
		}
		return Tenant{}, err
	}
	s.deleteLogo(ctx, oldKey, "replaced")
	return out, nil
}
func (s *Service) RemoveLogo(ctx context.Context, p authz.Principal, meta identity.RequestMeta) error {
	if !authz.Can(p.Role, authz.SettingsManage) {
		return authz.ErrForbidden
	}
	var oldKey string
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		row, err := q.LockTenant(ctx, p.TenantID)
		if err != nil {
			return db.MapError(err)
		}
		if !row.LogoObjectKey.Valid {
			return nil
		}
		oldKey = row.LogoObjectKey.String
		if _, err = q.UpdateTenantLogo(ctx, store.UpdateTenantLogoParams{TenantID: p.TenantID, UpdatedAt: time.Now().UTC()}); err != nil {
			return db.MapError(err)
		}
		return s.recordChange(ctx, tx, p, "tenant.logo_removed", map[string]any{"content_type": row.LogoContentType.String}, meta)
	})
	if err != nil {
		return err
	}
	s.deleteLogo(ctx, oldKey, "replaced")
	return nil
}

// Cleanup survives a canceled request and is bounded; a failed delete leaves an accepted orphan.
func (s *Service) deleteLogo(ctx context.Context, key, reason string) {
	if key == "" {
		return
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.storage.Delete(cleanup, key); err != nil {
		s.logger.WarnContext(cleanup, "logo object cleanup failed", "event", "logo_delete_failed", "reason", reason, "object_key", key, "error", err)
	}
}

// readLogoTenant releases the transaction before any storage call or ETag comparison.
func (s *Service) readLogoTenant(ctx context.Context, p authz.Principal) (store.AppTenant, error) {
	var row store.AppTenant
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		var err error
		row, err = store.New(tx).GetTenant(ctx, p.TenantID)
		if err != nil {
			return db.MapError(err)
		}
		if !row.LogoObjectKey.Valid {
			return ErrLogoNotFound
		}
		return nil
	})
	return row, err
}
func (s *Service) GetLogo(ctx context.Context, p authz.Principal, ifNoneMatch string) (LogoResult, error) {
	row, err := s.readLogoTenant(ctx, p)
	if err != nil {
		return LogoResult{}, err
	}
	for attempt := range 2 {
		key := row.LogoObjectKey.String
		id, err := uuid.Parse(strings.TrimSuffix(path.Base(key), path.Ext(key)))
		if err != nil {
			return LogoResult{}, fmt.Errorf("tenant: invalid logo object key: %w", err)
		}
		result := LogoResult{ETag: `"` + id.String() + `"`, ContentType: row.LogoContentType.String}
		result.NotModified = ifNoneMatch == result.ETag
		if result.NotModified {
			return result, nil
		}
		body, info, err := s.storage.Get(ctx, key)
		if err == nil {
			result.Body, result.Size = body, info.Size
			return result, nil
		}
		if !errors.Is(err, objectstore.ErrNotFound) {
			return LogoResult{}, fmt.Errorf("tenant: get logo: %w", err)
		}
		if attempt == 1 {
			return LogoResult{}, ErrLogoNotFound
		}
		row, err = s.readLogoTenant(ctx, p)
		if err != nil {
			return LogoResult{}, err
		}
		if row.LogoObjectKey.String == key {
			s.logger.ErrorContext(ctx, "referenced logo object missing", "event", "logo_object_missing", "tenant_id", p.TenantID.String(), "object_key", key)
			return LogoResult{}, ErrLogoNotFound
		}
	}
	return LogoResult{}, ErrLogoNotFound
}
