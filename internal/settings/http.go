package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RegisterRoutes expects Authenticate outside the group it receives. Reads need a session; every write sits
// behind RequirePermission(SettingsManage), so the permission is decided before the body is read or the
// resource is looked up (001/INV-12).
// T-002-B404 (002/DD-11, DD-12, DD-18): the stage operations.
// T-002-B504 (002/DD-11, DD-12, DD-16): the category operations.
// T-002-B604 (002/DD-9, DD-11, DD-12, ADR-029): the catalog operations; the other operations add their routes with their tasks.
func RegisterRoutes(r chi.Router, service *Service, logger *slog.Logger) {
	r.Get("/api/v1/stages", func(w http.ResponseWriter, req *http.Request) {
		p, _ := authz.PrincipalFrom(req.Context())
		stages, err := service.ListStages(req.Context(), p)
		if writeSettingsError(w, req, err, logger) {
			return
		}
		writeSettingsJSON(w, http.StatusOK, stageListResponse(stages))
	})
	r.Get("/api/v1/categories", func(w http.ResponseWriter, req *http.Request) {
		filter, ok := categoryFilterFrom(w, req)
		if !ok {
			return
		}
		p, _ := authz.PrincipalFrom(req.Context())
		categories, err := service.ListCategories(req.Context(), p, filter)
		if writeSettingsError(w, req, err, logger) {
			return
		}
		writeSettingsJSON(w, http.StatusOK, categoryListResponse(categories))
	})
	r.Get("/api/v1/catalog-items", func(w http.ResponseWriter, req *http.Request) {
		includeArchived, ok := includeArchivedFrom(w, req)
		if !ok {
			return
		}
		p, _ := authz.PrincipalFrom(req.Context())
		items, err := service.ListCatalogItems(req.Context(), p, includeArchived)
		if writeSettingsError(w, req, err, logger) {
			return
		}
		writeSettingsJSON(w, http.StatusOK, catalogItemListResponse(items))
	})
	r.Get("/api/v1/catalog-items/{catalogItemId}", func(w http.ResponseWriter, req *http.Request) {
		id, ok := catalogItemIDParam(w, req)
		if !ok {
			return
		}
		p, _ := authz.PrincipalFrom(req.Context())
		item, err := service.GetCatalogItem(req.Context(), p, id)
		if writeSettingsError(w, req, err, logger) {
			return
		}
		writeSettingsJSON(w, http.StatusOK, catalogItemResponseOf(item))
	})
	r.Group(func(r chi.Router) {
		r.Use(authz.RequirePermission(authz.SettingsManage))
		r.Post("/api/v1/categories", func(w http.ResponseWriter, req *http.Request) {
			var input categoryCreateRequest
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			created, err := service.CreateCategory(req.Context(), p, input.Name.value, input.Type.value, settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusCreated, categoryResponseOf(created))
		})
		r.Patch("/api/v1/categories/{categoryId}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := categoryIDParam(w, req)
			if !ok {
				return
			}
			var input categoryRenameRequest
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			if !input.Name.present { // the contract requires name; an empty PATCH is a malformed request, not a field error
				httpx.WriteProblem(w, req, httpx.CodeMalformedRequest)
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			renamed, err := service.RenameCategory(req.Context(), p, id, input.Name.value, settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusOK, categoryResponseOf(renamed))
		})
		r.Post("/api/v1/categories/{categoryId}/archive", func(w http.ResponseWriter, req *http.Request) {
			id, ok := categoryIDParam(w, req)
			if !ok {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			archived, err := service.ArchiveCategory(req.Context(), p, id, settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusOK, categoryResponseOf(archived))
		})
		r.Post("/api/v1/categories/{categoryId}/restore", func(w http.ResponseWriter, req *http.Request) {
			id, ok := categoryIDParam(w, req)
			if !ok {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			restored, err := service.RestoreCategory(req.Context(), p, id, settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusOK, categoryResponseOf(restored))
		})
		r.Post("/api/v1/catalog-items", func(w http.ResponseWriter, req *http.Request) {
			var input catalogItemCreateRequest
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			in, fields := input.toNewCatalogItem()
			if fields != nil {
				httpx.ValidationError(w, req, fields)
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			created, err := service.CreateCatalogItem(req.Context(), p, in, settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusCreated, catalogItemResponseOf(created))
		})
		r.Patch("/api/v1/catalog-items/{catalogItemId}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := catalogItemIDParam(w, req)
			if !ok {
				return
			}
			var input catalogItemUpdateRequest
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			changes, fields := input.toChanges()
			if fields != nil {
				httpx.ValidationError(w, req, fields)
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			updated, err := service.UpdateCatalogItem(req.Context(), p, id, changes, settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusOK, catalogItemResponseOf(updated))
		})
		r.Post("/api/v1/catalog-items/{catalogItemId}/archive", func(w http.ResponseWriter, req *http.Request) {
			id, ok := catalogItemIDParam(w, req)
			if !ok {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			archived, err := service.ArchiveCatalogItem(req.Context(), p, id, settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusOK, catalogItemResponseOf(archived))
		})
		r.Post("/api/v1/catalog-items/{catalogItemId}/restore", func(w http.ResponseWriter, req *http.Request) {
			id, ok := catalogItemIDParam(w, req)
			if !ok {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			restored, err := service.RestoreCatalogItem(req.Context(), p, id, settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusOK, catalogItemResponseOf(restored))
		})
		r.Post("/api/v1/stages", func(w http.ResponseWriter, req *http.Request) {
			var input stageCreateRequest
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			created, err := service.CreateStage(req.Context(), p, input.toNewStage(), settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusCreated, stageResponseOf(created))
		})
		r.Patch("/api/v1/stages/{stageId}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := stageIDParam(w, req)
			if !ok {
				return
			}
			var input stageUpdateRequest
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			updated, err := service.UpdateStage(req.Context(), p, id, input.toChanges(), settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusOK, stageResponseOf(updated))
		})
		r.Delete("/api/v1/stages/{stageId}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := stageIDParam(w, req)
			if !ok {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			if writeSettingsError(w, req, service.DeleteStage(req.Context(), p, id, settingsRequestMeta(req)), logger) {
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.Post("/api/v1/stages/{stageId}/move", func(w http.ResponseWriter, req *http.Request) {
			id, ok := stageIDParam(w, req)
			if !ok {
				return
			}
			var input stageMoveRequest
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			if !input.Position.present {
				httpx.ValidationError(w, req, FieldErrors{"position": "required"})
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			stages, err := service.MoveStage(req.Context(), p, id, input.Position.value, settingsRequestMeta(req))
			if writeSettingsError(w, req, err, logger) {
				return
			}
			writeSettingsJSON(w, http.StatusOK, stageListResponse(stages))
		})
	})
}

// optional is a JSON field that tells absent from present. It rejects an explicit null, which the contract
// does not admit in any stage field (plan 9.1: null where it is not admitted is malformed_request): the
// error makes httpx.DecodeJSON answer 400, like a wrong type.
type optional[T any] struct {
	value   T
	present bool
}

var errNullNotAllowed = errors.New("settings: null is not allowed")

func (o *optional[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errNullNotAllowed
	}
	o.present = true
	return json.Unmarshal(data, &o.value)
}

// ptr is the value when present and nil when absent.
func (o optional[T]) ptr() *T {
	if !o.present {
		return nil
	}
	return &o.value
}

// nullable is a JSON field that tells absent, null and a value apart: the fields the contract lets a client clear
// (description, unit_label, reference_price). value is nil for both absent and null; present says which. The
// value is decoded strictly, so an unknown field inside a nested object is a 400 like it is at the top level.
type nullable[T any] struct {
	value   *T
	present bool
}

func (n *nullable[T]) UnmarshalJSON(data []byte) error {
	n.present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		n.value = nil
		return nil
	}
	var v T
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	n.value = &v
	return nil
}

// patch is the field as the service reads it: Set when it came in the body, null included.
func (n nullable[T]) patch() Patch[*T] { return Patch[*T]{Set: n.present, Value: n.value} }

// stageCreateRequest is StageCreate. A missing required field reaches the service as its zero value and
// comes back as a field error (422), the same way signup treats its required fields.
type stageCreateRequest struct {
	Name     optional[string]    `json:"name"`
	Type     optional[StageType] `json:"type"`
	Color    optional[Color]     `json:"color"`
	Position optional[int]       `json:"position"`
}

func (r stageCreateRequest) toNewStage() NewStage {
	return NewStage{Name: r.Name.value, Type: r.Type.value, Color: r.Color.value, Position: r.Position.ptr()}
}

// stageUpdateRequest is StageUpdate: only the present fields change.
type stageUpdateRequest struct {
	Name  optional[string]    `json:"name"`
	Type  optional[StageType] `json:"type"`
	Color optional[Color]     `json:"color"`
}

func (r stageUpdateRequest) toChanges() StageChanges {
	return StageChanges{Name: r.Name.ptr(), Type: r.Type.ptr(), Color: r.Color.ptr()}
}

// stageMoveRequest is StageMove.
type stageMoveRequest struct {
	Position optional[int] `json:"position"`
}

// stageResponse is Stage.
type stageResponse struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Type     StageType `json:"type"`
	Color    Color     `json:"color"`
	Position int       `json:"position"`
}

func stageResponseOf(s Stage) stageResponse {
	return stageResponse(s) // same fields; a new field in Stage stops compiling here, so it is a conscious contract change
}

// stageListResponse is StageList. items is never null, also for an empty list.
type stageListBody struct {
	Items []stageResponse `json:"items"`
}

func stageListResponse(stages []Stage) stageListBody {
	items := make([]stageResponse, len(stages))
	for i, s := range stages {
		items[i] = stageResponseOf(s)
	}
	return stageListBody{Items: items}
}

// stageIDParam reads {stageId}; a malformed id is the same 404 as one that does not exist.
func stageIDParam(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "stageId"))
	if err != nil {
		httpx.WriteProblem(w, req, httpx.CodeNotFound)
		return uuid.Nil, false
	}
	return id, true
}

// categoryCreateRequest is CategoryCreate. A missing field reaches the service as its zero value and comes back
// as a field error (422).
type categoryCreateRequest struct {
	Name optional[string]       `json:"name"`
	Type optional[CategoryType] `json:"type"`
}

// categoryRenameRequest is CategoryRename.
type categoryRenameRequest struct {
	Name optional[string] `json:"name"`
}

// categoryResponse is Category. system is derived from system_key, which is null for the company's own.
type categoryResponse struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Type      CategoryType `json:"type"`
	System    bool         `json:"system"`
	SystemKey *SystemKey   `json:"system_key"`
	Archived  bool         `json:"archived"`
}

func categoryResponseOf(c Category) categoryResponse {
	return categoryResponse{ID: c.ID, Name: c.Name, Type: c.Type, System: c.SystemKey != nil, SystemKey: c.SystemKey, Archived: c.Archived}
}

// categoryListBody is CategoryList. items is never null, also for an empty list.
type categoryListBody struct {
	Items []categoryResponse `json:"items"`
}

func categoryListResponse(categories []Category) categoryListBody {
	items := make([]categoryResponse, len(categories))
	for i, c := range categories {
		items[i] = categoryResponseOf(c)
	}
	return categoryListBody{Items: items}
}

// categoryFilterFrom reads ?type and ?include_archived; a value outside the contract (type other than
// income/expense, a boolean other than true/false) is a 400 (plan 9.1).
func categoryFilterFrom(w http.ResponseWriter, req *http.Request) (CategoryFilter, bool) {
	query := req.URL.Query()
	var filter CategoryFilter
	if query.Has("type") {
		t := CategoryType(query.Get("type"))
		if validateCategoryType(t) != "" {
			httpx.WriteProblem(w, req, httpx.CodeMalformedRequest)
			return CategoryFilter{}, false
		}
		filter.Type = &t
	}
	includeArchived, ok := includeArchivedFrom(w, req)
	if !ok {
		return CategoryFilter{}, false
	}
	filter.IncludeArchived = includeArchived
	return filter, true
}

// includeArchivedFrom reads ?include_archived; a value other than true or false is a 400 (plan 9.1).
func includeArchivedFrom(w http.ResponseWriter, req *http.Request) (bool, bool) {
	query := req.URL.Query()
	if !query.Has("include_archived") {
		return false, true
	}
	switch query.Get("include_archived") {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	httpx.WriteProblem(w, req, httpx.CodeMalformedRequest)
	return false, false
}

// categoryIDParam reads {categoryId}; a malformed id is the same 404 as one that does not exist.
func categoryIDParam(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "categoryId"))
	if err != nil {
		httpx.WriteProblem(w, req, httpx.CodeNotFound)
		return uuid.Nil, false
	}
	return id, true
}

// moneyRequest is Money. The fields are pointers underneath (optional) so that a missing amount is not a zero, and
// amount_cents decodes into an int64, never a float: 1.5, 1e3 and "100" are malformed, and so is anything that
// does not fit (ADR-029).
type moneyRequest struct {
	AmountCents optional[int64]  `json:"amount_cents"`
	Currency    optional[string] `json:"currency"`
}

// toMoney converts the price. The second result is non-nil only when the amount is missing, which the service
// cannot see (a Money always has one): the errors of the whole price, the currency's included. Any other price
// problem is the service's to report.
func (m moneyRequest) toMoney() (*Money, FieldErrors) {
	price := Money{AmountCents: m.AmountCents.value, Currency: m.Currency.value}
	if m.AmountCents.present {
		return &price, nil
	}
	errs := FieldErrors{}
	errs.merge(validateReferencePrice(price))
	errs["reference_price.amount_cents"] = codeRequired
	return nil, errs
}

// referencePriceFrom is the price of a body: nil when absent or null.
func referencePriceFrom(price nullable[moneyRequest]) (*Money, FieldErrors) {
	if price.value == nil {
		return nil, nil
	}
	return price.value.toMoney()
}

// catalogItemCreateRequest is CatalogItemCreate. A missing name or unit reaches the service as its zero value and
// comes back as a field error (422).
type catalogItemCreateRequest struct {
	Name           optional[string]       `json:"name"`
	Description    nullable[string]       `json:"description"`
	Unit           optional[Unit]         `json:"unit"`
	UnitLabel      nullable[string]       `json:"unit_label"`
	ReferencePrice nullable[moneyRequest] `json:"reference_price"`
}

func (r catalogItemCreateRequest) toNewCatalogItem() (NewCatalogItem, FieldErrors) {
	price, fields := referencePriceFrom(r.ReferencePrice)
	if fields != nil {
		return NewCatalogItem{}, fields
	}
	return NewCatalogItem{Name: r.Name.value, Description: r.Description.value, Unit: r.Unit.value, UnitLabel: r.UnitLabel.value, ReferencePrice: price}, nil
}

// catalogItemUpdateRequest is CatalogItemUpdate: only the present fields change; name and unit do not admit null.
type catalogItemUpdateRequest struct {
	Name           optional[string]       `json:"name"`
	Description    nullable[string]       `json:"description"`
	Unit           optional[Unit]         `json:"unit"`
	UnitLabel      nullable[string]       `json:"unit_label"`
	ReferencePrice nullable[moneyRequest] `json:"reference_price"`
}

func (r catalogItemUpdateRequest) toChanges() (CatalogItemChanges, FieldErrors) {
	price, fields := referencePriceFrom(r.ReferencePrice)
	if fields != nil {
		return CatalogItemChanges{}, fields
	}
	changes := CatalogItemChanges{Description: r.Description.patch(), UnitLabel: r.UnitLabel.patch(),
		ReferencePrice: Patch[*Money]{Set: r.ReferencePrice.present, Value: price}}
	if r.Name.present {
		changes.Name = Patch[string]{Set: true, Value: r.Name.value}
	}
	if r.Unit.present {
		changes.Unit = Patch[Unit]{Set: true, Value: r.Unit.value}
	}
	return changes, nil
}

// moneyResponse is Money. AmountCents is an int64, so it leaves as a JSON integer.
type moneyResponse struct {
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

// catalogItemResponse is CatalogItem. description, unit_label and reference_price are null when there is none.
type catalogItemResponse struct {
	ID             uuid.UUID      `json:"id"`
	Name           string         `json:"name"`
	Description    *string        `json:"description"`
	Unit           Unit           `json:"unit"`
	UnitLabel      *string        `json:"unit_label"`
	ReferencePrice *moneyResponse `json:"reference_price"`
	Archived       bool           `json:"archived"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// catalogItemResponseOf maps an item to the body. The service already returns instants as stored (UTC,
// microseconds), so the answer to a write and the next read of the same item print the same text.
func catalogItemResponseOf(c CatalogItem) catalogItemResponse {
	out := catalogItemResponse{ID: c.ID, Name: c.Name, Description: c.Description, Unit: c.Unit, UnitLabel: c.UnitLabel,
		Archived: c.Archived, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
	if c.ReferencePrice != nil {
		out.ReferencePrice = &moneyResponse{AmountCents: c.ReferencePrice.AmountCents, Currency: c.ReferencePrice.Currency}
	}
	return out
}

// catalogItemListBody is CatalogItemList. items is never null, also for an empty list.
type catalogItemListBody struct {
	Items []catalogItemResponse `json:"items"`
}

func catalogItemListResponse(items []CatalogItem) catalogItemListBody {
	out := make([]catalogItemResponse, len(items))
	for i, c := range items {
		out[i] = catalogItemResponseOf(c)
	}
	return catalogItemListBody{Items: out}
}

// catalogItemIDParam reads {catalogItemId}; a malformed id is the same 404 as one that does not exist.
func catalogItemIDParam(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "catalogItemId"))
	if err != nil {
		httpx.WriteProblem(w, req, httpx.CodeNotFound)
		return uuid.Nil, false
	}
	return id, true
}

func settingsRequestMeta(req *http.Request) RequestMeta {
	return RequestMeta{IP: httpx.ClientIPFrom(req.Context()), UserAgent: req.UserAgent()}
}

func writeSettingsJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeSettingsError answers the error of a use case (plan 9.2) and reports whether there was one.
func writeSettingsError(w http.ResponseWriter, req *http.Request, err error, logger *slog.Logger) bool {
	if err == nil {
		return false
	}
	var fields FieldErrors
	switch {
	case errors.As(err, &fields):
		httpx.ValidationError(w, req, fields)
	case errors.Is(err, ErrMalformedChanges):
		httpx.WriteProblem(w, req, httpx.CodeMalformedRequest)
	case errors.Is(err, authz.ErrForbidden):
		httpx.WriteProblem(w, req, httpx.CodeForbidden)
	case errors.Is(err, ErrNotFound):
		httpx.WriteProblem(w, req, httpx.CodeNotFound)
	case errors.Is(err, ErrNameTaken):
		httpx.WriteProblem(w, req, httpx.CodeNameTaken)
	case errors.Is(err, ErrPipelineRule), errors.Is(err, ErrSystemCategory):
		httpx.WriteProblem(w, req, httpx.CodeInvalidState)
	case errors.Is(err, ErrTenantSettingsMissing):
		// The company lost its tenant_settings row (002/INV-1, DD-19): a bug to investigate, never a 404.
		logger.ErrorContext(req.Context(), "settings row missing", "event", "settings_row_missing",
			"request_id", httpx.RequestIDFrom(req.Context()), "err", err)
		httpx.WriteProblem(w, req, httpx.CodeInternal)
	default:
		httpx.WriteDBError(w, req, err, logger)
	}
	return true
}
