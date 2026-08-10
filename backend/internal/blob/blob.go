// Package blob abstracts binary storage (MinIO/S3 in prod, local dir in tests).
package blob

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound lets composite stores distinguish a missing legacy object from
// an unavailable object store. Only missing objects may fall back to another
// bucket; outages and permission errors must remain visible.
var ErrNotFound = errors.New("blob not found")

type Store interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	// Get returns object bytes. contentType may be empty when unknown (local dir).
	Get(ctx context.Context, key string) (data []byte, contentType string, err error)
	// Health verifies that the store required for new writes is reachable.
	Health(ctx context.Context) error
}

type LocalDir struct{ root string }

func NewLocalDir(root string) *LocalDir { return &LocalDir{root: root} }

func (l *LocalDir) Put(_ context.Context, key, _ string, data []byte) error {
	path := filepath.Join(l.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (l *LocalDir) Get(_ context.Context, key string) ([]byte, string, error) {
	path := filepath.Join(l.root, filepath.FromSlash(key))
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	return data, "", nil
}

func (l *LocalDir) Health(_ context.Context) error {
	return os.MkdirAll(l.root, 0o755)
}

// FallbackStore dual-writes during the migration window. Reads fall back to
// Legacy only when an object is genuinely absent from Primary, so rollback to
// the pre-migration API cannot strand objects created by the candidate.
type FallbackStore struct {
	Primary Store
	Legacy  Store
}

func (s *FallbackStore) Put(ctx context.Context, key, contentType string, data []byte) error {
	if err := s.Primary.Put(ctx, key, contentType, data); err != nil {
		return err
	}
	if s.Legacy != nil {
		if err := s.Legacy.Put(ctx, key, contentType, data); err != nil {
			return fmt.Errorf("legacy compatibility write: %w", err)
		}
	}
	return nil
}

func (s *FallbackStore) Get(ctx context.Context, key string) ([]byte, string, error) {
	data, contentType, err := s.Primary.Get(ctx, key)
	if !errors.Is(err, ErrNotFound) || s.Legacy == nil {
		return data, contentType, err
	}
	return s.Legacy.Get(ctx, key)
}

func (s *FallbackStore) Health(ctx context.Context) error {
	return s.Primary.Health(ctx)
}
