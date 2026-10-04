package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/platform/securetoken"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrEmailTaken      = errors.New("identity: email taken")
	ErrUnauthenticated = errors.New("identity: unauthenticated")
)

type RequestMeta struct {
	IP        netip.Addr
	UserAgent string
	RequestID string
}

type NewAdmin struct {
	TenantID     uuid.UUID
	Name         string
	Email        string
	PasswordHash string
}

type SessionResult struct {
	RawToken  string
	ExpiresAt time.Time
	Principal authz.Principal
}

type CurrentUser struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Email         string     `json:"email"`
	Role          authz.Role `json:"role"`
	Status        string     `json:"status"`
	EmailVerified bool       `json:"email_verified"`
}

type Service struct {
	runner   db.TxRunner
	outbox   outbox.Enqueuer
	clock    clock.Clock
	idle     time.Duration
	absolute time.Duration
	hasher   password.Hasher
	audit    audit.Recorder
	hmacKey  []byte
	logger   *slog.Logger
}

type ServiceOption func(*Service)

func NewService(runner db.TxRunner, enqueuer outbox.Enqueuer, c clock.Clock, idle, absolute time.Duration, options ...ServiceOption) *Service {
	if runner == nil || enqueuer == nil || c == nil || idle <= 0 || absolute <= 0 || idle > absolute {
		panic("identity: invalid service dependencies")
	}
	s := &Service{runner: runner, outbox: enqueuer, clock: c, idle: idle, absolute: absolute}
	for _, option := range options {
		option(s)
	}
	return s
}

// CreateFirstAdmin is part of the caller's registration transaction.
func (s *Service) CreateFirstAdmin(ctx context.Context, tx db.Tx, in NewAdmin) (uuid.UUID, error) {
	id, err := store.New(tx).InsertFirstAdmin(ctx, store.InsertFirstAdminParams{
		TenantID: in.TenantID, Email: in.Email,
		Name: pgtype.Text{String: in.Name, Valid: true}, PasswordHash: pgtype.Text{String: in.PasswordHash, Valid: true},
	})
	if err != nil {
		mapped := db.MapError(err)
		var constraint *db.ConstraintError
		if errors.As(mapped, &constraint) && constraint.Constraint == "users_email_key" {
			return uuid.Nil, ErrEmailTaken
		}
		return uuid.Nil, fmt.Errorf("identity: create admin: %w", mapped)
	}
	return id, nil
}

// CreateSession stores only a hash of the token. Both writers of user_agent use NormalizeUserAgent.
func (s *Service) CreateSession(ctx context.Context, tx db.Tx, p authz.Principal, meta RequestMeta) (SessionResult, error) {
	raw, hash, err := securetoken.New()
	if err != nil {
		return SessionResult{}, fmt.Errorf("identity: generate session: %w", err)
	}
	now := s.clock.Now()
	expires := now.Add(s.absolute)
	var ip *netip.Addr
	if meta.IP.IsValid() {
		value := meta.IP.Unmap()
		ip = &value
	}
	ua := httpx.NormalizeUserAgent(meta.UserAgent)
	id, err := store.New(tx).InsertSession(ctx, store.InsertSessionParams{TenantID: p.TenantID,
		UserID: p.UserID, TokenHash: hash, CreatedAt: now, LastSeenAt: now, ExpiresAt: expires,
		Ip: ip, UserAgent: pgtype.Text{String: ua, Valid: ua != ""}})
	if err != nil {
		return SessionResult{}, fmt.Errorf("identity: create session: %w", db.MapError(err))
	}
	p.SessionID = id
	return SessionResult{RawToken: raw, ExpiresAt: expires, Principal: p}, nil
}

// IssueEmailVerification enqueues delivery in the same transaction; SMTP is never called here.
func (s *Service) IssueEmailVerification(ctx context.Context, tx db.Tx, tenantID, userID uuid.UUID) error {
	q := store.New(tx)
	email, err := q.GetUserEmail(ctx, store.GetUserEmailParams{TenantID: tenantID, UserID: userID})
	if err != nil {
		return fmt.Errorf("identity: get verification recipient: %w", db.MapError(err))
	}
	raw, hash, err := securetoken.New()
	if err != nil {
		return fmt.Errorf("identity: generate verification token: %w", err)
	}
	now := s.clock.Now()
	if err := q.InsertVerificationToken(ctx, store.InsertVerificationTokenParams{TenantID: tenantID,
		UserID: userID, TokenHash: hash, CreatedAt: now, ExpiresAt: now.Add(48 * time.Hour)}); err != nil {
		return fmt.Errorf("identity: store verification token: %w", db.MapError(err))
	}
	return s.outbox.Enqueue(ctx, tx, outbox.Message{TenantID: tenantID, Kind: "email",
		Template: "email_verification", Recipient: email, Payload: map[string]string{"token": raw}})
}

// ResolveSession first locates the tenant with crm_auth's routing columns and then checks all
// session and user state under that tenant's RLS role.
func (s *Service) ResolveSession(ctx context.Context, raw string) (authz.Principal, error) {
	if strings.TrimSpace(raw) == "" {
		return authz.Principal{}, ErrUnauthenticated
	}
	var principal authz.Principal
	err := s.runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		route, err := store.New(tx).LookupSessionByHash(ctx, securetoken.Hash(raw))
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrUnauthenticated
		}
		if err != nil {
			return fmt.Errorf("identity: locate session: %w", db.MapError(err))
		}
		if err := tx.AsTenant(ctx, route.TenantID); err != nil {
			return err
		}
		q := store.New(tx)
		row, err := q.GetSessionUser(ctx, store.GetSessionUserParams{TenantID: route.TenantID, SessionID: route.ID})
		if errors.Is(db.MapError(err), db.ErrNotFound) {
			return ErrUnauthenticated
		}
		if err != nil {
			return fmt.Errorf("identity: read session: %w", db.MapError(err))
		}
		now := s.clock.Now()
		if row.RevokedAt != nil || !now.Before(row.ExpiresAt) || !now.Before(row.LastSeenAt.Add(s.idle)) || row.Status != "active" {
			return ErrUnauthenticated
		}
		if now.Sub(row.LastSeenAt) > 5*time.Minute {
			if err := q.TouchSession(ctx, store.TouchSessionParams{LastSeenAt: now, TenantID: route.TenantID, SessionID: route.ID}); err != nil {
				return fmt.Errorf("identity: touch session: %w", db.MapError(err))
			}
		}
		principal = authz.Principal{TenantID: route.TenantID, UserID: row.UserID, SessionID: route.ID, Role: authz.Role(row.Role)}
		return nil
	})
	return principal, err
}

func (s *Service) Me(ctx context.Context, p authz.Principal) (CurrentUser, error) {
	var user CurrentUser
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		row, err := store.New(tx).GetSessionUser(ctx, store.GetSessionUserParams{TenantID: p.TenantID, SessionID: p.SessionID})
		if err != nil {
			return db.MapError(err)
		}
		user = CurrentUser{ID: row.UserID, Name: row.Name.String, Email: row.Email, Role: authz.Role(row.Role),
			Status: row.Status, EmailVerified: row.EmailVerifiedAt != nil}
		return nil
	})
	return user, err
}
