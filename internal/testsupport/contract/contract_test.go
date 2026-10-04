package contract

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type observedBody struct {
	reader io.Reader
	closed bool
}

func (b *observedBody) Read(p []byte) (int, error) { return b.reader.Read(p) }
func (b *observedBody) Close() error               { b.closed = true; return nil }

type failedReader struct{ err error }

func (r failedReader) Read([]byte) (int, error) { return 0, r.err }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestCheckResponseClosesOriginalBody(t *testing.T) {
	v := Default(t)
	req := httptest.NewRequest(http.MethodGet, "https://crm.example/api/v1/industry-templates", nil)
	good := `{"items":[{"code":"generic","name":"Genérico"}]}`
	original := &observedBody{reader: strings.NewReader(good)}
	resp := response(200, "application/json", "")
	resp.Body = original
	if err := v.CheckResponse(req, resp); err != nil {
		t.Fatal(err)
	}
	if !original.closed {
		t.Fatal("original response body was not closed")
	}
	if restored, err := io.ReadAll(resp.Body); err != nil || string(restored) != good {
		t.Fatalf("restored response body=%q err=%v", restored, err)
	}
	_ = resp.Body.Close()

	readErr := errors.New("read failed")
	broken := &observedBody{reader: io.MultiReader(strings.NewReader("partial"), failedReader{readErr})}
	resp = response(200, "application/json", "")
	resp.Body = broken
	if err := v.CheckResponse(req, resp); !errors.Is(err, readErr) {
		t.Fatalf("read error=%v", err)
	}
	if !broken.closed {
		t.Fatal("original response body was not closed after read failure")
	}
	if restored, err := io.ReadAll(resp.Body); err != nil || string(restored) != "partial" {
		t.Fatalf("partial response body=%q err=%v", restored, err)
	}
	_ = resp.Body.Close()
}

func TestCheckRequestClosesOriginalBody(t *testing.T) {
	v := Default(t)
	good := `{"name":"Ana","email":"ana@example.com","password":"example-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`
	original := &observedBody{reader: strings.NewReader(good)}
	req := httptest.NewRequest(http.MethodPost, "https://crm.example/api/v1/auth/signup", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = original
	if err := v.CheckRequest(req); err != nil {
		t.Fatal(err)
	}
	if !original.closed {
		t.Fatal("original request body was not closed")
	}
	if restored, err := io.ReadAll(req.Body); err != nil || string(restored) != good {
		t.Fatalf("restored request body=%q err=%v", restored, err)
	}
	_ = req.Body.Close()
	readErr := errors.New("request read failed")
	broken := &observedBody{reader: io.MultiReader(strings.NewReader("partial"), failedReader{readErr})}
	req.Body = broken
	if err := v.CheckRequest(req); !errors.Is(err, readErr) {
		t.Fatalf("request read error=%v", err)
	}
	if !broken.closed {
		t.Fatal("original request body was not closed after read failure")
	}
	if restored, err := io.ReadAll(req.Body); err != nil || string(restored) != "partial" {
		t.Fatalf("partial request body=%q err=%v", restored, err)
	}
	_ = req.Body.Close()
}

func TestTransportClosesOriginalRequestBodyOnCopyFailure(t *testing.T) {
	v := Default(t)
	readErr := errors.New("request read failed")
	original := &observedBody{reader: failedReader{readErr}}
	req := httptest.NewRequest(http.MethodPost, "https://crm.example/api/v1/auth/signup", nil)
	req.Body = original
	transport := v.Transport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("transport was called after request body read failed")
		return nil, nil
	}), nil)
	if _, err := transport.RoundTrip(req); !errors.Is(err, readErr) {
		t.Fatalf("copy error=%v", err)
	}
	if !original.closed {
		t.Fatal("original request body was not closed after copy failure")
	}
	good := `{"name":"Ana","email":"ana@example.com","password":"example-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`
	original = &observedBody{reader: strings.NewReader(good)}
	req = httptest.NewRequest(http.MethodPost, "https://crm.example/api/v1/auth/signup", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = original
	transport = v.Transport(roundTripFunc(func(sent *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(sent.Body)
		_ = sent.Body.Close()
		if err != nil || string(body) != good {
			t.Fatalf("sent request body=%q err=%v", body, err)
		}
		return response(400, "application/problem+json", problem(400, "malformed_request")), nil
	}), func(err error) { t.Errorf("contract validation: %v", err) })
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if !original.closed {
		t.Fatal("original request body was not closed after successful copy")
	}
}

