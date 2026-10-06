package identity

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/platform/securetoken"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// Invite serializes administrator writes through INV-10, then locks an existing
// invitee before rechecking its state (acceptance does not take the tenant lock).
func (s *Service) Invite(ctx context.Context, p authz.Principal, email string, role authz.Role, meta RequestMeta) (User, bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, parseErr := mail.ParseAddress(email)
	fields := map[string]string{}
	if parseErr != nil || address.Address != email || !strings.Contains(email, "@") || strings.ContainsRune(email, '\x00') {
		fields["email"] = "invalid"
	}
	if !validUserRole(role) {
		fields["role"] = "invalid"
	}
	if len(fields) > 0 {
		return User{}, false, &UserValidationError{Fields: fields}
	}
	var result User
	var reissued bool
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		if _, err := q.LockUsersTenant(ctx, p.TenantID); err != nil {
			return db.MapError(err)
		}
		existing, err := q.FindInvitedEmail(ctx, store.FindInvitedEmailParams{TenantID: p.TenantID, Email: email})
		now := s.clock.Now()
		var id uuid.UUID
		switch {
		case errors.Is(db.MapError(err), db.ErrNotFound):
			id, err = q.InsertInvitedUser(ctx, store.InsertInvitedUserParams{TenantID: p.TenantID, Email: email, Role: string(role), Now: now})
			if err != nil {
				return db.MapError(err)
			}
			company, err := q.GetTokenFlowTenantName(ctx, p.TenantID)
			if err != nil {
				return db.MapError(err)
			}
			if err := s.issueToken(ctx, tx, p.TenantID, id, email, "invitation", 7*24*time.Hour, map[string]string{"company_name": company, "role": string(role)}, p.UserID); err != nil {
				return err
			}
			if err := s.recordUser(ctx, tx, p, id, "user.invited", map[string]any{"role": string(role)}, meta); err != nil {
				return err
			}
		case err != nil:
			return db.MapError(err)
		default:
			id = existing.ID
			user, err := q.LockManagedUser(ctx, store.LockManagedUserParams{TenantID: p.TenantID, UserID: id})
			if err != nil {
				return db.MapError(err)
			}
			if _, err := nextUserStatus(user.Status, reinvite, user.PasswordHash.Valid); err != nil {
				return ErrEmailTaken
			}
			if err := s.setUserRole(ctx, tx, p, user, role, meta); err != nil {
				return err
			}
			if err := s.reissueInvitation(ctx, tx, p.TenantID, id, email, string(role), now, "admin", meta, p.UserID); err != nil {
				return err
			}
			reissued = true
		}
		row, err := q.GetManagedUser(ctx, store.GetManagedUserParams{TenantID: p.TenantID, UserID: id})
		result = managedUser(row)
		return db.MapError(err)
	})
	var constraint *db.ConstraintError
	if errors.As(err, &constraint) && constraint.Constraint == "users_email_key" {
		err = ErrEmailTaken
	}
	return result, reissued, err
}

func validUserRole(role authz.Role) bool {
	return role == authz.RoleAdmin || role == authz.RoleOperator
}
func (s *Service) recordUser(ctx context.Context, tx db.Tx, p authz.Principal, id uuid.UUID, action string, data map[string]any, meta RequestMeta) error {
	return s.audit.Record(ctx, tx, audit.Entry{TenantID: p.TenantID, ActorUserID: &p.UserID, Action: action, TargetType: "user", TargetID: &id, Data: data, IP: meta.IP, UserAgent: meta.UserAgent})
}
func (s *Service) protectLastAdmin(ctx context.Context, q *store.Queries, p authz.Principal, user store.LockManagedUserRow) error {
	if user.Status != "active" || user.Role != "admin" {
		return nil
	}
	n, err := q.CountActiveAdmins(ctx, p.TenantID)
	if err != nil {
		return db.MapError(err)
	}
	if n <= 1 {
		return ErrLastAdmin
	}
	return nil
}
func (s *Service) setUserRole(ctx context.Context, tx db.Tx, p authz.Principal, user store.LockManagedUserRow, role authz.Role, meta RequestMeta) error {
	if user.Role == string(role) {
		return nil
	}
	q := store.New(tx)
	if role == authz.RoleOperator {
		if err := s.protectLastAdmin(ctx, q, p, user); err != nil {
			return err
		}
	}
	if err := q.SetManagedUserRole(ctx, store.SetManagedUserRoleParams{TenantID: p.TenantID, UserID: user.ID, Role: string(role), Now: s.clock.Now()}); err != nil {
		return db.MapError(err)
	}
	return s.recordUser(ctx, tx, p, user.ID, "user.role_changed", map[string]any{"from": user.Role, "to": string(role), "status": user.Status}, meta)
}

