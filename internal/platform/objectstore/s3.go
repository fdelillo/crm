package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrNotFound = errors.New("objectstore: not found")
var ErrUnavailable = errors.New("objectstore: unavailable")

type ObjectInfo struct {
	Size        int64
	ContentType string
}
type ObjectStorage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
}
type S3Config struct {
	Endpoint, Bucket, AccessKey, SecretKey string
	UseSSL                                 bool
}
type s3Store struct {
	client *minio.Client
	bucket string
}

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
func classifyS3(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if isMissingS3(err) {
		return fmt.Errorf("objectstore: %w", ErrNotFound)
	}
	return fmt.Errorf("objectstore: S3 request failed: %w", ErrUnavailable)
}
func isMissingS3(err error) bool {
	response := minio.ToErrorResponse(err)
	return response.Code == "NoSuchKey" || response.Code == "NoSuchObject" || response.StatusCode == 404
}