func response(status int, contentType, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestLoadAndCheckResponse(t *testing.T) {
	v, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	good := `{"items":[{"code":"generic","name":"Genérico"}]}`
	req := httptest.NewRequest(http.MethodGet, "https://crm.example/api/v1/industry-templates", nil)
	resp := response(200, "application/json", good)
	if err := v.CheckResponse(req, resp); err != nil {
		t.Fatalf("valid response: %v", err)
	}
	if body, _ := io.ReadAll(resp.Body); string(body) != good {
		t.Fatalf("response body was consumed: %q", body)
	}
	if err := v.CheckResponse(req, response(200, "application/json", `{"items":[{"code":"generic","name":"Genérico","extra":1}]}`)); err == nil {
		t.Fatal("unexpected property was accepted")
	}
	if err := v.CheckResponse(req, response(413, "application/problem+json", problem(413, "payload_too_large"))); err == nil {
		t.Fatal("undeclared 413 was accepted")
	}

	signup := httptest.NewRequest(http.MethodPost, "https://crm.example/api/v1/auth/signup", nil)
	if err := v.CheckResponse(signup, response(413, "application/problem+json", problem(413, "payload_too_large"))); err != nil {
		t.Fatalf("declared 413: %v", err)
	}
	if err := v.CheckResponse(signup, response(418, "application/problem+json", problem(418, "internal"))); err == nil {
		t.Fatal("undeclared 418 was accepted")
	}
	if err := v.CheckResponse(signup, response(409, "application/json", problem(409, "email_already_registered"))); err == nil {
		t.Fatal("wrong content type was accepted")
	}
	conflict := `{"type":"/problems/email_already_registered","title":"Ya existe","status":409,"code":"email_already_registered","detail":"Recuperá la contraseña","suggested_action":"password_reset"}`
	if err := v.CheckResponse(signup, response(409, "application/problem+json", conflict)); err != nil {
		t.Fatalf("valid conflict: %v", err)
	}
	if err := v.CheckResponse(signup, response(409, "application/problem+json", strings.Replace(conflict, `,"suggested_action":"password_reset"`, "", 1))); err == nil {
		t.Fatal("conflict without suggested_action accepted")
	}
	unavailable := response(503, "application/problem+json", problem(503, "service_unavailable"))
	unavailable.Header.Set("Retry-After", "2")
	if err := v.CheckResponse(signup, unavailable); err != nil {
		t.Fatalf("5XX and Retry-After: %v", err)
	}
	unavailable = response(503, "application/problem+json", problem(503, "service_unavailable"))
	unavailable.Header.Set("Retry-After", "0")
	if err := v.CheckResponse(signup, unavailable); err == nil {
		t.Fatal("invalid Retry-After accepted")
	}
}

func TestGlobalResponsesAndSchemas(t *testing.T) {
	v := Default(t)
	missing := httptest.NewRequest(http.MethodGet, "https://crm.example/api/v1/no-existe", nil)
	if err := v.CheckResponse(missing, response(404, "application/problem+json", problem(404, "not_found"))); err != nil {
		t.Fatalf("global 404: %v", err)
	}
	if err := v.CheckResponse(missing, response(200, "application/json", `{}`)); err == nil {
		t.Fatal("200 on missing route accepted")
	}
	badMethod := httptest.NewRequest(http.MethodDelete, "https://crm.example/api/v1/industry-templates", nil)
	resp := response(405, "application/problem+json", problem(405, "method_not_allowed"))
	resp.Header.Set("Allow", "GET")
	if err := v.CheckResponse(badMethod, resp); err != nil {
		t.Fatalf("global 405: %v", err)
	}
	if err := v.CheckResponse(badMethod, response(405, "application/problem+json", problem(405, "method_not_allowed"))); err == nil {
		t.Fatal("405 without Allow accepted")
	}
	if err := v.CheckSchema("ValidationProblem", []byte(`{"type":"/problems/validation_failed","title":"Error","status":422,"code":"validation_failed","errors":[{"field":"email","code":"required"}]}`)); err != nil {
		t.Fatalf("valid schema: %v", err)
	}
	if err := v.CheckSchema("ValidationProblem", []byte(`{"type":"/problems/validation_failed","title":"Error","status":422,"code":"validation_failed","errors":[{"field":"email","code":"no_such_code"}]}`)); err == nil {
		t.Fatal("bad field error code accepted")
	}
	if err := v.CheckSchema("NoExiste", []byte(`{}`)); err == nil {
		t.Fatal("missing schema accepted")
	}
}

func TestConcurrentValidationAndEmptyLogout(t *testing.T) {
	v := Default(t)
	logout := httptest.NewRequest(http.MethodPost, "https://crm.example/api/v1/auth/logout", nil)
	if err := v.CheckResponse(logout, response(204, "", "")); err != nil {
		t.Fatalf("empty logout: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "https://crm.example/api/v1/industry-templates", nil)
			errs <- v.CheckResponse(req, response(200, "application/json", `{"items":[{"code":"generic","name":"Genérico"}]}`))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent validation: %v", err)
		}
	}
}

func TestCheckRequestRestoresBody(t *testing.T) {
	v := Default(t)
	body := `{"name":"Ana","email":"ana@example.com","password":"example-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`
	req := httptest.NewRequest(http.MethodPost, "https://crm.example/api/v1/auth/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if err := v.CheckRequest(req); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	if got, _ := io.ReadAll(req.Body); string(got) != body {
		t.Fatalf("request body was consumed: %q", got)
	}
	req = httptest.NewRequest(http.MethodPost, "https://crm.example/api/v1/auth/signup", strings.NewReader(strings.TrimSuffix(body, "}")+`,"extra":1}`))
	req.Header.Set("Content-Type", "application/json")
	if err := v.CheckRequest(req); err == nil {
		t.Fatal("extra request field accepted")
	}
}

func TestTransportValidatesSuccessfulRequestsAndEveryResponse(t *testing.T) {
	v := Default(t)
	status := 201
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status == 201 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Set-Cookie", "__Host-crm_session=abc; Path=/; HttpOnly; Secure; SameSite=Lax")
			w.WriteHeader(201)
			_, _ = io.WriteString(w, `{"user":{"id":"00000000-0000-4000-8000-000000000001","name":"Ana","email":"ana@example.com","role":"admin","status":"active","email_verified":false},"tenant":{"id":"00000000-0000-4000-8000-000000000002","name":"ACME","base_currency":"ARS","timezone":"America/Argentina/Buenos_Aires","has_logo":false},"permissions":[]}`)
			return
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(400)
		_, _ = io.WriteString(w, problem(400, "malformed_request"))
	}))
	defer server.Close()
	client := server.Client()
	var reported []error
	client.Transport = v.Transport(client.Transport, func(err error) { reported = append(reported, err) })
	valid := `{"name":"Ana","email":"ana@example.com","password":"example-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"generic"}`
	post := func(body string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/signup", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	post(valid)
	if len(reported) != 0 {
		t.Fatalf("valid exchange: %v", reported)
	}
	invalid := strings.TrimSuffix(valid, "}") + `,"extra":1}`
	post(invalid)
	if len(reported) != 1 || !strings.Contains(reported[0].Error(), "request") {
		t.Fatalf("invalid request with 201: %v", reported)
	}
	reported = nil
	status = 400
	post(invalid)
	if len(reported) != 0 {
		t.Fatalf("invalid request with 400: %v", reported)
	}
}

func problem(status int, code string) string {
	return `{"type":"/problems/` + code + `","title":"Error","status":` + strconv.Itoa(status) + `,"code":"` + code + `"}`
}
