package supportchat

import (
	"os"
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
	if os.Getenv("SUPPORTCHAT_LOCAL_BLOBS") != "" && localRoot != "" {
		return blob.NewLocalDir(localRoot), nil
	}
	privateBucket := supportBucket()
	primary, err := blob.NewS3FromEnv(privateBucket)
	if err != nil {
		return nil, err
	}
	legacyBucket := envDefault("MINIO_LEGACY_SUPPORT_BUCKET", "media")
	if legacyBucket == privateBucket {
		return primary, nil
	}
	legacy, err := blob.NewS3FromEnv(legacyBucket)
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
