package supportchat

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"avtotest.uz/backend/internal/blob"
)

// OpenBlobStore builds private MinIO/S3 storage for chat attachments. New
// objects are always written to the resolved private bucket. Resolution keeps
// the pre-split MINIO_BUCKET contract compatible: MINIO_SUPPORT_BUCKET, then
// MINIO_BUCKET, then support-attachments. During migration, reads may fall back
// to MINIO_LEGACY_SUPPORT_BUCKET; that legacy support/ prefix must remain
// non-anonymous in MinIO policy.
func OpenBlobStore(localRoot string) (blob.Store, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	access := os.Getenv("MINIO_ACCESS_KEY")
	if access == "" {
		access = "avtotest"
	}
	secret := os.Getenv("MINIO_SECRET_KEY")
	if secret == "" {
		secret = "avtotest123"
	}
	if os.Getenv("SUPPORTCHAT_LOCAL_BLOBS") != "" && localRoot != "" {
		return blob.NewLocalDir(localRoot), nil
	}
	if strings.TrimSpace(access) == "" || strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("MINIO_ACCESS_KEY and MINIO_SECRET_KEY are required")
	}
	secure, err := strconv.ParseBool(envDefault("MINIO_USE_SSL", "false"))
	if err != nil {
		return nil, fmt.Errorf("invalid MINIO_USE_SSL: %w", err)
	}
	privateBucket := supportBucket()
	primary, err := blob.NewS3(endpoint, access, secret, privateBucket, secure)
	if err != nil {
		return nil, err
	}
	legacyBucket := envDefault("MINIO_LEGACY_SUPPORT_BUCKET", "media")
	if legacyBucket == privateBucket {
		return primary, nil
	}
	legacy, err := blob.NewS3(endpoint, access, secret, legacyBucket, secure)
	if err != nil {
		return nil, err
	}
	return &blob.FallbackStore{Primary: primary, Legacy: legacy}, nil
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func supportBucket() string {
	if value := strings.TrimSpace(os.Getenv("MINIO_SUPPORT_BUCKET")); value != "" {
		return value
	}
	return envDefault("MINIO_BUCKET", "support-attachments")
}
