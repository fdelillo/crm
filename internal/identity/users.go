package identity

import (
	"context"
	"errors"
	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/securetoken"
	"github.com/google/uuid"
	"time"
)

type User struct {
	ID                  uuid.UUID  `json:"id"`
	Email               string     `json:"email"`
	Name                *string    `json:"name"`
	Role                authz.Role `json:"role"`
	Status              string     `json:"status"`
	EmailVerified       bool       `json:"email_verified"`
	CreatedAt           time.Time  `json:"created_at"`
	InvitationExpiresAt *time.Time `json:"invitation_expires_at"`
}

type InvitationPreview struct {
	TenantName string     `json:"tenant_name"`
	Email      string     `json:"email"`
	Role       authz.Role `json:"role"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

type UserValidationError struct{ Fields map[string]string }

func (e *UserValidationError) Error() string { return "identity: invalid user input" }

var errUsersNotImplemented = errors.New("identity: users not implemented")

func (s *Service) Invite(ctx context.Context, p authz.Principal, email string, role authz.Role, meta RequestMeta) (User, bool, error) {
	return User{}, false, errUsersNotImplemented
}
func (s *Service) ListUsers(ctx context.Context, p authz.Principal) ([]User, error) {
	users := make([]User, 0)
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		rows, err := store.New(tx).ListManagedUsers(ctx, p.TenantID)
		if err != nil {
			return db.MapError(err)
		}
		for _, row := range rows {
			users = append(users, managedUser(store.GetManagedUserRow(row)))
		}
		return nil
	})
	return users, err
}

func managedUser(row store.GetManagedUserRow) User {
	u := User{ID: row.ID, Email: row.Email, Role: authz.Role(row.Role), Status: row.Status,
		EmailVerified: row.EmailVerifiedAt != nil, CreatedAt: row.CreatedAt, InvitationExpiresAt: row.InvitationExpiresAt}
	if row.Name.Valid {
		name := row.Name.String
		u.Name = &name
	}
	return u
}
func (s *Service) ChangeRole(ctx context.Context, p authz.Principal, userID uuid.UUID, role authz.Role, meta RequestMeta) (User, error) {
	return User{}, errUsersNotImplemented
}
func (s *Service) Deactivate(ctx context.Context, p authz.Principal, userID uuid.UUID, meta RequestMeta) (User, error) {
	return User{}, errUsersNotImplemented
}
func (s *Service) Reactivate(ctx context.Context, p authz.Principal, userID uuid.UUID, meta RequestMeta) (User, error) {
	return User{}, errUsersNotImplemented
}
func (s *Service) PreviewInvitation(ctx context.Context, raw string) (InvitationPreview, error) {
	if raw == "" {
		return InvitationPreview{}, ErrTokenInvalid
	}
	var preview InvitationPreview
	err := s.runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		route, err := q.LookupTokenByHash(ctx, securetoken.Hash(raw))
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrTokenInvalid
		}
		if err != nil {
			return db.MapError(err)
		}
		if route.Purpose != "invitation" {
			return ErrTokenInvalid
		}
		if err := tx.AsTenant(ctx, route.TenantID); err != nil {
			return err
		}
		userID, err := q.GetTokenUser(ctx, store.GetTokenUserParams{TenantID: route.TenantID, TokenID: route.ID})
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrTokenInvalid
		}
		if err != nil {
			return db.MapError(err)
		}
		// Read-only preview still serializes with reissuance using user -> token.
		user, err := q.GetTokenFlowUser(ctx, store.GetTokenFlowUserParams{TenantID: route.TenantID, UserID: userID})
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrTokenInvalid
		}
		if err != nil {
			return db.MapError(err)
		}
		token, err := q.GetTokenForUpdate(ctx, store.GetTokenForUpdateParams{TenantID: route.TenantID, TokenID: route.ID})
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrTokenInvalid
		}
		if err != nil {
			return db.MapError(err)
		}
		if token.UserID != userID || token.Purpose != "invitation" || token.UsedAt != nil || token.RevokedAt != nil || !s.clock.Now().Before(token.ExpiresAt) || user.Status != "invited" {
			return ErrTokenInvalid
		}
		name, err := q.GetTokenFlowTenantName(ctx, route.TenantID)
		if err != nil {
			return db.MapError(err)
		}
		preview = InvitationPreview{TenantName: name, Email: user.Email, Role: authz.Role(user.Role), ExpiresAt: token.ExpiresAt}
		return nil
	})
	return preview, err
}
func (s *Service) AcceptInvitation(ctx context.Context, raw, name, plain string, meta RequestMeta) (SessionResult, error) {
	return SessionResult{}, errUsersNotImplemented
}
