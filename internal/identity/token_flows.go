package identity

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/platform/securetoken"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrTokenInvalid = errors.New("identity: token invalid")

type PasswordValidationError struct{ Code string }

func (e *PasswordValidationError) Error() string { return "identity: invalid password" }

// RequestPasswordReset returns nil for every account state, and the handler answers an empty 202
// (INV-13). The work, and so the response time, still depends on the account state; that
// difference is not equalized (accepted risk, DD-39).
func (s *Service) RequestPasswordReset(ctx context.Context, email string, meta RequestMeta) error {
	if s.hasher == nil {
		return errors.New("identity: authentication not configured")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if allowed, _ := s.resetEmailLimiter.Allow(hex.EncodeToString(s.emailHMAC(email))); !allowed {
		return nil
	}
	return s.runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		route, err := q.LookupUserByEmail(ctx, email)
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("identity: locate reset user: %w", db.MapError(err))
		}
		if err := tx.AsTenant(ctx, route.TenantID); err != nil {
			return err
		}
		user, err := q.GetTokenFlowUser(ctx, store.GetTokenFlowUserParams{TenantID: route.TenantID, UserID: route.ID})
		if err != nil {
			return fmt.Errorf("identity: lock reset user: %w", db.MapError(err))
		}
		if user.Status == "disabled" {
			return nil
		}
		now := s.clock.Now()
		if user.Status == "invited" {
			return s.reissueInvitation(ctx, tx, route.TenantID, route.ID, user.Email, user.Role, now, "password_reset_request", meta)
		}
		if user.Status != "active" {
			return nil
		}
		if err := s.issueToken(ctx, tx, route.TenantID, route.ID, user.Email, "password_reset", time.Hour, nil); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Entry{TenantID: route.TenantID, Action: "auth.password_reset_requested", TargetType: "user", TargetID: &route.ID, IP: meta.IP, UserAgent: meta.UserAgent})
	})
}

// reissueInvitation is shared by self-service recovery and future administrator reinvitation.
func (s *Service) reissueInvitation(ctx context.Context, tx db.Tx, tenantID, userID uuid.UUID, email, role string, now time.Time, trigger string, meta RequestMeta) error {
	companyName, err := store.New(tx).GetTokenFlowTenantName(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("identity: invitation company: %w", db.MapError(err))
	}
	if err := s.issueToken(ctx, tx, tenantID, userID, email, "invitation", 7*24*time.Hour,
		map[string]string{"company_name": companyName, "role": role}); err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, audit.Entry{TenantID: tenantID, Action: "user.invitation_reissued", TargetType: "user", TargetID: &userID,
		Data: map[string]any{"role": role, "trigger": trigger}, IP: meta.IP, UserAgent: meta.UserAgent})
}

func (s *Service) issueToken(ctx context.Context, tx db.Tx, tenantID, userID uuid.UUID, email, purpose string, lifetime time.Duration, extra map[string]string) error {
	raw, hash, err := securetoken.New()
	if err != nil {
		return fmt.Errorf("identity: generate token: %w", err)
	}
	now := s.clock.Now()
	q := store.New(tx)
	if err := q.RevokeOpenUserTokens(ctx, store.RevokeOpenUserTokensParams{TenantID: tenantID, UserID: userID, Purpose: purpose, Now: &now}); err != nil {
		return fmt.Errorf("identity: revoke prior token: %w", db.MapError(err))
	}
	if err := q.InsertUserToken(ctx, store.InsertUserTokenParams{TenantID: tenantID, UserID: userID, Purpose: purpose,
		TokenHash: hash, CreatedAt: now, ExpiresAt: now.Add(lifetime)}); err != nil {
		return fmt.Errorf("identity: insert token: %w", db.MapError(err))
	}
	payload := map[string]string{"token": raw}
	for k, v := range extra {
		payload[k] = v
	}
	return s.outbox.Enqueue(ctx, tx, outbox.Message{TenantID: tenantID, Kind: "email", Template: purpose, Recipient: email, Payload: payload})
}

