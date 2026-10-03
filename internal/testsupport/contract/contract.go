// Package contract validates API tests against the canonical OpenAPI document of spec 001.
package contract

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	libvalidator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// Validator serializes calls because libopenapi-validator does not promise concurrent safety.
type Validator struct {
	mu      sync.Mutex
	api     libvalidator.Validator
	schemas schema_validation.SchemaValidator
	model   *v3.Document
}

// Load finds go.mod from the caller's working directory and loads the canonical spec.
func Load() (*Validator, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("contract: working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("contract: find go.mod: %w", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, errors.New("contract: go.mod not found")
		}
		dir = parent
	}
	path := filepath.Join(dir, "specs/001-empresas-usuarios/contracts/openapi.yaml")
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("contract: read %s: %w", path, err)
	}
	doc, err := libopenapi.NewDocument(source)
	if err != nil {
		return nil, fmt.Errorf("contract: parse: %w", err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("contract: build model: %w", err)
	}
	api, constructionErrors := libvalidator.NewValidator(doc, config.WithFormatAssertions())
	if len(constructionErrors) != 0 {
		return nil, fmt.Errorf("contract: construct validator: %w", errors.Join(constructionErrors...))
	}
	if valid, validationErrors := api.ValidateDocument(); !valid {
		return nil, fmt.Errorf("contract: invalid document: %w", joinValidationErrors(validationErrors))
	}
	return &Validator{api: api, schemas: schema_validation.NewSchemaValidator(config.WithFormatAssertions()), model: &model.Model}, nil
}

var loadDefault = sync.OnceValues(Load)

// Default loads the document once per test process.
func Default(t testing.TB) *Validator {
	t.Helper()
	v, err := loadDefault()
	if err != nil {
		t.Fatalf("load API contract: %v", err)
	}
	return v
}

// CheckResponse validates status, declared headers, media type and body, restoring resp.Body.
func (v *Validator) CheckResponse(req *http.Request, resp *http.Response) error {
	if req == nil || resp == nil {
		return errors.New("contract: request and response are required")
	}
	if resp.Body == nil {
		resp.Body = http.NoBody
	}
	body, err := readAndClose(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("contract: read response body: %w", err)
	}
	v.mu.Lock()
	valid, validationErrors := v.api.ValidateHttpResponse(req, resp)
	v.mu.Unlock()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if valid {
		return nil
	}
	if isMissingRoute(validationErrors) {
		if req.URL != nil && ((resp.StatusCode == http.StatusNotFound && validationErrors[0].ValidationSubType == helpers.ValidationMissing) ||
			(resp.StatusCode == http.StatusMethodNotAllowed && validationErrors[0].ValidationSubType == helpers.ValidationMissingOperation)) &&
			strings.HasPrefix(resp.Header.Get("Content-Type"), "application/problem+json") {
			if resp.StatusCode == http.StatusMethodNotAllowed && resp.Header.Get("Allow") == "" {
				return fmt.Errorf("contract: %s %s 405: missing Allow header", req.Method, req.URL.Path)
			}
			if err := v.CheckSchema("Problem", body); err != nil {
				return fmt.Errorf("contract: %s %s %d: %w", req.Method, req.URL.Path, resp.StatusCode, err)
			}
			return nil
		}
	}
	return fmt.Errorf("contract: %s %s %d: %w", req.Method, req.URL.Path, resp.StatusCode, joinValidationErrors(validationErrors))
}

func isMissingRoute(errs []*liberrors.ValidationError) bool {
	return len(errs) == 1 && errs[0].ValidationType == helpers.PathValidation
}

// CheckRequest validates parameters and body, restoring req.Body.
func (v *Validator) CheckRequest(req *http.Request) error {
	if req == nil {
		return errors.New("contract: request is required")
	}
	if req.Body == nil {
		req.Body = http.NoBody
	}
	body, err := readAndClose(req.Body)
	req.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("contract: read request body: %w", err)
	}
	v.mu.Lock()
	valid, validationErrors := v.api.ValidateHttpRequest(req)
	v.mu.Unlock()
	req.Body = io.NopCloser(bytes.NewReader(body))
	if !valid {
		return fmt.Errorf("contract: %s %s request: %w", req.Method, req.URL.Path, joinValidationErrors(validationErrors))
	}
	return nil
}

// CheckSchema validates a JSON body against a named components/schemas entry.
func (v *Validator) CheckSchema(name string, body []byte) error {
	if v.model.Components == nil || v.model.Components.Schemas == nil {
		return errors.New("contract: no component schemas")
	}
	proxy, ok := v.model.Components.Schemas.Get(name)
	if !ok {
		return fmt.Errorf("contract: schema %q does not exist", name)
	}
	v.mu.Lock()
	valid, validationErrors := v.schemas.ValidateSchemaBytes(proxy.Schema(), body)
	v.mu.Unlock()
	if !valid {
		return fmt.Errorf("contract: schema %s: %w", name, joinValidationErrors(validationErrors))
	}
	return nil
}

func joinValidationErrors(validationErrors []*liberrors.ValidationError) error {
	if len(validationErrors) == 0 {
		return errors.New("validation failed without details")
	}
	errList := make([]error, 0, len(validationErrors))
	for _, err := range validationErrors {
		errList = append(errList, err)
	}
	return errors.Join(errList...)
}

// readAndClose copies a streamed body before replacing it with an in-memory reader. The original
// must be closed even when reading fails; otherwise the HTTP transport retains its resources.
func readAndClose(body io.ReadCloser) ([]byte, error) {
	data, readErr := io.ReadAll(body)
	return data, errors.Join(readErr, body.Close())
}

type validatingTransport struct {
	validator *Validator
	next      http.RoundTripper
	report    func(error)
}

// Transport validates every response and validates a request only after a 2xx response.
func (v *Validator) Transport(next http.RoundTripper, report func(error)) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return validatingTransport{validator: v, next: next, report: report}
}

func (t validatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var requestBody []byte
	if req.Body != nil {
		var err error
		requestBody, err = readAndClose(req.Body)
		if err != nil {
			return nil, fmt.Errorf("contract: copy request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(requestBody))
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if err := t.validator.CheckResponse(req, resp); err != nil && t.report != nil {
		t.report(err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		copyReq := req.Clone(req.Context())
		copyReq.Body = io.NopCloser(bytes.NewReader(requestBody))
		if err := t.validator.CheckRequest(copyReq); err != nil && t.report != nil {
			t.report(err)
		}
	}
	return resp, nil
}

// Wrap replaces the client's transport and reports failures through the test.
func (v *Validator) Wrap(t testing.TB, client *http.Client) {
	t.Helper()
	client.Transport = v.Transport(client.Transport, func(err error) { t.Errorf("API contract: %v", err) })
}

// RequireRecorded validates a recorder's response and fails the test on a mismatch.
func (v *Validator) RequireRecorded(t testing.TB, req *http.Request, rec *httptest.ResponseRecorder) {
	t.Helper()
	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()
	if err := v.CheckResponse(req, resp); err != nil {
		t.Errorf("API contract: %v", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := v.CheckRequest(req); err != nil {
			t.Errorf("API contract: %v", err)
		}
	}
}
