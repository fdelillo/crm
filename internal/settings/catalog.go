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

// Unit is the unit of measure of a catalog item.
type Unit string

const (
	unitUnit        Unit = "unit"
	unitSquareMeter Unit = "square_meter"
	unitLinearMeter Unit = "linear_meter"
	unitHour        Unit = "hour"
	unitOther       Unit = "other"

	currencyARS = "ARS"
	currencyUSD = "USD"
)

var units = []Unit{unitUnit, unitSquareMeter, unitLinearMeter, unitHour, unitOther}

// Money is an amount in cents and its currency (ADR-029). AmountCents is in [0, MaxAmountCents].
type Money struct {
	AmountCents int64
	Currency    string // "ARS" | "USD"
}

// MaxAmountCents is 2^53 - 1: the largest amount every layer represents exactly (INV-7).
const MaxAmountCents int64 = 1<<53 - 1

// Patch tells an absent field (Set == false) from a present one, null included.
type Patch[T any] struct {
	Set   bool
	Value T
}

// CatalogItem is a catalog entry; ReferencePrice nil means "to be quoted".
type CatalogItem struct {
	ID             uuid.UUID
	Name           string
	Description    *string
	Unit           Unit
	UnitLabel      *string
	ReferencePrice *Money
	Archived       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewCatalogItem is the input of CreateCatalogItem.
type NewCatalogItem struct {
	Name           string
	Description    *string
	Unit           Unit
	UnitLabel      *string
	ReferencePrice *Money
}

// CatalogItemChanges is the PATCH of a catalog item.
type CatalogItemChanges struct {
	Name           Patch[string]  // null is a 400 when decoding (plan 9.1)
	Description    Patch[*string] // null or "" -> no description
	Unit           Patch[Unit]    // null is a 400 when decoding (plan 9.1)
	UnitLabel      Patch[*string]
	ReferencePrice Patch[*Money] // null -> no price
}

func validateUnit(u Unit) string { return validateEnum(u, units) }

// validateReferencePrice checks a price against INV-7. The keys are the contract paths.
// T-002-B302 (002/INV-7, DD-9): amount in [0, MaxAmountCents]; currency ARS or USD, "" is required.
func validateReferencePrice(m Money) FieldErrors {
	errs := FieldErrors{}
	if m.AmountCents < 0 || m.AmountCents > MaxAmountCents {
		errs["reference_price.amount_cents"] = codeInvalidValue
	}
	switch m.Currency {
	case currencyARS, currencyUSD:
	case "":
		errs["reference_price.currency"] = codeRequired
	default:
		errs["reference_price.currency"] = codeInvalidValue
	}
	return errs.orNil()
}

// normalizeUnitLabel applies the rule of the unit: other requires a label (at most 20 characters),
// any other unit discards it.
// T-002-B302 (002/DD-9, P-9): the error key is unit_label.
func normalizeUnitLabel(unit Unit, label *string) (*string, FieldErrors) {
	if unit != unitOther {
		return nil, nil
	}
	if label == nil {
		return nil, FieldErrors{"unit_label": codeRequired}
	}
	text, code := normalizeRequiredText(*label, maxUnitLabelLength)
	if code != "" {
		return nil, FieldErrors{"unit_label": code}
	}
	return &text, nil
}

// normalizeNewCatalogItem validates and normalizes a creation: it returns every field error at once.
// T-002-B302 (002/DD-9, FR-005): the keys are the contract paths of createCatalogItem.
func normalizeNewCatalogItem(in NewCatalogItem) (NewCatalogItem, FieldErrors) {
	errs := FieldErrors{}
	out := NewCatalogItem{Unit: in.Unit}

	name, code := normalizeRequiredText(in.Name, maxItemNameLength)
	errs.add("name", code)
	out.Name = name

	if in.Description != nil {
		description, code := normalizeOptionalText(*in.Description, maxDescriptionLength)
		errs.add("description", code)
		out.Description = description
	}

	if in.Unit == "" {
		errs.add("unit", codeRequired)
	} else if code := validateUnit(in.Unit); code != "" {
		errs.add("unit", code)
	} else {
		label, labelErrs := normalizeUnitLabel(in.Unit, in.UnitLabel)
		errs.merge(labelErrs)
		out.UnitLabel = label
	}

	if in.ReferencePrice != nil {
		errs.merge(validateReferencePrice(*in.ReferencePrice))
		price := *in.ReferencePrice
		out.ReferencePrice = &price
	}
	if len(errs) > 0 {
		return NewCatalogItem{}, errs
	}
	return out, nil
}

// ListCatalogItems returns the catalog of the company by name ignoring case, then id. Archived items only with
// includeArchived. Any role may read (002/DD-11).
// T-002-B602 (002/FR-005, DD-9, DD-16)
func (s *Service) ListCatalogItems(ctx context.Context, p authz.Principal, includeArchived bool) ([]CatalogItem, error) {
	var out []CatalogItem
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		rows, err := store.New(tx).ListCatalogItems(ctx, store.ListCatalogItemsParams{TenantID: p.TenantID, IncludeArchived: includeArchived})
		if err != nil {
			return fmt.Errorf("settings: list catalog items: %w", db.MapError(err))
		}
		out = make([]CatalogItem, len(rows))
		for i, r := range rows {
			out[i] = catalogItemFromRow(r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetCatalogItem returns one item, archived or not. An item that is not the company's is ErrNotFound.
// T-002-B602 (002/DD-9, DD-11, DD-19)
func (s *Service) GetCatalogItem(ctx context.Context, p authz.Principal, id uuid.UUID) (CatalogItem, error) {
	var out CatalogItem
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		row, err := store.New(tx).GetCatalogItem(ctx, store.GetCatalogItemParams{TenantID: p.TenantID, ID: id})
		if err != nil {
			return catalogReadError("get", err)
		}
		out = catalogItemFromRow(row)
		return nil
	})
	if err != nil {
		return CatalogItem{}, err
	}
	return out, nil
}

// CreateCatalogItem adds an item to the catalog. Invalid fields are FieldErrors with the contract paths. Names are
// not unique, so there is nothing to lock.
// T-002-B602 (002/FR-005, DD-9, P-9, INV-7)
func (s *Service) CreateCatalogItem(ctx context.Context, p authz.Principal, in NewCatalogItem, meta RequestMeta) (CatalogItem, error) {
	if err := requireSettingsManage(p); err != nil {
		return CatalogItem{}, err
	}
	in, fields := normalizeNewCatalogItem(in)
	if fields != nil {
		return CatalogItem{}, fields
	}
	id, err := uuid.NewV7()
	if err != nil {
		return CatalogItem{}, fmt.Errorf("settings: new catalog item id: %w", err)
	}
	var created CatalogItem
	err = s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		now := storedInstant(s.clock.Now())
		price := moneyColumns(in.ReferencePrice)
		err := store.New(tx).InsertCatalogItem(ctx, store.InsertCatalogItemParams{ID: id, TenantID: p.TenantID, Name: in.Name,
			Description: textColumn(in.Description), Unit: string(in.Unit), UnitLabel: textColumn(in.UnitLabel),
			ReferencePriceCents: price.cents, ReferencePriceCurrency: price.currency, Now: now})
		if err != nil {
			return fmt.Errorf("settings: insert catalog item: %w", db.MapError(err))
		}
		created = CatalogItem{ID: id, Name: in.Name, Description: in.Description, Unit: in.Unit, UnitLabel: in.UnitLabel,
			ReferencePrice: in.ReferencePrice, CreatedAt: now, UpdatedAt: now}
		return s.recordCatalogItem(ctx, tx, p, meta, "catalog_item.created", id, map[string]any{"unit": string(in.Unit)})
	})
	if err != nil {
		return CatalogItem{}, err
	}
	return created, nil
}

