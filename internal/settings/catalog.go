package settings

import (
	"time"

	"github.com/google/uuid"
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
	Name           Patch[string]  // null -> ErrMalformedChanges
	Description    Patch[*string] // null or "" -> no description
	Unit           Patch[Unit]    // null -> ErrMalformedChanges
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
