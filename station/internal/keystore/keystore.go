// Package keystore holds the station's Ed25519 private key.
//
// On Windows the key is sealed with DPAPI at machine scope, so the file is
// undecryptable on any other machine — copying it to a home PC yields
// nothing. The non-Windows implementation is a plain 0600 file for
// development only and refuses to look like the real thing.
package keystore

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
)

// Store loads or creates the station key.
type Store interface {
	Load() (ed25519.PrivateKey, error)
	Save(ed25519.PrivateKey) error
}

// ErrCorruptKey means the stored key could not be unsealed — usually because
// the file was copied from a different machine.
var ErrCorruptKey = errors.New("station key could not be unsealed on this machine")

type fileStore struct {
	path string
	seal func([]byte) ([]byte, error)
	open func([]byte) ([]byte, error)
}

// Open returns the platform keystore rooted at dir, creating dir if needed.
func Open(dir string) (Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &fileStore{
		path: filepath.Join(dir, "station.key"),
		seal: seal,
		open: unseal,
	}, nil
}

// Load returns the existing key, generating and persisting one on first run.
func (s *fileStore) Load() (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		plain, err := s.open(raw)
		if err != nil {
			return nil, ErrCorruptKey
		}
		if len(plain) != ed25519.PrivateKeySize {
			return nil, ErrCorruptKey
		}
		return ed25519.PrivateKey(plain), nil
	case errors.Is(err, os.ErrNotExist):
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		if err := s.Save(priv); err != nil {
			return nil, err
		}
		return priv, nil
	default:
		return nil, err
	}
}

// Save seals and writes the key.
func (s *fileStore) Save(priv ed25519.PrivateKey) error {
	sealed, err := s.seal(priv)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, sealed, 0o600)
}
