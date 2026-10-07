// Package e2e extends fixture with service-created companies for HTTP isolation tests.
// It is separate from fixture's low-level helpers so same-package domain tests can still use
// those helpers without an import cycle through the real services.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/objectstore"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Password = "Fixture-password-892!"
const NewPassword = "Replacement-password-736!"

type User struct {
	ID                  uuid.UUID
	Email, Name, Status string
	Principal           authz.Principal
	Cookie              *http.Cookie
}

type Company struct {
	ID                              uuid.UUID
	Users                           map[string]User
	Details                         tenant.Tenant
	Logo                            []byte
	ETag, LogoKey                   string
	Reset, Verification, Invitation string
}

// Scenario owns no connections: construction and all assertions use sequential transactions.
type Scenario struct {
	A, B      Company
	Users     *identity.Service
	Companies *tenant.Service
	Runner    db.TxRunner
	Clock     clock.Clock
	Logger    *slog.Logger
	Storage   *Storage
}

// New uses Register, Invite, AcceptInvitation, Deactivate, Update, SetLogo and token services.
// Inactive-user cookies are deliberately created with CreateSession, not Login: authentication
// must reject them on use even when a valid session row exists (INV-11).
func New(t testing.TB, pool *pgxpool.Pool) *Scenario {
	t.Helper()
	c := clock.Real{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := db.NewTxRunner(pool, db.WithLogger(logger))
	h := password.NewHasher(2)
	recorder := audit.NewRecorder()
	storage := &Storage{objects: map[string][]byte{}}
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour,
		identity.WithAuthentication(h, recorder, []byte("0123456789abcdef0123456789abcdef"), logger))
	companies := tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, h, recorder, logger, tenant.WithObjectStorage(storage))
	f := &Scenario{Users: users, Companies: companies, Runner: runner, Clock: c, Logger: logger, Storage: storage}
	f.A = f.company(t, "A", color.NRGBA{R: 241, A: 255}, "30123456781", "America/Argentina/Buenos_Aires")
	f.B = f.company(t, "B", color.NRGBA{B: 239, A: 255}, "20123456786", "America/Montevideo")
	return f
}

