//go:build integration

package tenant_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"path"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/objectstore"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type logoObject struct {
	data        []byte
	contentType string
}
type logoStorage struct {
	mu                  sync.Mutex
	objects             map[string]logoObject
	puts, gets, deletes []string
	putErr, deleteErr   error
	onPut, onDelete     func(string)
}

func newLogoStorage() *logoStorage { return &logoStorage{objects: map[string]logoObject{}} }
func (f *logoStorage) Put(_ context.Context, key string, r io.Reader, size int64, ct string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		return errors.New("size mismatch")
	}
	if f.onPut != nil {
		f.onPut(key)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts = append(f.puts, key)
	if f.putErr != nil {
		return f.putErr
	}
	f.objects[key] = logoObject{data, ct}
	return nil
}
func (f *logoStorage) Get(_ context.Context, key string) (io.ReadCloser, objectstore.ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets = append(f.gets, key)
	o, ok := f.objects[key]
	if !ok {
		return nil, objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(o.data)), objectstore.ObjectInfo{Size: int64(len(o.data)), ContentType: o.contentType}, nil
}
func (f *logoStorage) Delete(_ context.Context, key string) error {
	if f.onDelete != nil {
		f.onDelete(key)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes = append(f.deletes, key)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.objects, key)
	return nil
}
func logoService(t *testing.T, runner db.TxRunner, recorder audit.Recorder, f objectstore.ObjectStorage, logs io.Writer) *tenant.Service {
	t.Helper()
	_, ident, _ := newRegistrationServices(t)
	return tenant.NewService(runner, ident, industrytemplate.NoopSeeder{}, password.NewHasher(2), recorder, slog.New(slog.NewTextHandler(logs, nil)), tenant.WithObjectStorage(f))
}
func pngLogo(t *testing.T, w, h, size int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	if size == 0 {
		return data
	}
	padding := size - len(data) - 12
	if padding < 0 {
		t.Fatal("PNG fixture size too small")
	}
	chunk := make([]byte, padding+12)
	binary.BigEndian.PutUint32(chunk, uint32(padding))
	copy(chunk[4:8], "tEXt")
	copy(chunk[8:], "padding\x00")
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	return append(append(append([]byte{}, data[:len(data)-12]...), chunk...), data[len(data)-12:]...)
}
func jpegLogo(t *testing.T, exif bool) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 10, 10)), nil); err != nil {
		t.Fatal(err)
	}
	if !exif {
		return b.Bytes()
	}
	// Valid little-endian TIFF: orientation=6 and GPS IFD with fictitious latitude 1/1,2/1,3/1.
	tiff := make([]byte, 92)
	copy(tiff, "II")
	binary.LittleEndian.PutUint16(tiff[2:], 42)
	binary.LittleEndian.PutUint32(tiff[4:], 8)
	binary.LittleEndian.PutUint16(tiff[8:], 2)
	binary.LittleEndian.PutUint16(tiff[10:], 0x112)
	binary.LittleEndian.PutUint16(tiff[12:], 3)
	binary.LittleEndian.PutUint32(tiff[14:], 1)
	binary.LittleEndian.PutUint16(tiff[18:], 6)
	binary.LittleEndian.PutUint16(tiff[22:], 0x8825)
	binary.LittleEndian.PutUint16(tiff[24:], 4)
	binary.LittleEndian.PutUint32(tiff[26:], 1)
	binary.LittleEndian.PutUint32(tiff[30:], 38)
	binary.LittleEndian.PutUint16(tiff[38:], 2)
	binary.LittleEndian.PutUint16(tiff[40:], 1)
	binary.LittleEndian.PutUint16(tiff[42:], 2)
	binary.LittleEndian.PutUint32(tiff[44:], 2)
	copy(tiff[48:], "N\x00")
	binary.LittleEndian.PutUint16(tiff[52:], 2)
	binary.LittleEndian.PutUint16(tiff[54:], 5)
	binary.LittleEndian.PutUint32(tiff[56:], 3)
	binary.LittleEndian.PutUint32(tiff[60:], 68)
	for i := range 3 {
		binary.LittleEndian.PutUint32(tiff[68+i*8:], uint32(i+1))
		binary.LittleEndian.PutUint32(tiff[72+i*8:], 1)
	}
	metadata := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(metadata)+2))
	return append(append(append([]byte{}, b.Bytes()[:2]...), append(segment, metadata...)...), b.Bytes()[2:]...)
}
func companyRow(t *testing.T, runner db.TxRunner, p authz.Principal) (key, ct *string) {
	t.Helper()
	err := runner.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT logo_object_key,logo_content_type FROM app.tenants WHERE id=$1`, p.TenantID).Scan(&key, &ct)
	})
	if err != nil {
		t.Fatal(err)
	}
	return
}
func logoAudit(t *testing.T, runner db.TxRunner, p authz.Principal, action, ct string) {
	t.Helper()
	var data []byte
	var sameTx bool
	err := runner.InTenantTx(context.Background(), p.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT a.data,a.xmin=t.xmin FROM app.audit_log a JOIN app.tenants t ON t.id=a.tenant_id WHERE a.tenant_id=$1 AND a.action=$2 ORDER BY a.occurred_at DESC LIMIT 1`, p.TenantID, action).Scan(&data, &sameTx)
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]any{"content_type": ct}) || !sameTx {
		t.Fatalf("audit=%s sameTx=%v", data, sameTx)
	}
}
func TestLogoServiceValidation(t *testing.T) {
	_, _, runner := newRegistrationServices(t)
	f := newLogoStorage()
	svc := logoService(t, runner, audit.NewRecorder(), f, io.Discard)
	p := companyAdmin(t, svc)
	for _, tc := range []struct {
		name    string
		data    []byte
		ct, ext string
	}{
		{"PNG 1 MiB", pngLogo(t, 10, 10, 1<<20), "image/png", "png"},
		{"exact maximum", pngLogo(t, 10, 10, tenant.LogoMaxBytes), "image/png", "png"},
		{"2000 square", pngLogo(t, 2000, 2000, 0), "image/png", "png"},
		{"JPEG", jpegLogo(t, false), "image/jpeg", "jpg"},
		{"JPEG EXIF", jpegLogo(t, true), "image/jpeg", "jpg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, _ := svc.Get(context.Background(), p)
			out, err := svc.SetLogo(context.Background(), p, tc.data, identity.RequestMeta{})
			if err != nil {
				t.Fatal(err)
			}
			key, ct := companyRow(t, runner, p)
			if key == nil || ct == nil || *ct != tc.ct || !out.HasLogo || !out.UpdatedAt.After(before.UpdatedAt) {
				t.Fatalf("row/out=%+v key=%v ct=%v", out, key, ct)
			}
			id, err := uuid.Parse(strings.TrimSuffix(path.Base(*key), "."+tc.ext))
			if err != nil || id.Version() != 7 || *key != "tenants/"+p.TenantID.String()+"/logo/"+id.String()+"."+tc.ext {
				t.Fatalf("invalid object key %s", *key)
			}
			o := f.objects[*key]
			if o.contentType != tc.ct || !bytes.Equal(o.data, tc.data) {
				t.Fatal("storage changed bytes or type")
			}
			logoAudit(t, runner, p, "tenant.logo_updated", tc.ct)
		})
	}
	huge := pngLogo(t, 10, 10, 0)
	binary.BigEndian.PutUint32(huge[16:], 50000)
	binary.BigEndian.PutUint32(huge[20:], 50000)
	binary.BigEndian.PutUint32(huge[29:], crc32.ChecksumIEEE(huge[12:29]))
	for _, tc := range []struct {
		name string
		data []byte
		err  error
	}{
		{"one extra byte", pngLogo(t, 10, 10, tenant.LogoMaxBytes+1), tenant.ErrLogoTooLarge},
		{"GIF", []byte("GIF89a"), tenant.ErrLogoUnsupportedType},
		{"WebP", []byte("RIFF0000WEBP"), tenant.ErrLogoUnsupportedType},
		{"SVG", []byte(`<svg/>`), tenant.ErrLogoUnsupportedType},
		{"2001 side", pngLogo(t, 2001, 10, 0), tenant.ErrLogoInvalidImage},
		{"huge header", huge, tenant.ErrLogoInvalidImage},
		{"truncated", []byte("\x89PNG\r\n\x1a\n"), tenant.ErrLogoInvalidImage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := len(f.puts)
			before, _ := svc.Get(context.Background(), p)
			_, err := svc.SetLogo(context.Background(), p, tc.data, identity.RequestMeta{})
			if !errors.Is(err, tc.err) {
				t.Fatalf("err=%v want %v", err, tc.err)
			}
			after, _ := svc.Get(context.Background(), p)
			if len(f.puts) != n || !reflect.DeepEqual(before, after) {
				t.Fatal("rejected data reached storage or changed row")
			}
		})
	}
}
func TestLogoReplacementOrderingAndCleanup(t *testing.T) {
	_, _, runner := newRegistrationServices(t)
	f := newLogoStorage()
	var logs bytes.Buffer
	svc := logoService(t, runner, audit.NewRecorder(), f, &logs)
	p := companyAdmin(t, svc)
	if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	old, _ := companyRow(t, runner, p)
	var newKey string
	f.onPut = func(key string) {
		newKey = key
		current, _ := companyRow(t, runner, p)
		if current == nil || *current != *old {
			t.Fatal("new reference committed before Put")
		}
	}
	f.onDelete = func(key string) {
		current, _ := companyRow(t, runner, p)
		if key != *old || current == nil || *current != newKey {
			t.Fatalf("old object deleted before COMMIT: key=%s current=%v new=%s", key, current, newKey)
		}
	}
	if _, err := svc.SetLogo(context.Background(), p, jpegLogo(t, false), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if newKey == *old || len(f.deletes) != 1 {
		t.Fatal("replacement did not delete old/change key")
	}
	f.onPut = nil
	f.onDelete = nil
	old, _ = companyRow(t, runner, p)
	f.deleteErr = objectstore.ErrUnavailable
	if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.objects[*old]; !ok || !strings.Contains(logs.String(), "level=WARN") {
		t.Fatal("old deletion failure was not retained and logged WARN")
	}
}

func TestLogoFailureCompensation(t *testing.T) {
	_, _, runner := newRegistrationServices(t)
	f := newLogoStorage()
	var logs bytes.Buffer
	svc := logoService(t, runner, audit.NewRecorder(), f, &logs)
	p := companyAdmin(t, svc)
	if _, err := svc.SetLogo(context.Background(), p, jpegLogo(t, false), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	original, _ := companyRow(t, runner, p)
	f.puts = nil
	f.deletes = nil
	before, _ := svc.Get(context.Background(), p)
	f.putErr = objectstore.ErrUnavailable
	_, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{})
	if !errors.Is(err, objectstore.ErrUnavailable) {
		t.Fatalf("Put failure=%v", err)
	}
	after, _ := svc.Get(context.Background(), p)
	if !reflect.DeepEqual(before, after) || len(f.deletes) != 0 {
		t.Fatal("Put failure changed row")
	}
	f.putErr = nil
	broken := logoService(t, runner, failedLogoAudit{}, f, &logs)
	_, err = broken.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{})
	if err == nil || len(f.deletes) != 1 || len(f.objects) != 1 {
		t.Fatalf("rollback compensation err=%v deletes=%v", err, f.deletes)
	}
	if _, ok := f.objects[*original]; !ok {
		t.Fatal("rollback deleted original logo")
	}
	after, _ = svc.Get(context.Background(), p)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("rollback changed row")
	}
	f.deleteErr = objectstore.ErrUnavailable
	logs.Reset()
	_, err = broken.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{})
	if err == nil || !strings.Contains(logs.String(), "level=WARN") {
		t.Fatal("failed compensation not logged")
	}
}
func TestLogoGetCacheAndRemove(t *testing.T) {
	_, _, runner := newRegistrationServices(t)
	f := newLogoStorage()
	svc := logoService(t, runner, audit.NewRecorder(), f, io.Discard)
	p := companyAdmin(t, svc)
	if _, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	key, _ := companyRow(t, runner, p)
	etag := `"` + strings.TrimSuffix(path.Base(*key), ".png") + `"`
	for _, tag := range []string{"", "\"old-uuid\""} {
		n := len(f.gets)
		result, err := svc.GetLogo(context.Background(), p, tag)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(result.Body)
		result.Body.Close()
		if err != nil || result.NotModified || result.ETag != etag || !bytes.Equal(data, f.objects[*key].data) || result.ContentType != "image/png" || result.Size != int64(len(data)) || len(f.gets) != n+1 {
			t.Fatalf("GET result=%+v err=%v", result, err)
		}
	}
	n := len(f.gets)
	result, err := svc.GetLogo(context.Background(), p, etag)
	if err != nil || !result.NotModified || result.Body != nil || result.ETag != etag || len(f.gets) != n {
		if result.Body != nil {
			result.Body.Close()
		}
		t.Fatalf("revalidation fetched S3: gets=%d want=%d result=%+v err=%v", len(f.gets), n, result, err)
	}
	other := companyAdmin(t, svc)
	otherData := jpegLogo(t, false)
	if _, err := svc.SetLogo(context.Background(), other, otherData, identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	result, err = svc.GetLogo(context.Background(), other, etag)
	if err != nil || result.NotModified || result.ETag == etag {
		t.Fatalf("other tenant validator=%+v err=%v", result, err)
	}
	otherBody, readErr := io.ReadAll(result.Body)
	result.Body.Close()
	if readErr != nil || !bytes.Equal(otherBody, otherData) || result.ContentType != "image/jpeg" || result.Size != int64(len(otherData)) {
		t.Fatalf("other tenant returned wrong logo bytes/type: result=%+v err=%v", result, readErr)
	}
	before, _ := svc.Get(context.Background(), p)
	f.onDelete = func(deleted string) {
		current, _ := companyRow(t, runner, p)
		if deleted != *key || current != nil {
			t.Fatal("RemoveLogo deleted object before COMMIT")
		}
	}
	if err := svc.RemoveLogo(context.Background(), p, identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	k, ct := companyRow(t, runner, p)
	after, _ := svc.Get(context.Background(), p)
	if k != nil || ct != nil || after.HasLogo || !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatal("logo removal not persisted")
	}
	if _, ok := f.objects[*key]; ok {
		t.Fatal("object not removed")
	}
	logoAudit(t, runner, p, "tenant.logo_removed", "image/png")
	if err := svc.RemoveLogo(context.Background(), p, identity.RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetLogo(context.Background(), p, etag); !errors.Is(err, tenant.ErrLogoNotFound) {
		t.Fatalf("deleted logo=%v", err)
	}
}

// Observe actual SQL on the real transaction, including the recorder's INSERT.
// DD-40 protects FK lock order; merely sharing xmin does not prove write order.
type tenantWriteRunner struct {
	db.TxRunner
	writes []string
}
type tenantWriteTx struct {
	db.Tx
	runner *tenantWriteRunner
}

func (r *tenantWriteRunner) InTenantTx(ctx context.Context, id uuid.UUID, fn func(context.Context, db.Tx) error) error {
	return r.TxRunner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error { return fn(ctx, &tenantWriteTx{Tx: tx, runner: r}) })
}
func (tx *tenantWriteTx) observe(sql string) {
	for strings.HasPrefix(strings.TrimSpace(sql), "--") {
		_, tail, ok := strings.Cut(sql, "\n")
		if !ok {
			return
		}
		sql = tail
	}
	upper := strings.ToUpper(strings.TrimSpace(sql))
	if strings.HasPrefix(upper, "UPDATE ") || strings.HasPrefix(upper, "INSERT ") || strings.HasPrefix(upper, "DELETE ") {
		tx.runner.writes = append(tx.runner.writes, upper)
	}
}
func (tx *tenantWriteTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.observe(sql)
	return tx.Tx.Exec(ctx, sql, args...)
}
func (tx *tenantWriteTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.observe(sql)
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx *tenantWriteTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.observe(sql)
	return tx.Tx.Query(ctx, sql, args...)
}
func TestTenantWritesPrecedeAudit(t *testing.T) {
	_, _, base := newRegistrationServices(t)
	runner := &tenantWriteRunner{TxRunner: base}
	f := newLogoStorage()
	svc := logoService(t, runner, audit.NewRecorder(), f, io.Discard)
	p := companyAdmin(t, svc)
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"Update", func() error {
			_, err := svc.Update(context.Background(), p, patch(t, `{"name":"Changed"}`), identity.RequestMeta{})
			return err
		}},
		{"SetLogo", func() error {
			_, err := svc.SetLogo(context.Background(), p, pngLogo(t, 10, 10, 0), identity.RequestMeta{})
			return err
		}},
		{"RemoveLogo", func() error { return svc.RemoveLogo(context.Background(), p, identity.RequestMeta{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner.writes = nil
			if err := tc.run(); err != nil {
				t.Fatal(err)
			}
			if len(runner.writes) != 2 || !strings.HasPrefix(runner.writes[0], "UPDATE APP.TENANTS SET") || !strings.HasPrefix(runner.writes[1], "INSERT INTO APP.AUDIT_LOG") {
				t.Fatalf("DD-40 write order=%v", runner.writes)
			}
		})
	}
}