// withManagedUser always acquires the tenant before the target. Every successful
// mutation, its outbox messages and its audit entries share this transaction.
func (s *Service) withManagedUser(ctx context.Context, p authz.Principal, id uuid.UUID, action userAction, fn func(context.Context, db.Tx, store.LockManagedUserRow, string) error) (User, error) {
	var result User
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		if _, err := q.LockUsersTenant(ctx, p.TenantID); err != nil {
			return db.MapError(err)
		}
		user, err := q.LockManagedUser(ctx, store.LockManagedUserParams{TenantID: p.TenantID, UserID: id})
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrUserNotFound
		}
		if err != nil {
			return db.MapError(err)
		}
		next, err := nextUserStatus(user.Status, action, user.PasswordHash.Valid)
		if err != nil {
			return err
		}
		if err := fn(ctx, tx, user, next); err != nil {
			return err
		}
		row, err := q.GetManagedUser(ctx, store.GetManagedUserParams{TenantID: p.TenantID, UserID: id})
		result = managedUser(row)
		return db.MapError(err)
	})
	return result, err
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
	if !validUserRole(role) {
		return User{}, &UserValidationError{Fields: map[string]string{"role": "invalid"}}
	}
	return s.withManagedUser(ctx, p, userID, changeRole, func(ctx context.Context, tx db.Tx, user store.LockManagedUserRow, _ string) error {
		return s.setUserRole(ctx, tx, p, user, role, meta)
	})
}
func (s *Service) Deactivate(ctx context.Context, p authz.Principal, userID uuid.UUID, meta RequestMeta) (User, error) {
	return s.withManagedUser(ctx, p, userID, deactivate, func(ctx context.Context, tx db.Tx, user store.LockManagedUserRow, next string) error {
		q := store.New(tx)
		if err := s.protectLastAdmin(ctx, q, p, user); err != nil {
			return err
		}
		now := s.clock.Now()
		if err := q.SetManagedUserStatus(ctx, store.SetManagedUserStatusParams{TenantID: p.TenantID, UserID: userID, Status: next, Now: now}); err != nil {
			return db.MapError(err)
		}
		count, err := q.RevokeDisabledUserSessions(ctx, store.RevokeDisabledUserSessionsParams{TenantID: p.TenantID, UserID: userID, Now: &now})
		if err != nil {
			return db.MapError(err)
		}
		if err := q.RevokeDisabledUserTokens(ctx, store.RevokeDisabledUserTokensParams{TenantID: p.TenantID, UserID: userID, Now: &now}); err != nil {
			return db.MapError(err)
		}
		return s.recordUser(ctx, tx, p, userID, "user.deactivated", map[string]any{"sessions_revoked": count}, meta)
	})
}
func (s *Service) Reactivate(ctx context.Context, p authz.Principal, userID uuid.UUID, meta RequestMeta) (User, error) {
	return s.withManagedUser(ctx, p, userID, reactivate, func(ctx context.Context, tx db.Tx, user store.LockManagedUserRow, next string) error {
		now := s.clock.Now()
		q := store.New(tx)
		if err := q.SetManagedUserStatus(ctx, store.SetManagedUserStatusParams{TenantID: p.TenantID, UserID: userID, Status: next, Now: now}); err != nil {
			return db.MapError(err)
		}
		if next == "invited" {
			if err := s.reissueInvitation(ctx, tx, p.TenantID, userID, user.Email, user.Role, now, "reactivation", meta, p.UserID); err != nil {
				return err
			}
		}
		return s.recordUser(ctx, tx, p, userID, "user.reactivated", map[string]any{"to_status": next}, meta)
	})
}
func (s *Service) PreviewInvitation(ctx context.Context, raw string) (InvitationPreview, error) {
	var preview InvitationPreview
	err := s.withInvitation(ctx, raw, func(ctx context.Context, tx db.Tx, route store.LookupTokenByHashRow, id uuid.UUID, user store.GetTokenFlowUserRow, token store.GetTokenForUpdateRow) error {
		name, err := store.New(tx).GetTokenFlowTenantName(ctx, route.TenantID)
		if err != nil {
			return db.MapError(err)
		}
		preview = InvitationPreview{TenantName: name, Email: user.Email, Role: authz.Role(user.Role), ExpiresAt: token.ExpiresAt}
		return nil
	})
	return preview, err
}