// withToken locates only routing columns as crm_auth, then locks the user and token under RLS.
// Password reset first locks the email's login throttle, preserving Login's lock order:
// throttle -> user -> token. Verification has no reason to touch the throttle.
func (s *Service) withToken(ctx context.Context, raw, purpose string, lockLoginThrottle bool,
	fn func(context.Context, db.Tx, store.LookupTokenByHashRow, store.GetTokenFlowUserRow, time.Time) error) error {
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
			return fmt.Errorf("identity: locate token: %w", db.MapError(err))
		}
		if route.Purpose != purpose {
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
			return fmt.Errorf("identity: read token user: %w", db.MapError(err))
		}
		if lockLoginThrottle {
			email, err := q.GetUserEmail(ctx, store.GetUserEmailParams{TenantID: route.TenantID, UserID: userID})
			if errors.Is(db.MapError(err), db.ErrNotFound) {
				return ErrTokenInvalid
			}
			if err != nil {
				return fmt.Errorf("identity: read reset email: %w", db.MapError(err))
			}
			if err := tx.AsSystem(ctx, db.RoleAuth); err != nil {
				return err
			}
			key := s.emailHMAC(email)
			if err := q.EnsureLoginThrottle(ctx, store.EnsureLoginThrottleParams{EmailHmac: key, Now: s.clock.Now()}); err != nil {
				return fmt.Errorf("identity: ensure reset throttle: %w", db.MapError(err))
			}
			if _, err := q.LockLoginThrottle(ctx, key); err != nil {
				return fmt.Errorf("identity: lock reset throttle: %w", db.MapError(err))
			}
			if err := tx.AsTenant(ctx, route.TenantID); err != nil {
				return err
			}
		}
		user, err := q.GetTokenFlowUser(ctx, store.GetTokenFlowUserParams{TenantID: route.TenantID, UserID: userID})
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrTokenInvalid
		}
		if err != nil {
			return fmt.Errorf("identity: lock token user: %w", db.MapError(err))
		}
		token, err := q.GetTokenForUpdate(ctx, store.GetTokenForUpdateParams{TenantID: route.TenantID, TokenID: route.ID})
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrTokenInvalid
		}
		if err != nil {
			return fmt.Errorf("identity: lock token: %w", db.MapError(err))
		}
		now := s.clock.Now()
		if token.UserID != userID || token.Purpose != purpose || token.UsedAt != nil || token.RevokedAt != nil || !now.Before(token.ExpiresAt) || user.Status != "active" {
			return ErrTokenInvalid
		}
		return fn(ctx, tx, route, user, now)
	})
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, raw, plain string, meta RequestMeta) error {
	if s.hasher == nil {
		return errors.New("identity: authentication not configured")
	}
	return s.withToken(ctx, raw, "password_reset", true, func(ctx context.Context, tx db.Tx, route store.LookupTokenByHashRow, user store.GetTokenFlowUserRow, now time.Time) error {
		if code := password.Validate(plain, user.Email); code != "" {
			return &PasswordValidationError{Code: code}
		}
		hash, err := s.hasher.Hash(ctx, plain)
		if err != nil {
			return fmt.Errorf("identity: hash reset password: %w", err)
		}
		q := store.New(tx)
		userID, err := q.GetTokenUser(ctx, store.GetTokenUserParams{TenantID: route.TenantID, TokenID: route.ID})
		if err != nil {
			return fmt.Errorf("identity: reset token user: %w", db.MapError(err))
		}
		used, err := q.UseUserToken(ctx, store.UseUserTokenParams{TenantID: route.TenantID, TokenID: route.ID, Purpose: "password_reset", Now: &now})
		if err != nil {
			return fmt.Errorf("identity: use reset token: %w", db.MapError(err))
		}
		if used != 1 {
			return ErrTokenInvalid
		}
		if err := q.SetResetPassword(ctx, store.SetResetPasswordParams{TenantID: route.TenantID, UserID: userID, PasswordHash: pgtype.Text{String: hash, Valid: true}, Now: now}); err != nil {
			return fmt.Errorf("identity: reset password: %w", db.MapError(err))
		}
		sessions, err := q.RevokeUserSessions(ctx, store.RevokeUserSessionsParams{TenantID: route.TenantID, UserID: userID, Now: &now})
		if err != nil {
			return fmt.Errorf("identity: revoke sessions: %w", db.MapError(err))
		}
		if err := s.audit.Record(ctx, tx, audit.Entry{TenantID: route.TenantID, ActorUserID: &userID, Action: "auth.password_reset_completed", TargetType: "user", TargetID: &userID,
			Data: map[string]any{"sessions_revoked": sessions}, IP: meta.IP, UserAgent: meta.UserAgent}); err != nil {
			return err
		}
		if err := tx.AsSystem(ctx, db.RoleAuth); err != nil {
			return err
		}
		if err := q.DeleteLoginThrottle(ctx, s.emailHMAC(user.Email)); err != nil {
			return fmt.Errorf("identity: clear login throttle: %w", db.MapError(err))
		}
		return nil
	})
}

func (s *Service) ConfirmEmailVerification(ctx context.Context, raw string, meta RequestMeta) error {
	if s.audit == nil {
		return errors.New("identity: authentication not configured")
	}
	return s.withToken(ctx, raw, "email_verification", false, func(ctx context.Context, tx db.Tx, route store.LookupTokenByHashRow, user store.GetTokenFlowUserRow, now time.Time) error {
		if user.EmailVerifiedAt != nil {
			return ErrTokenInvalid
		}
		q := store.New(tx)
		userID, err := q.GetTokenUser(ctx, store.GetTokenUserParams{TenantID: route.TenantID, TokenID: route.ID})
		if err != nil {
			return fmt.Errorf("identity: verification token user: %w", db.MapError(err))
		}
		used, err := q.UseUserToken(ctx, store.UseUserTokenParams{TenantID: route.TenantID, TokenID: route.ID, Purpose: "email_verification", Now: &now})
		if err != nil {
			return fmt.Errorf("identity: use verification token: %w", db.MapError(err))
		}
		if used != 1 {
			return ErrTokenInvalid
		}
		updated, err := q.MarkEmailVerified(ctx, store.MarkEmailVerifiedParams{TenantID: route.TenantID, UserID: userID, Now: &now})
		if err != nil {
			return fmt.Errorf("identity: verify email: %w", db.MapError(err))
		}
		if updated != 1 {
			return ErrTokenInvalid
		}
		return s.audit.Record(ctx, tx, audit.Entry{TenantID: route.TenantID, ActorUserID: &userID, Action: "auth.email_verified", TargetType: "user", TargetID: &userID, IP: meta.IP, UserAgent: meta.UserAgent})
	})
}

func (s *Service) ResendEmailVerification(ctx context.Context, p authz.Principal) error {
	return s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		user, err := store.New(tx).GetTokenFlowUser(ctx, store.GetTokenFlowUserParams{TenantID: p.TenantID, UserID: p.UserID})
		if err != nil {
			return fmt.Errorf("identity: verification user: %w", db.MapError(err))
		}
		if user.Status != "active" || user.EmailVerifiedAt != nil {
			return nil
		}
		return s.issueToken(ctx, tx, p.TenantID, p.UserID, user.Email, "email_verification", 48*time.Hour, nil)
	})
}
