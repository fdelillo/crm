package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/fdelillo/crm/internal/platform/securetoken"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/time/rate"
)

var (
	ErrInvalidCredentials = errors.New("identity: invalid credentials")
	ErrAccountDisabled    = errors.New("identity: account disabled")
	loginFailedTotal      = expvar.NewInt("login_failed_total")
	loginLockedTotal      = expvar.NewInt("login_locked_total")
)

type loginSignal struct {
	event       string
	tenantID    uuid.UUID
	lockStarted bool
}

type LockedError struct{ RetryAfter time.Duration }

func (e *LockedError) Error() string { return "identity: login locked" }

type throttle struct {
	FailedCount   int32
	FirstFailedAt time.Time
	LastFailedAt  time.Time
	LockedUntil   *time.Time
}

// nextThrottle only changes state after an evaluated password attempt. A locked attempt never
// extends the deadline and must not verify a password.
func nextThrottle(state throttle, success bool, now time.Time) (throttle, bool, time.Duration) {
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		return state, true, state.LockedUntil.Sub(now)
	}
	if success {
		return throttle{}, false, 0
	}
	if state.LockedUntil != nil {
		state = throttle{}
	}
	if state.FailedCount == 0 {
		state.FirstFailedAt = now
	}
	state.FailedCount++
	state.LastFailedAt = now
	if state.FailedCount >= 5 {
		until := now.Add(15 * time.Minute)
		state.LockedUntil = &until
	}
	return state, false, 0
}

// WithAuthentication supplies the login dependencies; the key is copied and never logged.
func WithAuthentication(hasher password.Hasher, recorder audit.Recorder, hmacKey []byte, logger *slog.Logger) ServiceOption {
	if hasher == nil || recorder == nil || len(hmacKey) < 32 || logger == nil {
		panic("identity: invalid authentication dependencies")
	}
	key := append([]byte(nil), hmacKey...)
	return func(s *Service) {
		s.hasher, s.audit, s.hmacKey, s.logger = hasher, recorder, key, logger
		s.resetEmailLimiter = ratelimit.NewLimiter(rate.Every(time.Hour/3), 3, s.clock, time.Hour)
	}
}

func (s *Service) emailHMAC(email string) []byte {
	mac := hmac.New(sha256.New, s.hmacKey)
	_, _ = mac.Write([]byte(email))
	return mac.Sum(nil)
}

// Login serializes attempts for one normalized email by locking its HMAC row. Domain errors are
// returned after commit, so failed attempts and their audit entries are not rolled back.
func (s *Service) Login(ctx context.Context, email, plain string, meta RequestMeta) (SessionResult, error) {
	if s.hasher == nil {
		return SessionResult{}, errors.New("identity: authentication not configured")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	key := s.emailHMAC(email)
	var result SessionResult
	var outcome error
	var signal loginSignal
	err := s.runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		now := s.clock.Now()
		if err := q.EnsureLoginThrottle(ctx, store.EnsureLoginThrottleParams{EmailHmac: key, Now: now}); err != nil {
			return fmt.Errorf("identity: ensure throttle: %w", db.MapError(err))
		}
		row, err := q.LockLoginThrottle(ctx, key)
		if err != nil {
			return fmt.Errorf("identity: lock throttle: %w", db.MapError(err))
		}
		state := throttle{FailedCount: row.FailedCount, FirstFailedAt: row.FirstFailedAt,
			LastFailedAt: row.LastFailedAt, LockedUntil: row.LockedUntil}
		if _, locked, retry := nextThrottle(state, false, now); locked {
			outcome = &LockedError{RetryAfter: retry}
			signal.event = "login_locked"
			return nil
		}
		route, err := q.LookupUserByEmail(ctx, email)
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			s.hasher.VerifyDummy(ctx, plain)
			return s.failedLogin(ctx, tx, key, state, now, uuid.Nil, "", meta, &outcome, &signal)
		}
		if err != nil {
			return fmt.Errorf("identity: locate user: %w", db.MapError(err))
		}
		if err := tx.AsTenant(ctx, route.TenantID); err != nil {
			return err
		}
		user, err := q.GetLoginUser(ctx, store.GetLoginUserParams{TenantID: route.TenantID, UserID: route.ID})
		if err != nil {
			return fmt.Errorf("identity: read login user: %w", db.MapError(err))
		}
		valid := false
		needsRehash := false
		if user.PasswordHash.Valid && user.Status != "invited" {
			valid, needsRehash, err = s.hasher.Verify(ctx, plain, user.PasswordHash.String)
			if err != nil {
				return fmt.Errorf("identity: verify password: %w", err)
			}
		} else {
			s.hasher.VerifyDummy(ctx, plain)
		}
		if !valid || user.Status == "invited" {
			reason := "bad_password"
			if user.Status == "invited" {
				reason = "not_active"
			}
			signal.tenantID = route.TenantID
			return s.failedLogin(ctx, tx, key, state, now, route.ID, reason, meta, &outcome, &signal)
		}
		if user.Status == "disabled" {
			if err := s.recordLogin(ctx, tx, route.TenantID, route.ID, uuid.Nil, "auth.login_rejected_disabled", "", meta); err != nil {
				return err
			}
			outcome = ErrAccountDisabled
			signal = loginSignal{event: "login_disabled", tenantID: route.TenantID}
			return nil
		}
		if needsRehash {
			hash, err := s.hasher.Hash(ctx, plain)
			if err != nil {
				return fmt.Errorf("identity: rehash password: %w", err)
			}
			if err := q.UpdatePasswordHash(ctx, store.UpdatePasswordHashParams{TenantID: route.TenantID,
				UserID: route.ID, PasswordHash: pgtype.Text{String: hash, Valid: true}, Now: now}); err != nil {
				return fmt.Errorf("identity: update password hash: %w", db.MapError(err))
			}
		}
		result, err = s.CreateSession(ctx, tx, authz.Principal{TenantID: route.TenantID,
			UserID: route.ID, Role: authz.Role(user.Role)}, meta)
		if err != nil {
			return err
		}
		if err := s.recordLogin(ctx, tx, route.TenantID, route.ID, result.Principal.SessionID, "auth.login_succeeded", "", meta); err != nil {
			return err
		}
		if err := tx.AsSystem(ctx, db.RoleAuth); err != nil {
			return err
		}
		if err := q.DeleteLoginThrottle(ctx, key); err != nil {
			return fmt.Errorf("identity: clear throttle: %w", db.MapError(err))
		}
		return nil
	})
	if err != nil {
		return SessionResult{}, err
	}
	s.emitLoginSignal(ctx, signal, key, meta)
	return result, outcome
}