func require(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func cookie(raw string) *http.Cookie {
	return &http.Cookie{Name: identity.SessionCookieName, Value: raw, Path: "/", Secure: true, HttpOnly: true}
}

func (f *Scenario) company(t testing.TB, label string, tint color.NRGBA, taxID, timezone string) Company {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ToLower(label) + "-" + uuid.NewString()
	currency, template := "ARS", "generic"
	if label == "B" {
		currency, template = "USD", "aluminum_carpentry"
	}
	reg, err := f.Companies.Register(ctx, tenant.Signup{Name: "Admin " + suffix, Email: "admin-" + suffix + "@example.test",
		Password: Password, CompanyName: "Company " + suffix, BaseCurrency: currency, IndustryTemplateCode: template, Timezone: timezone}, identity.RequestMeta{})
	require(t, err)
	p := reg.Session.Principal
	c := Company{ID: p.TenantID, Users: map[string]User{"admin": {ID: p.UserID, Email: reg.User.Email, Name: reg.User.Name, Status: "active", Principal: p, Cookie: cookie(reg.Session.RawToken)}}}
	for _, key := range []string{"operator", "invited", "disabled_password", "disabled_no_password"} {
		email := key + "-" + suffix + "@example.test"
		u, _, err := f.Users.Invite(ctx, p, email, authz.RoleOperator, identity.RequestMeta{})
		require(t, err)
		member := User{ID: u.ID, Email: email, Status: "invited", Principal: authz.Principal{TenantID: c.ID, UserID: u.ID, Role: authz.RoleOperator}}
		if key == "operator" || key == "disabled_password" {
			member.Name = key + " " + suffix
			session, err := f.Users.AcceptInvitation(ctx, f.Token(t, c.ID, email, "invitation"), member.Name, Password, identity.RequestMeta{})
			require(t, err)
			member.Principal, member.Cookie, member.Status = session.Principal, cookie(session.RawToken), "active"
		} else {
			require(t, f.Runner.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
				session, err := f.Users.CreateSession(ctx, tx, member.Principal, identity.RequestMeta{})
				member.Cookie, member.Principal = cookie(session.RawToken), session.Principal
				return err
			}))
		}
		if key == "disabled_password" || key == "disabled_no_password" {
			_, err = f.Users.Deactivate(ctx, p, member.ID, identity.RequestMeta{})
			require(t, err)
			member.Status = "disabled"
		}
		c.Users[key] = member
	}
	patch := tenant.Update{}
	for key, value := range map[string]string{"legal_name": "Legal " + suffix, "tax_id": taxID, "address": "Address " + suffix, "phone": "Phone " + suffix, "email": "contact-" + suffix + "@example.test"} {
		v := value
		patch[key] = &v
	}
	c.Details, err = f.Companies.Update(ctx, p, patch, identity.RequestMeta{})
	require(t, err)
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, tint)
	var data bytes.Buffer
	require(t, png.Encode(&data, img))
	c.Logo = data.Bytes()
	c.Details, err = f.Companies.SetLogo(ctx, p, c.Logo, identity.RequestMeta{})
	require(t, err)
	logo, err := f.Companies.GetLogo(ctx, p, "")
	require(t, err)
	require(t, logo.Body.Close())
	c.ETag = logo.ETag
	require(t, f.Runner.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT logo_object_key FROM app.tenants WHERE id=$1`, c.ID).Scan(&c.LogoKey)
	}))
	require(t, f.Users.RequestPasswordReset(ctx, c.Users["operator"].Email, identity.RequestMeta{}))
	c.Reset = f.Token(t, c.ID, c.Users["operator"].Email, "password_reset")
	c.Verification = f.Token(t, c.ID, c.Users["admin"].Email, "email_verification")
	c.Invitation = f.Token(t, c.ID, c.Users["invited"].Email, "invitation")
	return c
}

// Token reads the real transactional outbox as its own company, including reissued tokens.
func (f *Scenario) Token(t testing.TB, tenantID uuid.UUID, email, purpose string) string {
	t.Helper()
	var token string
	require(t, f.Runner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT payload->>'token' FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template=$3 ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, email, purpose).Scan(&token)
	}))
	return token
}

// Storage is the in-memory implementation of the existing object storage port.
// Database writes, keys and HTTP streaming still use the real tenant service.
type Storage struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *Storage) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = data
	return nil
}
func (s *Storage) Get(_ context.Context, key string) (io.ReadCloser, objectstore.ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return nil, objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), objectstore.ObjectInfo{Size: int64(len(data))}, nil
}
func (s *Storage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

// Snapshot includes every persisted field, including audit, sessions, tokens and outbox.
// All reads are under InTenantTx(id); an owner/superuser never proves isolation.
func (f *Scenario) Snapshot(t testing.TB, id uuid.UUID) map[string]string {
	t.Helper()
	result := map[string]string{}
	require(t, f.Runner.InTenantTx(context.Background(), id, func(ctx context.Context, tx db.Tx) error {
		for _, table := range []string{"tenants", "users", "sessions", "user_tokens", "outbox_messages", "audit_log"} {
			col := "tenant_id"
			if table == "tenants" {
				col = "id"
			}
			var value string
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY id), '[]'::jsonb)::text FROM app.%s r WHERE %s=$1`, table, col), id).Scan(&value); err != nil {
				return err
			}
			result[table] = value
		}
		return nil
	}))

	f.Storage.mu.Lock()
	objects := map[string][]byte{}
	for key, data := range f.Storage.objects {
		if strings.HasPrefix(key, "tenants/"+id.String()+"/") {
			objects[key] = data
		}
	}
	encoded, err := json.Marshal(objects)
	f.Storage.mu.Unlock()
	require(t, err)
	result["objects"] = string(encoded)
	return result
}
