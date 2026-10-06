//go:build integration

package app_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/objectstore"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

type httpLogoStorage struct {
	mu             sync.Mutex
	objects        map[string][]byte
	getCount       int
	putErr, getErr error
}

func (s *httpLogoStorage) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putErr != nil {
		return s.putErr
	}
	data, err := io.ReadAll(r)
	s.objects[key] = data
	return err
}
func (s *httpLogoStorage) Get(_ context.Context, key string) (io.ReadCloser, objectstore.ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCount++
	if s.getErr != nil {
		return nil, objectstore.ObjectInfo{}, s.getErr
	}
	data, ok := s.objects[key]
	if !ok {
		return nil, objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), objectstore.ObjectInfo{Size: int64(len(data))}, nil
}
func (s *httpLogoStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}
func phase7Handler(t *testing.T) (http.Handler, string, string, *httpLogoStorage) {
	t.Helper()
	runner := db.NewTxRunner(pgtest.AppPool(t))
	c := clock.Real{}
	h := password.NewHasher(2)
	recorder := audit.NewRecorder()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	storage := &httpLogoStorage{objects: map[string][]byte{}}
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour, identity.WithAuthentication(h, recorder, []byte("0123456789abcdef0123456789abcdef"), logger))
	companies := tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, h, recorder, logger, tenant.WithObjectStorage(storage))
	reg, err := companies.Register(context.Background(), tenant.Signup{Name: "Admin", Email: uuid.NewString() + "@example.com", Password: "valid-password", CompanyName: "ACME", BaseCurrency: "ARS", IndustryTemplateCode: "generic"}, identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	email := uuid.NewString() + "@example.com"
	if _, _, err = users.Invite(context.Background(), reg.Session.Principal, email, "operator", identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	raw := invitationHTTPToken(t, runner, reg.Tenant.ID, email)
	operator, err := users.AcceptInvitation(context.Background(), raw, "Operator", "valid-password", identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	r := app.NewAPIRouter()
	app.RegisterTenantRoutes(r, users, companies, logger)
	return app.NewRootHandler(app.RootDeps{API: r, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: app.SPAUnavailableHandler()}, app.NewCommonMiddleware(logger, true, nil)), reg.Session.RawToken, operator.RawToken, storage
}
func httpPNG(t *testing.T, w, h, size int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if size == 0 {
		return data
	}
	chunk := make([]byte, size-len(data))
	binary.BigEndian.PutUint32(chunk, uint32(len(chunk)-12))
	copy(chunk[4:], "tEXt")
	copy(chunk[8:], "padding\x00")
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	return append(append(append([]byte{}, data[:len(data)-12]...), chunk...), data[len(data)-12:]...)
}
func logoMultipart(t *testing.T, data []byte, names ...string) ([]byte, string) {
	t.Helper()
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	for _, name := range names {
		p, err := m.CreateFormFile(name, strings.Repeat("n", 196)+".png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = p.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes(), m.FormDataContentType()
}
func tenantRequest(t *testing.T, h http.Handler, token, method, path, ct string, data []byte, etag string, want int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", ct)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: token})
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s: got %d want %d body=%s", method, path, rec.Code, want, rec.Body.Bytes())
	}
	req.Body = io.NopCloser(bytes.NewReader(data))
	contract.Default(t).RequireRecorded(t, req, rec)
	cache := "no-store"
	if method == "GET" && strings.HasPrefix(path, "/api/v1/tenant/logo") && (want == 200 || want == 304) {
		cache = "private, no-cache"
	}
	if rec.Header().Get("Cache-Control") != cache {
		t.Fatalf("cache=%q want=%q", rec.Header().Get("Cache-Control"), cache)
	}
	return rec
}
func TestTenantHTTPDataAndPermissions(t *testing.T) {
	h, admin, operator, _ := phase7Handler(t)
	for _, token := range []string{admin, operator} {
		tenantRequest(t, h, token, "GET", "/api/v1/tenant", "", nil, "", 200)
	}
	tenantRequest(t, h, admin, "PATCH", "/api/v1/tenant", "application/json", []byte(`{"tax_id":"30-12345678-1","legal_name":"Legal"}`), "", 200)
	tenantRequest(t, h, admin, "PATCH", "/api/v1/tenant", "application/json", []byte(`{"legal_name":null}`), "", 200)
	for _, tc := range []struct {
		body        string
		status      int
		field, code string
	}{
		{`{"tax_id":"30123456782"}`, 422, "tax_id", "invalid_tax_id"},
		{`{"timezone":"Marte/Olympus"}`, 422, "timezone", "invalid_timezone"},
		{`{"base_currency":"USD"}`, 400, "", "malformed_request"},
		{`{}`, 400, "", "malformed_request"},
	} {
		rec := tenantRequest(t, h, admin, "PATCH", "/api/v1/tenant", "application/json", []byte(tc.body), "", tc.status)
		assertTenantProblem(t, rec, tc.field, tc.code)
	}
	tenantRequest(t, h, operator, "PATCH", "/api/v1/tenant", "application/json", []byte(`{"name":"Other"}`), "", 403)
	for _, method := range []string{"PUT", "DELETE"} {
		tenantRequest(t, h, operator, method, "/api/v1/tenant/logo", "application/json", []byte(`{}`), "", 403)
	}
	for _, tc := range []struct{ method, path string }{{"GET", "/api/v1/tenant"}, {"PATCH", "/api/v1/tenant"}, {"GET", "/api/v1/tenant/logo"}, {"PUT", "/api/v1/tenant/logo"}, {"DELETE", "/api/v1/tenant/logo"}} {
		tenantRequest(t, h, "", tc.method, tc.path, "application/json", nil, "", 401)
	}
}
func assertTenantProblem(t *testing.T, rec *httptest.ResponseRecorder, field, code string) {
	t.Helper()
	var p struct {
		Code   string                         `json:"code"`
		Errors []struct{ Field, Code string } `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if field == "" {
		if p.Code != code {
			t.Fatalf("code=%s want=%s", p.Code, code)
		}
		return
	}
	if p.Code != "validation_failed" || len(p.Errors) != 1 || p.Errors[0].Field != field || p.Errors[0].Code != code {
		t.Fatalf("problem=%s", rec.Body.Bytes())
	}
}
func TestTenantHTTPMultipartTable(t *testing.T) {
	h, admin, _, f := phase7Handler(t)
	valid := httpPNG(t, 10, 10, 0)
	for _, tc := range []struct {
		name        string
		data        []byte
		names       []string
		status      int
		field, code string
	}{
		{"valid", valid, []string{"file"}, 200, "", ""},
		{"exact max and long filename", httpPNG(t, 10, 10, tenant.LogoMaxBytes), []string{"file"}, 200, "", ""},
		{"one extra byte", httpPNG(t, 10, 10, tenant.LogoMaxBytes+1), []string{"file"}, 413, "", "payload_too_large"},
		{"missing", nil, nil, 422, "file", "required"},
		{"empty", nil, []string{"file"}, 422, "file", "required"},
		{"file and foo", valid, []string{"file", "foo"}, 400, "", "malformed_request"},
		{"two files", valid, []string{"file", "file"}, 400, "", "malformed_request"},
		{"only foo", valid, []string{"foo"}, 400, "", "malformed_request"},
		{"spoofed GIF", []byte("GIF89a"), []string{"file"}, 415, "", "unsupported_media_type"},
		{"too wide", httpPNG(t, 2001, 10, 0), []string{"file"}, 422, "file", "invalid_value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, ct := logoMultipart(t, tc.data, tc.names...)
			if tc.name == "spoofed GIF" {
				body = bytes.ReplaceAll(body, []byte("application/octet-stream"), []byte("image/png"))
			}
			rec := tenantRequest(t, h, admin, "PUT", "/api/v1/tenant/logo", ct, body, "", tc.status)
			if tc.status != 200 {
				assertTenantProblem(t, rec, tc.field, tc.code)
			}
		})
	}
	body, ct := logoMultipart(t, valid, "file")
	body = body[:bytes.LastIndex(body, []byte("\r\n--"))+2]
	assertTenantProblem(t, tenantRequest(t, h, admin, "PUT", "/api/v1/tenant/logo", ct, body, "", 400), "", "malformed_request")
	tenantRequest(t, h, admin, "PUT", "/api/v1/tenant/logo", "application/json", []byte(`{}`), "", 415)
	f.putErr = objectstore.ErrUnavailable
	body, ct = logoMultipart(t, valid, "file")
	tenantRequest(t, h, admin, "PUT", "/api/v1/tenant/logo", ct, body, "", 503)
}

type countedLogoReader struct {
	r    io.Reader
	read int
}

func (r *countedLogoReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.read += n
	return n, err
}
func TestTenantHTTPStopsReadingOversizedBodies(t *testing.T) {
	h, admin, _, _ := phase7Handler(t)
	file, ct := logoMultipart(t, httpPNG(t, 10, 10, tenant.LogoMaxBytes+1), "file")
	small, smallCT := logoMultipart(t, httpPNG(t, 10, 10, 0), "file")
	// Oversized preamble is legal multipart syntax. Without MaxBytesReader the small file succeeds.
	preamble := append([]byte(strings.Repeat("x\r\n", (tenant.LogoMaxBodyBytes+100000)/3)), small...)
	for _, tc := range []struct {
		name string
		body []byte
		ct   string
		max  int
	}{
		{"file stops before remaining body", append(file, make([]byte, 1<<20)...), ct, tenant.LogoMaxBodyBytes},
		{"whole body limit", preamble, smallCT, tenant.LogoMaxBodyBytes + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &countedLogoReader{r: bytes.NewReader(tc.body)}
			req := httptest.NewRequest("PUT", "/api/v1/tenant/logo", reader)
			req.Header.Set("Content-Type", tc.ct)
			req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: admin})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != 413 || reader.read > tc.max || reader.read >= len(tc.body) {
				t.Fatalf("status=%d read=%d bound=%d total=%d", rec.Code, reader.read, tc.max, len(tc.body))
			}
			contract.Default(t).RequireRecorded(t, req, rec)
			assertTenantProblem(t, rec, "", "payload_too_large")
		})
	}
}
func TestTenantHTTPLogoCacheLifecycle(t *testing.T) {
	h, admin, operator, f := phase7Handler(t)
	tenantRequest(t, h, admin, "GET", "/api/v1/tenant/logo", "", nil, "", 404)
	before := tenantRequest(t, h, admin, "GET", "/api/v1/tenant", "", nil, "", 200)
	valid := httpPNG(t, 10, 10, 0)
	body, ct := logoMultipart(t, valid, "file")
	upload := tenantRequest(t, h, admin, "PUT", "/api/v1/tenant/logo", ct, body, "", 200)
	var old, newTenant tenant.Tenant
	json.Unmarshal(before.Body.Bytes(), &old)
	json.Unmarshal(upload.Body.Bytes(), &newTenant)
	if !newTenant.HasLogo || !newTenant.UpdatedAt.After(old.UpdatedAt) {
		t.Fatal("upload did not return updated Tenant")
	}
	logo := tenantRequest(t, h, operator, "GET", "/api/v1/tenant/logo", "", nil, "", 200)
	tag := logo.Header().Get("ETag")
	if !bytes.Equal(logo.Body.Bytes(), valid) || logo.Header().Get("Content-Type") != "image/png" || logo.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(tag, `"`) || !strings.HasSuffix(tag, `"`) {
		t.Fatal("invalid logo response headers/bytes")
	}
	tenantRequest(t, h, operator, "GET", "/api/v1/tenant/logo?v=cualquier-cosa", "", nil, "", 200)
	count := f.getCount
	cached := tenantRequest(t, h, operator, "GET", "/api/v1/tenant/logo", "", nil, tag, 304)
	if cached.Body.Len() != 0 || cached.Header().Get("ETag") != tag || cached.Header().Get("X-Content-Type-Options") != "nosniff" || f.getCount != count {
		t.Fatal("304 response had body, wrong ETag or fetched storage")
	}
	replacement := httpPNG(t, 11, 10, 0)
	body, ct = logoMultipart(t, replacement, "file")
	tenantRequest(t, h, admin, "PUT", "/api/v1/tenant/logo", ct, body, "", 200)
	newLogo := tenantRequest(t, h, operator, "GET", "/api/v1/tenant/logo", "", nil, tag, 200)
	if newLogo.Header().Get("ETag") == tag || !bytes.Equal(newLogo.Body.Bytes(), replacement) {
		t.Fatal("replacement reused old ETag/bytes")
	}
	f.getErr = objectstore.ErrUnavailable
	tenantRequest(t, h, operator, "GET", "/api/v1/tenant/logo", "", nil, "", 503)
	f.getErr = nil
	deleted := tenantRequest(t, h, admin, "DELETE", "/api/v1/tenant/logo", "", nil, "", 204)
	if deleted.Body.Len() != 0 {
		t.Fatal("DELETE body")
	}
	tenantRequest(t, h, operator, "GET", "/api/v1/tenant/logo", "", nil, newLogo.Header().Get("ETag"), 404)
	tenantRequest(t, h, admin, "DELETE", "/api/v1/tenant/logo", "", nil, "", 204)
}