func (s *Service) failedLogin(ctx context.Context, tx db.Tx, key []byte, state throttle, now time.Time,
	userID uuid.UUID, reason string, meta RequestMeta, outcome *error, signal *loginSignal) error {
	state, _, _ = nextThrottle(state, false, now)
	if userID != uuid.Nil {
		if err := s.recordLogin(ctx, tx, txTenantID(tx), userID, uuid.Nil, "auth.login_failed", reason, meta); err != nil {
			return err
		}
		if state.LockedUntil != nil {
			if err := s.recordLogin(ctx, tx, txTenantID(tx), userID, uuid.Nil, "auth.login_locked", "", meta); err != nil {
				return err
			}
		}
		if err := tx.AsSystem(ctx, db.RoleAuth); err != nil {
			return err
		}
	}
	if err := store.New(tx).SaveLoginThrottle(ctx, store.SaveLoginThrottleParams{EmailHmac: key,
		FailedCount: state.FailedCount, FirstFailedAt: state.FirstFailedAt, LastFailedAt: state.LastFailedAt,
		LockedUntil: state.LockedUntil}); err != nil {
		return fmt.Errorf("identity: save throttle: %w", db.MapError(err))
	}
	*outcome = ErrInvalidCredentials
	signal.event = "login_failed"
	signal.lockStarted = state.LockedUntil != nil
	return nil
}

// emitLoginSignal runs only after commit: rolled-back attempts must not increment metrics or
// claim a security event. The HMAC is hex-encoded for logs; plaintext email never leaves Login.
func (s *Service) emitLoginSignal(ctx context.Context, signal loginSignal, key []byte, meta RequestMeta) {
	if signal.event == "" {
		return
	}
	switch signal.event {
	case "login_failed":
		loginFailedTotal.Add(1) // evaluated failure, including the fifth and unknown emails
	case "login_locked":
		loginLockedTotal.Add(1) // a rejected attempt while the 15-minute lock is active
	}
	ip := ""
	if meta.IP.IsValid() {
		ip = meta.IP.Unmap().String()
	}
	fields := []any{"security_event", signal.event, "email_hmac", hex.EncodeToString(key), "ip", ip}
	if signal.tenantID != uuid.Nil {
		fields = append(fields, "tenant_id", signal.tenantID.String())
	}
	if meta.RequestID != "" {
		fields = append(fields, "request_id", meta.RequestID)
	}
	if signal.lockStarted {
		fields = append(fields, "lock_started", true)
	}
	s.logger.WarnContext(ctx, "login security event", fields...)
	if signal.lockStarted {
		// The fifth bad password starts the lock but still returns 401. Log the transition;
		// login_locked_total counts only attempts actually rejected with 429.
		lockedFields := append([]any(nil), fields...)
		lockedFields[1] = "login_locked"
		s.logger.WarnContext(ctx, "login lock started", lockedFields...)
	}
}

func txTenantID(tx db.Tx) uuid.UUID { id, _ := tx.TenantID(); return id }

func (s *Service) recordLogin(ctx context.Context, tx db.Tx, tenantID, userID, sessionID uuid.UUID, action, reason string, meta RequestMeta) error {
	entry := audit.Entry{TenantID: tenantID, Action: action, IP: meta.IP, UserAgent: meta.UserAgent}
	if action == "auth.login_succeeded" {
		entry.ActorUserID = &userID
		entry.TargetType, entry.TargetID = "session", &sessionID
	} else {
		entry.TargetType, entry.TargetID = "user", &userID
		if reason != "" {
			entry.Data = map[string]any{"reason": reason}
		}
	}
	return s.audit.Record(ctx, tx, entry)
}

// Logout revokes only the presented session. Missing, expired and previously revoked tokens are
// idempotent. The cookie is cleared by the HTTP handler even when there is no valid session.
func (s *Service) Logout(ctx context.Context, raw string, meta RequestMeta) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return s.runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		route, err := q.LookupSessionByHash(ctx, securetoken.Hash(raw))
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("identity: locate logout session: %w", db.MapError(err))
		}
		if err := tx.AsTenant(ctx, route.TenantID); err != nil {
			return err
		}
		now := s.clock.Now()
		userID, err := q.RevokeSession(ctx, store.RevokeSessionParams{TenantID: route.TenantID,
			SessionID: route.ID, Now: &now, IdleCutoff: now.Add(-s.idle)})
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("identity: revoke session: %w", db.MapError(err))
		}
		return s.audit.Record(ctx, tx, audit.Entry{TenantID: route.TenantID, ActorUserID: &userID,
			Action: "auth.logout", TargetType: "session", TargetID: &route.ID,
			IP: meta.IP, UserAgent: meta.UserAgent})
	})
}
