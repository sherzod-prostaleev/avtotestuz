package blob

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDirPut(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalDir(dir)
	if err := s.Put(context.Background(), "images/abc.png", "image/png", []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "images", "abc.png"))
	if err != nil || len(data) != 3 {
		t.Fatalf("read back: %v len=%d", err, len(data))
	}
	// idempotent overwrite
	if err := s.Put(context.Background(), "images/abc.png", "image/png", []byte{9}); err != nil {
		t.Fatal(err)
	}
}
