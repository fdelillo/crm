package authz

import (
	"context"
	"errors"
	"net/http"

	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
)

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

func Can(role Role, permission Permission) bool {
	for _, p := range Permissions(role) {
		if p == permission {
			return true
		}
	}
	return false
}
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

type Principal struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	SessionID uuid.UUID
	Role      Role
}
type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
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
