package supportchat

import (
	"os"

	"avtotest.uz/backend/internal/blob"
)

// OpenBlobStore builds MinIO/S3 storage for chat attachments.
// Falls back to a local directory when MINIO_ENDPOINT is empty (tests).
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
	bucket := os.Getenv("MINIO_BUCKET")
	if bucket == "" {
		bucket = "media"
	}
	if os.Getenv("SUPPORTCHAT_LOCAL_BLOBS") != "" && localRoot != "" {
		return blob.NewLocalDir(localRoot), nil
	}
	return blob.NewS3(endpoint, access, secret, bucket, false)
}