// UpdateCatalogItem applies a partial change under the item's lock: it merges what the row holds now with the
// present fields, and writes and audits only the fields that end up different. Nothing different writes and
// audits nothing. A unit other than "other" discards the label; "other" without a label is FieldErrors{unit_label:
// required}. Archived items can be edited. No field at all is ErrMalformedChanges; an item that is not the
// company's is ErrNotFound.
// T-002-B602 (002/FR-005, DD-9, DD-15, INV-7, INV-13)
func (s *Service) UpdateCatalogItem(ctx context.Context, p authz.Principal, id uuid.UUID, ch CatalogItemChanges, meta RequestMeta) (CatalogItem, error) {
	if err := requireSettingsManage(p); err != nil {
		return CatalogItem{}, err
	}
	if !ch.Name.Set && !ch.Description.Set && !ch.Unit.Set && !ch.UnitLabel.Set && !ch.ReferencePrice.Set {
		return CatalogItem{}, ErrMalformedChanges
	}
	// What does not depend on the stored row is validated before the lock; the label depends on the final unit.
	fields := FieldErrors{}
	var name string
	if ch.Name.Set {
		var code string
		name, code = normalizeRequiredText(ch.Name.Value, maxItemNameLength)
		setFieldCode(fields, "name", code)
	}
	var description *string
	if ch.Description.Set && ch.Description.Value != nil {
		var code string
		description, code = normalizeOptionalText(*ch.Description.Value, maxDescriptionLength)
		setFieldCode(fields, "description", code)
	}
	if ch.Unit.Set {
		setFieldCode(fields, "unit", validateUnit(ch.Unit.Value))
	}
	if ch.ReferencePrice.Set && ch.ReferencePrice.Value != nil {
		fields.merge(validateReferencePrice(*ch.ReferencePrice.Value))
	}
	if len(fields) > 0 {
		return CatalogItem{}, fields
	}

	return s.writeCatalogItem(ctx, p, id, func(q *store.Queries, tx db.Tx, current CatalogItem, now time.Time) (CatalogItem, error) {
		next := current
		var changed []string // in the order of the audit catalog: name, description, unit, unit_label, reference_price
		var write store.UpdateCatalogItemFieldsParams
		if ch.Name.Set && name != current.Name {
			next.Name = name
			write.SetName, write.Name = true, name
			changed = append(changed, "name")
		}
		if ch.Description.Set && !equalText(description, current.Description) {
			next.Description = description
			write.SetDescription, write.Description = true, textColumn(description)
			changed = append(changed, "description")
		}
		if ch.Unit.Set && ch.Unit.Value != current.Unit {
			next.Unit = ch.Unit.Value
			write.SetUnit, write.Unit = true, string(ch.Unit.Value)
			changed = append(changed, "unit")
		}
		label := next.UnitLabel
		if ch.UnitLabel.Set {
			label = ch.UnitLabel.Value
		}
		label, labelErrs := normalizeUnitLabel(next.Unit, label)
		if labelErrs != nil {
			return CatalogItem{}, labelErrs
		}
		if !equalText(label, current.UnitLabel) {
			next.UnitLabel = label
			write.SetUnitLabel, write.UnitLabel = true, textColumn(label)
			changed = append(changed, "unit_label")
		}
		if ch.ReferencePrice.Set && !equalMoney(ch.ReferencePrice.Value, current.ReferencePrice) {
			next.ReferencePrice = copyMoney(ch.ReferencePrice.Value)
			price := moneyColumns(next.ReferencePrice)
			write.SetReferencePrice, write.ReferencePriceCents, write.ReferencePriceCurrency = true, price.cents, price.currency
			changed = append(changed, "reference_price")
		}
		if len(changed) == 0 {
			return current, nil
		}
		write.TenantID, write.ID, write.Now = p.TenantID, id, now
		if err := q.UpdateCatalogItemFields(ctx, write); err != nil {
			return CatalogItem{}, fmt.Errorf("settings: update catalog item: %w", db.MapError(err))
		}
		next.UpdatedAt = now
		return next, s.recordCatalogItem(ctx, tx, p, meta, "catalog_item.updated", id, map[string]any{"fields": changed})
	})
}

