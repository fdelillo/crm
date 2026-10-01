package authz

import (
	"context"
	"errors"
	"net/http"

	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/google/uuid"
)

// Role is a company user's role. The matrix below (Permissions) is the single place that maps a
// role to what it can do (ADR-013, FR-007): there is no per-endpoint role check anywhere else.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
)

// Permission is one action a role may or may not have (FR-007's static list).
type Permission string

const (
	CustomersManage           Permission = "customers.manage"
	ProjectsManage            Permission = "projects.manage"
	QuotesManage              Permission = "quotes.manage"
	CustomerReceiptsCreate    Permission = "customer_receipts.create"
	BalancesView              Permission = "balances.view"
	ProjectCostsView          Permission = "project_costs.view"
	SuppliersViewContact      Permission = "suppliers.view_contact"
	PartyFinancesManage       Permission = "party_finances.manage"
	MoneyAccountsManage       Permission = "money_accounts.manage"
	FinancialOperationsManage Permission = "financial_operations.manage"
	ChecksManage              Permission = "checks.manage"
	BankStatementsImport      Permission = "bank_statements.import"
	ReportsView               Permission = "reports.view"
	SettingsManage            Permission = "settings.manage"
	MovementsVoid             Permission = "movements.void"
)

// ErrForbidden is a sentinel for callers that need to distinguish "no permission" from other
// failures outside the HTTP path (RequirePermission writes the 403 itself and never returns this).
var ErrForbidden = errors.New("authz: forbidden")

var allPermissions = []Permission{
	CustomersManage, ProjectsManage, QuotesManage, CustomerReceiptsCreate, BalancesView,
	ProjectCostsView, SuppliersViewContact, PartyFinancesManage, MoneyAccountsManage,
	FinancialOperationsManage, ChecksManage, BankStatementsImport, ReportsView,
	SettingsManage, MovementsVoid,
}
var operatorPermissions = []Permission{
	CustomersManage, ProjectsManage, QuotesManage, CustomerReceiptsCreate, BalancesView, SuppliersViewContact,
}

// Can reports whether role has permission, per the static matrix of Permissions (ADR-013).
func Can(role Role, permission Permission) bool {
	for _, p := range Permissions(role) {
		if p == permission {
			return true
		}
	}
	return false
}

// Permissions returns role's permissions (a fresh copy); an unknown role has none.
func Permissions(role Role) []Permission {
	switch role {
	case RoleAdmin:
		return append([]Permission(nil), allPermissions...)
	case RoleOperator:
		return append([]Permission(nil), operatorPermissions...)
	default:
		return nil
	}
}

// Principal is the authenticated caller of a request: the session's company, user and role.
type Principal struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	SessionID uuid.UUID
	Role      Role
}
type principalKey struct{}

// WithPrincipal attaches p to ctx, for the session-resolving middleware that runs before
// RequirePermission.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the Principal set by WithPrincipal, if any.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// RequirePermission is route middleware: 401 with no Principal in context, 403 without
// permission, otherwise it calls next (ADR-013).
func RequirePermission(permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteProblem(w, r, httpx.CodeUnauthenticated)
				return
			}
			if !Can(principal.Role, permission) {
				httpx.WriteProblem(w, r, httpx.CodeForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
