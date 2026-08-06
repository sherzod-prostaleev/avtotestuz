package keystore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"avtotest.uz/station/internal/keystore"
)

// TestLoadRejectsEmptyKeyFileWithoutPanicking covers an interrupted Save (a
// crash mid-write, a full disk, an AV quarantine) that leaves a zero-byte
// station.key on disk. The platform seal/unseal implementations index their
// input unconditionally, so Load must reject an empty file before handing it
// to them rather than let it panic the whole agent on an unattended machine.
func TestLoadRejectsEmptyKeyFileWithoutPanicking(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "station.key"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ks.Load()
	if !errors.Is(err, keystore.ErrCorruptKey) {
		t.Fatalf("Load() err = %v, want ErrCorruptKey", err)
	}
}
