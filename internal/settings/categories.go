package settings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

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

// categoryNameConstraint is the partial unique index of active names per type (002/INV-6): the database, not the
// service, decides that a name is taken, so two concurrent creations need no lock.
const categoryNameConstraint = "categories_active_name_key"

// ListCategories returns the categories of the company: income first and, inside each type, the system ones and then
// the company's, by name. Archived ones only with f.IncludeArchived. Any role may read (002/DD-11).
// T-002-B502 (002/FR-003, DD-8)
func (s *Service) ListCategories(ctx context.Context, p authz.Principal, f CategoryFilter) ([]Category, error) {
	var filterType pgtype.Text
	if f.Type != nil {
		filterType = pgtype.Text{String: string(*f.Type), Valid: true}
	}
	var out []Category
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		rows, err := store.New(tx).ListCategories(ctx, store.ListCategoriesParams{TenantID: p.TenantID, Type: filterType, IncludeArchived: f.IncludeArchived})
		if err != nil {
			return fmt.Errorf("settings: list categories: %w", db.MapError(err))
		}
		out = make([]Category, len(rows))
		for i, r := range rows {
			out[i] = categoryFromRow(r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateCategory adds a category of the company. Invalid fields are FieldErrors (name, type); a name used by an
// active category of the same type, ignoring case, is ErrNameTaken.
// T-002-B502 (002/DD-8, INV-6)
func (s *Service) CreateCategory(ctx context.Context, p authz.Principal, name string, t CategoryType, meta RequestMeta) (Category, error) {
	if err := requireSettingsManage(p); err != nil {
		return Category{}, err
	}
	fields := FieldErrors{}
	name, code := normalizeCategoryName(name)
	setFieldCode(fields, "name", code)
	setFieldCode(fields, "type", validateCategoryType(t))
	if len(fields) > 0 {
		return Category{}, fields
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Category{}, fmt.Errorf("settings: new category id: %w", err)
	}
	created := Category{ID: id, Name: name, Type: t}
	err = s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		// No lock: the unique index serializes two creations of the same name.
		err := store.New(tx).InsertCategory(ctx, store.InsertCategoryParams{ID: id, TenantID: p.TenantID, Name: name, Type: string(t), Now: s.clock.Now()})
		if err != nil {
			return mapCategoryError(err)
		}
		return s.recordCategory(ctx, tx, p, meta, "category.created", id, map[string]any{"type": string(t)})
	})
	if err != nil {
		return Category{}, err
	}
	return created, nil
}

// RenameCategory changes the name of a category, a system one included, archived or not, and returns it. The same
// name, once normalized, writes and audits nothing. A category that is not the company's is ErrNotFound.
// T-002-B502 (002/DD-8, DD-15, INV-6, INV-13)
func (s *Service) RenameCategory(ctx context.Context, p authz.Principal, id uuid.UUID, name string, meta RequestMeta) (Category, error) {
	if err := requireSettingsManage(p); err != nil {
		return Category{}, err
	}
	name, code := normalizeCategoryName(name)
	if code != "" {
		return Category{}, FieldErrors{"name": code}
	}
	return s.writeCategory(ctx, p, id, func(q *store.Queries, tx db.Tx, c Category, now time.Time) (Category, error) {
		if c.Name == name {
			return c, nil
		}
		if err := q.UpdateCategoryName(ctx, store.UpdateCategoryNameParams{TenantID: p.TenantID, ID: id, Name: name, Now: now}); err != nil {
			return Category{}, mapCategoryError(err)
		}
		c.Name = name
		return c, s.recordCategory(ctx, tx, p, meta, "category.renamed", id, map[string]any{})
	})
}

// ArchiveCategory archives a category of the company. A system category is ErrSystemCategory; one already archived
// comes back as it is, with no write and no audit.
// T-002-B502 (002/DD-8, DD-15, INV-4, INV-13)
func (s *Service) ArchiveCategory(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) (Category, error) {
	if err := requireSettingsManage(p); err != nil {
		return Category{}, err
	}
	return s.writeCategory(ctx, p, id, func(q *store.Queries, tx db.Tx, c Category, now time.Time) (Category, error) {
		if c.SystemKey != nil {
			return Category{}, ErrSystemCategory
		}
		if c.Archived {
			return c, nil
		}
		if err := q.SetCategoryArchived(ctx, store.SetCategoryArchivedParams{TenantID: p.TenantID, ID: id, ArchivedAt: &now, Now: now}); err != nil {
			return Category{}, mapCategoryError(err)
		}
		c.Archived = true
		return c, s.recordCategory(ctx, tx, p, meta, "category.archived", id, map[string]any{})
	})
}

// RestoreCategory makes an archived category active again. If another active category of the same type already
// uses its name it is ErrNameTaken; one that is already active comes back as it is, with no write and no audit.
// T-002-B502 (002/DD-8, DD-15, INV-6, INV-13)
func (s *Service) RestoreCategory(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) (Category, error) {
	if err := requireSettingsManage(p); err != nil {
		return Category{}, err
	}
	return s.writeCategory(ctx, p, id, func(q *store.Queries, tx db.Tx, c Category, now time.Time) (Category, error) {
		if !c.Archived {
			return c, nil
		}
		if err := q.SetCategoryArchived(ctx, store.SetCategoryArchivedParams{TenantID: p.TenantID, ID: id, ArchivedAt: nil, Now: now}); err != nil {
			return Category{}, mapCategoryError(err)
		}
		c.Archived = false
		return c, s.recordCategory(ctx, tx, p, meta, "category.restored", id, map[string]any{})
	})
}

// writeCategory is the skeleton of the three writes on one category: lock its row, read the clock under the lock
// (a write that waited must not stamp an older updated_at over the change that held it), and let decide act on the
// category as found. A row that is not there, or is not the company's, is ErrNotFound; the settings row is not
// locked (002/DD-19).
func (s *Service) writeCategory(ctx context.Context, p authz.Principal, id uuid.UUID,
	decide func(q *store.Queries, tx db.Tx, current Category, now time.Time) (Category, error)) (Category, error) {
	var out Category
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		row, err := q.LockCategory(ctx, store.LockCategoryParams{TenantID: p.TenantID, ID: id})
		if err != nil {
			mapped := db.MapError(err)
			if errors.Is(mapped, db.ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("settings: lock category: %w", mapped)
		}
		now := s.clock.Now()
		out, err = decide(q, tx, categoryFromRow(row), now)
		return err
	})
	if err != nil {
		return Category{}, err
	}
	return out, nil
}

func (s *Service) recordCategory(ctx context.Context, tx db.Tx, p authz.Principal, meta RequestMeta, action string, id uuid.UUID, data map[string]any) error {
	return s.audit.Record(ctx, tx, audit.Entry{TenantID: p.TenantID, ActorUserID: &p.UserID, Action: action,
		TargetType: "category", TargetID: &id, Data: data, IP: meta.IP, UserAgent: meta.UserAgent})
}

func categoryFromRow(r store.AppCategory) Category {
	c := Category{ID: r.ID, Name: r.Name, Type: CategoryType(r.Type), Archived: r.ArchivedAt != nil}
	if r.SystemKey.Valid {
		key := SystemKey(r.SystemKey.String)
		c.SystemKey = &key
	}
	return c
}

// mapCategoryError turns the violation of the active-name index into ErrNameTaken; the rest is classified as usual.
func mapCategoryError(err error) error {
	mapped := db.MapError(err)
	var ce *db.ConstraintError
	if errors.As(mapped, &ce) && ce.Constraint == categoryNameConstraint {
		return ErrNameTaken
	}
	return fmt.Errorf("settings: write category: %w", mapped)
}