// Acceptance only increases the active administrator count; DD-40 permits it to
// acquire user -> token without the tenant lock used for administrator writes.
func (s *Service) withInvitation(ctx context.Context, raw string, fn func(context.Context, db.Tx, store.LookupTokenByHashRow, uuid.UUID, store.GetTokenFlowUserRow, store.GetTokenForUpdateRow) error) error {
	if raw == "" {
		return ErrTokenInvalid
	}
	return s.runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
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
		id, err := q.GetTokenUser(ctx, store.GetTokenUserParams{TenantID: route.TenantID, TokenID: route.ID})
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrTokenInvalid
		}
		if err != nil {
			return db.MapError(err)
		}
		user, err := q.GetTokenFlowUser(ctx, store.GetTokenFlowUserParams{TenantID: route.TenantID, UserID: id})
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
		if token.UserID != id || token.Purpose != "invitation" || token.UsedAt != nil || token.RevokedAt != nil || !s.clock.Now().Before(token.ExpiresAt) || user.Status != "invited" {
			return ErrTokenInvalid
		}
		return fn(ctx, tx, route, id, user, token)
	})
}
func (s *Service) AcceptInvitation(ctx context.Context, raw, name, plain string, meta RequestMeta) (SessionResult, error) {
	var session SessionResult
	err := s.withInvitation(ctx, raw, func(ctx context.Context, tx db.Tx, route store.LookupTokenByHashRow, id uuid.UUID, user store.GetTokenFlowUserRow, _ store.GetTokenForUpdateRow) error {
		name = strings.TrimSpace(name)
		if name == "" || utf8.RuneCountInString(name) > 120 || strings.ContainsRune(name, '\x00') {
			return &UserValidationError{Fields: map[string]string{"name": "invalid"}}
		}
		if code := password.Validate(plain, user.Email); code != "" {
			return &PasswordValidationError{Code: code}
		}
		hash, err := s.hasher.Hash(ctx, plain)
		if err != nil {
			return err
		}
		q := store.New(tx)
		now := s.clock.Now()
		used, err := q.UseUserToken(ctx, store.UseUserTokenParams{TenantID: route.TenantID, TokenID: route.ID, Purpose: "invitation", Now: &now})
		if err != nil {
			return db.MapError(err)
		}
		if used != 1 {
			return ErrTokenInvalid
		}
		if err := q.ActivateInvitedUser(ctx, store.ActivateInvitedUserParams{TenantID: route.TenantID, UserID: id, Name: pgtype.Text{String: name, Valid: true}, PasswordHash: pgtype.Text{String: hash, Valid: true}, Now: &now}); err != nil {
			return db.MapError(err)
		}
		principal := authz.Principal{TenantID: route.TenantID, UserID: id, Role: authz.Role(user.Role)}
		session, err = s.CreateSession(ctx, tx, principal, meta)
		if err != nil {
			return err
		}
		return s.recordUser(ctx, tx, principal, id, "user.invitation_accepted", nil, meta)
	})
	return session, err
}
