package objectstore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

// M2 of the PR #8 review: NoSuchBucket must not be treated as "the key is missing" (it is a
// configuration problem, 503), and the wrapped message must not repeat the "objectstore:" prefix
// that ErrNotFound/ErrUnavailable already carry.
func TestClassifyS3(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{"missing key", minio.ErrorResponse{Code: "NoSuchKey", StatusCode: 404}, ErrNotFound},
		{"missing bucket", minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: 404}, ErrUnavailable},
		{"other S3 error", minio.ErrorResponse{Code: "AccessDenied", StatusCode: 403}, ErrUnavailable},
		{"not an S3 error", errors.New("dial tcp: connection refused"), ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyS3(context.Background(), tc.err)
			if !errors.Is(got, tc.want) {
				t.Fatalf("classifyS3(%v) = %v, want %v", tc.err, got, tc.want)
			}
			if strings.Contains(got.Error(), "objectstore: objectstore:") {
				t.Fatalf("duplicated prefix: %q", got.Error())
			}
			if !strings.Contains(got.Error(), tc.err.Error()) {
				t.Fatalf("lost the cause: %q does not contain %q", got.Error(), tc.err.Error())
			}
		})
	}
	if err := classifyS3(context.Background(), nil); err != nil {
		t.Fatalf("nil error = %v", err)
	}
}

func TestClassifyS3PrefersContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := classifyS3(ctx, minio.ErrorResponse{Code: "NoSuchKey"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
