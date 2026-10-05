package identity

import (
	"context"
	"errors"
	"github.com/fdelillo/crm/internal/authz"
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
	return nil, errUsersNotImplemented
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
	return InvitationPreview{}, errUsersNotImplemented
}
func (s *Service) AcceptInvitation(ctx context.Context, raw, name, plain string, meta RequestMeta) (SessionResult, error) {
	return SessionResult{}, errUsersNotImplemented
}
