package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RegisterRoutes expects Authenticate outside the group it receives. Reads need a session; every write sits
// behind RequirePermission(SettingsManage), so the permission is decided before the body is read or the
// resource is looked up (001/INV-12).
// T-002-B404 (002/DD-11, DD-12, DD-18): the stage operations; the other operations add their routes with their tasks.
func RegisterRoutes(r chi.Router, service *Service, logger *slog.Logger) {
	r.Get("/api/v1/stages", func(w http.ResponseWriter, req *http.Request) {
		p, _ := authz.PrincipalFrom(req.Context())
		stages, err := service.ListStages(req.Context(), p)
		if writeSettingsError(w, req, err, logger) {
			return
		}
		writeSettingsJSON(w, http.StatusOK, stageListResponse(stages))
	})
	r.Group(func(r chi.Router) {
		r.Use(authz.RequirePermission(authz.SettingsManage))
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
	case errors.Is(err, ErrPipelineRule):
		httpx.WriteProblem(w, req, httpx.CodeInvalidState)
	case errors.Is(err, db.ErrNotFound):
		// The use cases turn a missing resource into ErrNotFound. A bare database "no row" is the company
		// that lost its tenant_settings row (002/INV-1): a bug to investigate, never a 404.
		logger.ErrorContext(req.Context(), "settings row missing", "event", "settings_row_missing",
			"request_id", httpx.RequestIDFrom(req.Context()), "err", err)
		httpx.WriteProblem(w, req, httpx.CodeInternal)
	default:
		httpx.WriteDBError(w, req, err, logger)
	}
	return true
}
