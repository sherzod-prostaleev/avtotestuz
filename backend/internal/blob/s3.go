package blob

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3 struct {
	client *minio.Client
	bucket string
}

func NewS3(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*S3, error) {
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return &S3{client: c, bucket: bucket}, nil
}

// NewS3FromEnv opens bucket on the MinIO the API is configured for:
// MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY and MINIO_USE_SSL, with
// the local docker-compose values as defaults. Every bucket the API talks to
// lives on that one server, so the variables are read in one place.
func NewS3FromEnv(bucket string) (*S3, error) {
	endpoint := envOr("MINIO_ENDPOINT", "localhost:9000")
	access := envOr("MINIO_ACCESS_KEY", "avtotest")
	secret := envOr("MINIO_SECRET_KEY", "avtotest123")
	if strings.TrimSpace(access) == "" || strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("MINIO_ACCESS_KEY and MINIO_SECRET_KEY are required")
	}
	ssl := strings.TrimSpace(os.Getenv("MINIO_USE_SSL"))
	if ssl == "" {
		ssl = "false"
	}
	secure, err := strconv.ParseBool(ssl)
	if err != nil {
		return nil, fmt.Errorf("invalid MINIO_USE_SSL: %w", err)
	}
	return NewS3(endpoint, access, secret, bucket, secure)
}

// envOr is os.Getenv with a default for an unset or empty variable.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (s *S3) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data),
		int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = obj.Close() }()
	info, err := obj.Stat()
	if err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.Code == "NoSuchKey" || resp.Code == "NoSuchObject" {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", err
	}
	return data, info.ContentType, nil
}

// Delete removes key. S3 treats deleting a missing key as success.
func (s *S3) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3) Health(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("blob bucket %q does not exist", s.bucket)
	}
	return nil
}
