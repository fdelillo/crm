// Package contract validates API tests against the canonical OpenAPI documents of the specs (Documents).
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
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	libvalidator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/index"
)

// Validator serializes calls because libopenapi-validator does not promise concurrent safety.
type Validator struct {
	mu      sync.Mutex
	docs    []*document
	schemas schema_validation.SchemaValidator
}

// document is one loaded contract; Documents order decides which one answers first.
type document struct {
	path       string
	api        libvalidator.Validator
	model      *v3.Document
	references []Reference
}

// Load finds go.mod from the caller's working directory and loads every contract of Documents.
// It fails when a contract is invalid, when a $ref (internal or into another file) does not
// resolve, or when two contracts declare the same method and path.
func Load() (*Validator, error) {
	root, err := moduleRoot()
	if err != nil {
		return nil, err
	}
	return load(root, Documents)
}

// Documents lists the canonical contracts the tests validate against, relative to the module root.
// Adding a spec is one line here (DD-13).
var Documents = []string{
	"specs/001-empresas-usuarios/contracts/openapi.yaml",
	"specs/002-configuracion/contracts/openapi.yaml", //nolint:misspell // Spanish directory name of spec 002
}

// Reference is one $ref found in a contract and whether it resolved to a node.
type Reference struct {
	Document string
	Ref      string
	Resolved bool
}

// References lists every $ref of every loaded contract, internal and into other files.
func (v *Validator) References() []Reference {
	var all []Reference
	for _, d := range v.docs {
		all = append(all, d.references...)
	}
	return all
}

// UnresolvedReferences lists the references that did not resolve to a node. Load fails when it
// would not be empty, so a loaded Validator always returns none.
func (v *Validator) UnresolvedReferences() []Reference {
	var unresolved []Reference
	for _, ref := range v.References() {
		if !ref.Resolved {
			unresolved = append(unresolved, ref)
		}
	}
	return unresolved
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("contract: working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("contract: find go.mod: %w", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("contract: go.mod not found")
		}
		dir = parent
	}
}

func load(root string, files []string) (*Validator, error) {
	v := &Validator{schemas: schema_validation.NewSchemaValidator(config.WithFormatAssertions())}
	routes := map[string]string{}
	for _, file := range files {
		d, err := loadDocument(root, file)
		if err != nil {
			return nil, err
		}
		for _, op := range d.operations() {
			key := op.Method + " " + op.Path
			if other, dup := routes[key]; dup {
				return nil, fmt.Errorf("contract: route %s is declared in both %s and %s", key, other, file)
			}
			routes[key] = file
		}
		v.docs = append(v.docs, d)
	}
	return v, nil
}

// loadDocument parses one contract resolving $ref into sibling files relative to its directory
// (S-4: libopenapi follows local file references when BasePath and AllowFileReferences are set).
func loadDocument(root, file string) (*document, error) {
	path := filepath.Join(root, file)
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("contract: read %s: %w", path, err)
	}
	cfg := datamodel.NewDocumentConfiguration()
	cfg.BasePath = filepath.Dir(path)
	cfg.AllowFileReferences = true
	doc, err := libopenapi.NewDocumentWithConfiguration(source, cfg)
	if err != nil {
		return nil, fmt.Errorf("contract: parse %s: %w", file, err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("contract: build model of %s: %w", file, err)
	}
	api, constructionErrors := libvalidator.NewValidator(doc, config.WithFormatAssertions())
	if len(constructionErrors) != 0 {
		return nil, fmt.Errorf("contract: construct validator of %s: %w", file, errors.Join(constructionErrors...))
	}
	if valid, validationErrors := api.ValidateDocument(); !valid {
		return nil, fmt.Errorf("contract: invalid document %s: %w", file, joinValidationErrors(validationErrors))
	}
	d := &document{path: file, api: api, model: &model.Model, references: referencesOf(file, model.Model.Index)}
	for _, ref := range d.references {
		if !ref.Resolved {
			return nil, fmt.Errorf("contract: %s: unresolved reference %s", file, ref.Ref)
		}
	}
	return d, nil
}

// referencesOf lists every $ref the index found; a reference is resolved when the index mapped it
// to a node.
func referencesOf(file string, idx *index.SpecIndex) []Reference {
	mapped := idx.GetMappedReferences()
	var refs []Reference
	for ref := range idx.GetAllReferences() {
		_, ok := mapped[ref]
		refs = append(refs, Reference{Document: file, Ref: ref, Resolved: ok})
	}
	slices.SortFunc(refs, func(a, b Reference) int { return strings.Compare(a.Ref, b.Ref) })
	return refs
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
	valid, validationErrors := v.dispatch(func(api libvalidator.Validator) (bool, []*liberrors.ValidationError) {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return api.ValidateHttpResponse(req, resp)
	})
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

// dispatch asks each contract in Documents order and keeps the first answer that is not "route not
// found", so a request is judged by the contract that declares its path. When none declares it, the
// answer says so; if a contract has the path but not the method (405), that answer wins over the
// "path not found" of the others. Callers hold v.mu.
func (v *Validator) dispatch(validate func(libvalidator.Validator) (bool, []*liberrors.ValidationError)) (bool, []*liberrors.ValidationError) {
	var fallback []*liberrors.ValidationError
	for _, d := range v.docs {
		valid, errs := validate(d.api)
		if valid || !isMissingRoute(errs) {
			return valid, errs
		}
		if fallback == nil || errs[0].ValidationSubType == helpers.ValidationMissingOperation {
			fallback = errs
		}
	}
	return false, fallback
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
	valid, validationErrors := v.dispatch(func(api libvalidator.Validator) (bool, []*liberrors.ValidationError) {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return api.ValidateHttpRequest(req)
	})
	v.mu.Unlock()
	req.Body = io.NopCloser(bytes.NewReader(body))
	if !valid {
		return fmt.Errorf("contract: %s %s request: %w", req.Method, req.URL.Path, joinValidationErrors(validationErrors))
	}
	return nil
}

// CheckSchema validates a JSON body against a named components/schemas entry, looked up in the
// contracts in Documents order.
func (v *Validator) CheckSchema(name string, body []byte) error {
	for _, d := range v.docs {
		if d.model.Components == nil || d.model.Components.Schemas == nil {
			continue
		}
		proxy, ok := d.model.Components.Schemas.Get(name)
		if !ok {
			continue
		}
		v.mu.Lock()
		valid, validationErrors := v.schemas.ValidateSchemaBytes(proxy.Schema(), body)
		v.mu.Unlock()
		if !valid {
			return fmt.Errorf("contract: schema %s: %w", name, joinValidationErrors(validationErrors))
		}
		return nil
	}
	return fmt.Errorf("contract: schema %q does not exist", name)
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

// Operation is one explicitly declared HTTP method and path of a canonical document, with its
// operationId and the contract file (as listed in Documents) that declares it.
type Operation struct{ Method, Path, ID, Document string }

// Operations exposes paths/methods of every contract without deriving expectations from the
// application's router. Paths are relative to the OpenAPI server (/api/v1 for business routes);
// ops remain root paths.
func (v *Validator) Operations() []Operation {
	v.mu.Lock()
	defer v.mu.Unlock()
	var result []Operation
	for _, d := range v.docs {
		result = append(result, d.operations()...)
	}
	return result
}

func (d *document) operations() []Operation {
	var result []Operation
	for path, item := range d.model.Paths.PathItems.FromOldest() {
		for method, op := range item.GetOperations().FromOldest() {
			result = append(result, Operation{Method: strings.ToUpper(method), Path: path, ID: op.OperationId, Document: d.path})
		}
	}
	return result
}
