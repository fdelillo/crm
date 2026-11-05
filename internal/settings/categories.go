package settings

import "github.com/google/uuid"

// CategoryType is "income" or "expense"; it never changes after creation (002/DD-8).
type CategoryType string

// SystemKey identifies a system category that other modules look up by key.
type SystemKey string

const (
	categoryIncome  CategoryType = "income"
	categoryExpense CategoryType = "expense"

	keyCustomerReceipts    SystemKey = "customer_receipts"
	keyPartnerContribution SystemKey = "partner_contributions"
	keySupplierPayments    SystemKey = "supplier_payments"
	keyEmployeePayments    SystemKey = "employee_payments"
	keyPartnerWithdrawals  SystemKey = "partner_withdrawals"
)

var (
	categoryTypes = []CategoryType{categoryIncome, categoryExpense}
	systemKeys    = []SystemKey{keyCustomerReceipts, keyPartnerContribution, keySupplierPayments, keyEmployeePayments, keyPartnerWithdrawals}
)

// Category is a movement category; SystemKey is nil for the ones the company created.
type Category struct {
	ID        uuid.UUID
	Name      string
	Type      CategoryType
	SystemKey *SystemKey // nil = category of the company
	Archived  bool
}

// CategoryFilter narrows ListCategories.
type CategoryFilter struct {
	Type            *CategoryType
	IncludeArchived bool
}

// systemCategory is a category every company has, whatever its template (002/DD-2, DD-8).
type systemCategory struct {
	Key  SystemKey
	Type CategoryType
	Name string
}

// systemCategories is the list seeded in every company (data-model.md section 4.1).
// T-002-B302 (002/DD-8, FR-004): five keys, each with its fixed type and initial name.
var systemCategories = []systemCategory{
	{keyCustomerReceipts, categoryIncome, "Cobros de clientes"},
	{keyPartnerContribution, categoryIncome, "Aportes de socios"},
	{keySupplierPayments, categoryExpense, "Pagos a proveedores"},
	{keyEmployeePayments, categoryExpense, "Pagos a empleados"},
	{keyPartnerWithdrawals, categoryExpense, "Retiros de socios"},
}

func validateCategoryType(t CategoryType) string { return validateEnum(t, categoryTypes) }
func validateSystemKey(k SystemKey) string       { return validateEnum(k, systemKeys) }

func normalizeCategoryName(name string) (string, string) {
	return normalizeRequiredText(name, maxCategoryNameLength)
}