// ArchiveCatalogItem takes an item out of the default list. One already archived comes back as it is, with no write
// and no audit. An item that is not the company's is ErrNotFound.
// T-002-B602 (002/DD-9, DD-15, INV-13)
func (s *Service) ArchiveCatalogItem(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) (CatalogItem, error) {
	return s.setCatalogItemArchived(ctx, p, id, meta, true)
}

// RestoreCatalogItem puts an archived item back in the default list. One already active comes back as it is, with
// no write and no audit. An item that is not the company's is ErrNotFound.
// T-002-B602 (002/DD-9, DD-15, INV-13)
func (s *Service) RestoreCatalogItem(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta) (CatalogItem, error) {
	return s.setCatalogItemArchived(ctx, p, id, meta, false)
}

func (s *Service) setCatalogItemArchived(ctx context.Context, p authz.Principal, id uuid.UUID, meta RequestMeta, archive bool) (CatalogItem, error) {
	if err := requireSettingsManage(p); err != nil {
		return CatalogItem{}, err
	}
	action := "catalog_item.restored"
	if archive {
		action = "catalog_item.archived"
	}
	return s.writeCatalogItem(ctx, p, id, func(q *store.Queries, tx db.Tx, current CatalogItem, now time.Time) (CatalogItem, error) {
		if current.Archived == archive {
			return current, nil
		}
		var archivedAt *time.Time
		if archive {
			archivedAt = &now
		}
		if err := q.SetCatalogItemArchived(ctx, store.SetCatalogItemArchivedParams{TenantID: p.TenantID, ID: id, ArchivedAt: archivedAt, Now: now}); err != nil {
			return CatalogItem{}, fmt.Errorf("settings: set catalog item archived: %w", db.MapError(err))
		}
		current.Archived, current.UpdatedAt = archive, now
		return current, s.recordCatalogItem(ctx, tx, p, meta, action, id, map[string]any{})
	})
}

