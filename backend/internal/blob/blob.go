// Package blob abstracts binary storage (MinIO/S3 in prod, local dir in tests).
package blob

import (
	"context"
	"os"
	"path/filepath"
)

type Store interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	// Get returns object bytes. contentType may be empty when unknown (local dir).
	Get(ctx context.Context, key string) (data []byte, contentType string, err error)
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
		return nil, "", err
	}
	return data, "", nil
}
