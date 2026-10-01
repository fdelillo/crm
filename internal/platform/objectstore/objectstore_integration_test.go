//go:build integration

package objectstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestS3AdapterAgainstMinIO(t *testing.T) {
	const accessKey, secretKey, bucket = "minio_dev", "minio_dev_secret", "crm-test"
	// The original quay.io/minio/minio image currently returns 401. This digest contains
	// the open-source MinIO server (2025-05-24), repackaged by Bitnami for test use.
	container, err := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "bitnamilegacy/minio@sha256:451fe6858cb770cc9d0e77ba811ce287420f781c7c1b806a386f6896471a349c",
			Env:          map[string]string{"MINIO_ROOT_USER": accessKey, "MINIO_ROOT_PASSWORD": secretKey},
			ExposedPorts: []string{"9000/tcp"},
			WaitingFor:   wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container, testcontainers.StopTimeout(0)) })
	host, err := container.Host(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(context.Background(), "9000/tcp")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := net.JoinHostPort(host, port.Port())
	setup, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := setup.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}

	store, err := NewS3(S3Config{Endpoint: endpoint, Bucket: bucket, AccessKey: accessKey, SecretKey: secretKey})
	if err != nil {
		t.Fatal(err)
	}
	key := "tenants/t1/logo/example.png"
	content := []byte("image bytes")
	if err := store.Put(context.Background(), key, bytes.NewReader(content), int64(len(content)), "image/png"); err != nil {
		t.Fatal(err)
	}
	object, info, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer object.Close()
	got, err := io.ReadAll(object)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) || info.Size != int64(len(content)) || info.ContentType != "image/png" {
		t.Fatalf("content=%q info=%+v", got, info)
	}
	if err := store.Delete(context.Background(), "missing"); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(context.Background(), key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted object error=%v", err)
	}
	if _, _, err := store.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing error=%v", err)
	}

	unavailable, err := NewS3(S3Config{Endpoint: "127.0.0.1:1", Bucket: bucket, AccessKey: accessKey, SecretKey: secretKey})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err = unavailable.Put(ctx, "x", strings.NewReader("x"), 1, "text/plain")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("endpoint down error=%v", err)
	}
}
