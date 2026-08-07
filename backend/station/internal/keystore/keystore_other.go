//go:build !windows

package keystore

// seal and unseal are identity functions off Windows. This build is for
// development only: without DPAPI the key file is copyable, so a non-Windows
// agent must never be handed to a school.
func seal(plain []byte) ([]byte, error) { return plain, nil }

func unseal(sealed []byte) ([]byte, error) { return sealed, nil }
