package supportchat

import "testing"

func TestSupportBucketCompatibilityOrder(t *testing.T) {
	t.Setenv("MINIO_SUPPORT_BUCKET", "")
	t.Setenv("MINIO_BUCKET", "")
	if got := supportBucket(); got != "support-attachments" {
		t.Fatalf("default bucket=%q", got)
	}

	t.Setenv("MINIO_BUCKET", "existing-private")
	if got := supportBucket(); got != "existing-private" {
		t.Fatalf("legacy MINIO_BUCKET=%q", got)
	}

	t.Setenv("MINIO_SUPPORT_BUCKET", "explicit-private")
	if got := supportBucket(); got != "explicit-private" {
		t.Fatalf("MINIO_SUPPORT_BUCKET=%q", got)
	}
}
