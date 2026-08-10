package blob

import (
	"context"
	"errors"
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

func TestFallbackStoreReadsLegacyAndDualWrites(t *testing.T) {
	ctx := context.Background()
	primary := NewLocalDir(t.TempDir())
	legacy := NewLocalDir(t.TempDir())
	store := &FallbackStore{Primary: primary, Legacy: legacy}

	if err := legacy.Put(ctx, "support/old.txt", "text/plain", []byte("legacy")); err != nil {
		t.Fatal(err)
	}
	data, _, err := store.Get(ctx, "support/old.txt")
	if err != nil || string(data) != "legacy" {
		t.Fatalf("legacy read: data=%q err=%v", data, err)
	}

	if err := store.Put(ctx, "support/new.txt", "text/plain", []byte("private")); err != nil {
		t.Fatal(err)
	}
	data, _, err = primary.Get(ctx, "support/new.txt")
	if err != nil || string(data) != "private" {
		t.Fatalf("primary write: data=%q err=%v", data, err)
	}
	data, _, err = legacy.Get(ctx, "support/new.txt")
	if err != nil || string(data) != "private" {
		t.Fatalf("legacy compatibility write: data=%q err=%v", data, err)
	}
}

func TestFallbackStoreFailsIfLegacyMirrorFails(t *testing.T) {
	want := errors.New("legacy unavailable")
	store := &FallbackStore{Primary: NewLocalDir(t.TempDir()), Legacy: failingStore{err: want}}
	if err := store.Put(context.Background(), "support/file.txt", "text/plain", []byte("x")); !errors.Is(err, want) {
		t.Fatalf("Put error=%v want legacy failure", err)
	}
}

type failingStore struct{ err error }

func (s failingStore) Put(context.Context, string, string, []byte) error { return s.err }
func (s failingStore) Get(context.Context, string) ([]byte, string, error) {
	return nil, "", s.err
}
func (s failingStore) Health(context.Context) error { return s.err }

func TestFallbackStoreDoesNotMaskPrimaryOutage(t *testing.T) {
	want := errors.New("primary unavailable")
	legacy := NewLocalDir(t.TempDir())
	if err := legacy.Put(context.Background(), "support/file.txt", "text/plain", []byte("legacy")); err != nil {
		t.Fatal(err)
	}
	store := &FallbackStore{Primary: failingStore{err: want}, Legacy: legacy}
	if _, _, err := store.Get(context.Background(), "support/file.txt"); !errors.Is(err, want) {
		t.Fatalf("Get error=%v want primary outage", err)
	}
}