// writeCatalogItem is the skeleton of the writes on one item: lock its row (the first statement), read the clock
// under the lock (a write that waited must not stamp an older updated_at over the change that held it), and let
// decide act on the item as found under the lock. A row that is not there, or is not the company's, is ErrNotFound;
// the settings row is not locked (002/DD-19).
func (s *Service) writeCatalogItem(ctx context.Context, p authz.Principal, id uuid.UUID,
	decide func(q *store.Queries, tx db.Tx, current CatalogItem, now time.Time) (CatalogItem, error)) (CatalogItem, error) {
	var out CatalogItem
	err := s.runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		row, err := q.LockCatalogItem(ctx, store.LockCatalogItemParams{TenantID: p.TenantID, ID: id})
		if err != nil {
			return catalogReadError("lock", err)
		}
		now := storedInstant(s.clock.Now())
		out, err = decide(q, tx, catalogItemFromRow(row), now)
		return err
	})
	if err != nil {
		return CatalogItem{}, err
	}
	return out, nil
}

func (s *Service) recordCatalogItem(ctx context.Context, tx db.Tx, p authz.Principal, meta RequestMeta, action string, id uuid.UUID, data map[string]any) error {
	return s.audit.Record(ctx, tx, audit.Entry{TenantID: p.TenantID, ActorUserID: &p.UserID, Action: action,
		TargetType: "catalog_item", TargetID: &id, Data: data, IP: meta.IP, UserAgent: meta.UserAgent})
}

// catalogReadError turns "no row" into ErrNotFound, so no handler needs to know about platform/db (002/DD-19).
func catalogReadError(what string, err error) error {
	mapped := db.MapError(err)
	if errors.Is(mapped, db.ErrNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("settings: %s catalog item: %w", what, mapped)
}

func catalogItemFromRow(r store.AppCatalogItem) CatalogItem {
	item := CatalogItem{ID: r.ID, Name: r.Name, Unit: Unit(r.Unit), Archived: r.ArchivedAt != nil, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC()}
	if r.Description.Valid {
		item.Description = &r.Description.String
	}
	if r.UnitLabel.Valid {
		item.UnitLabel = &r.UnitLabel.String
	}
	// The table keeps amount and currency together or neither (catalog_items_reference_price_pair_chk).
	if r.ReferencePriceCents.Valid && r.ReferencePriceCurrency.Valid {
		item.ReferencePrice = &Money{AmountCents: r.ReferencePriceCents.Int64, Currency: r.ReferencePriceCurrency.String}
	}
	return item
}

func textColumn(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

type moneyColumn struct {
	cents    pgtype.Int8
	currency pgtype.Text
}

func moneyColumns(m *Money) moneyColumn {
	if m == nil {
		return moneyColumn{}
	}
	return moneyColumn{cents: pgtype.Int8{Int64: m.AmountCents, Valid: true}, currency: pgtype.Text{String: m.Currency, Valid: true}}
}

func equalText(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalMoney(a, b *Money) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func copyMoney(m *Money) *Money {
	if m == nil {
		return nil
	}
	c := *m
	return &c
}

// storedInstant is t as a timestamptz column keeps it (UTC, microseconds), so the item a write returns prints
// the same instants as the next read of it.
func storedInstant(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }
