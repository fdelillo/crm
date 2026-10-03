package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrNotFound is Get or Get's Stat finding no object at the given key (classifyS3).
var ErrNotFound = errors.New("objectstore: not found")

// ErrUnavailable is any S3 failure that is not ErrNotFound: endpoint down, a missing bucket
// (configuration, not a missing object), credentials, and the like (plan §9.1: mapped to 503).
var ErrUnavailable = errors.New("objectstore: unavailable")

// ObjectInfo is what Get reports about the object alongside its content.
type ObjectInfo struct {
	Size        int64
	ContentType string
}

// ObjectStorage is the S3-compatible storage port (ADR-011); the adapter below is the only
// implementation. Keys are the caller's full object path (tenants/<id>/logo/..., plan §4.4).
type ObjectStorage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Get returns ErrNotFound if key does not exist. The caller must Close the reader.
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	// Delete is idempotent: deleting a key that does not exist is not an error.
	Delete(ctx context.Context, key string) error
}

// S3Config is the adapter's connection settings (S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY,
// S3_SECRET_KEY, S3_USE_SSL).
type S3Config struct {
	Endpoint, Bucket, AccessKey, SecretKey string
	UseSSL                                 bool
}

type s3Store struct {
	client *minio.Client
	bucket string
}

// NewS3 validates cfg and builds the client. It does not check the bucket exists: a missing
// bucket surfaces as ErrUnavailable on the first operation (classifyS3).
func NewS3(cfg S3Config) (ObjectStorage, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("objectstore: S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY and S3_SECRET_KEY are required")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("objectstore: invalid S3 configuration: %w", err)
	}
	return &s3Store{client: client, bucket: cfg.Bucket}, nil
}

func (s *s3Store) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	return classifyS3(ctx, err)
}

func (s *s3Store) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, classifyS3(ctx, err)
	}
	info, err := object.Stat() // GetObject is lazy: Stat forces the request and detects missing keys.
	if err != nil {
		_ = object.Close()
		return nil, ObjectInfo{}, classifyS3(ctx, err)
	}
	return object, ObjectInfo{Size: info.Size, ContentType: info.ContentType}, nil
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err == nil || isMissingS3(err) {
		return nil
	}
	return classifyS3(ctx, err)
}

// classifyS3 turns a minio-go error into ErrNotFound (only the key is missing) or ErrUnavailable
// (anything else, including a missing bucket: plan §9.1 treats that as a configuration problem,
// not a missing object). err is kept in the chain (M2 of the PR #8 review): dropping it, as the
// previous %w with no verb for err did, loses the diagnosis (which S3 error, which status code).
func classifyS3(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if isMissingS3(err) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return fmt.Errorf("%w: %w", ErrUnavailable, err)
}

// isMissingS3 is true only for NoSuchKey: minio-go's StatusCode is 404 for NoSuchBucket too (the
// bucket itself missing, a configuration problem per plan §9.1), and NoSuchKey is the sole code
// Put/Get/Delete use for "there is no object at that key" (M2 of the PR #8 review).
func isMissingS3(err error) bool {
	return minio.ToErrorResponse(err).Code == minio.NoSuchKey
}
