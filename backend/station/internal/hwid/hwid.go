// Package hwid derives a stable identifier for the physical machine.
//
// It is the second half of the binding: even if the sealed key were somehow
// extracted, the server refuses a token whose hwid does not match the one
// recorded at enrollment, so a cloned disk image authenticates nowhere.
package hwid

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrNoIdentity means no machine-specific value could be read.
var ErrNoIdentity = errors.New("no stable hardware identity available")

// Collect returns a 64-char lowercase sha256 hex digest of the machine's
// identifying values.
func Collect() (string, error) {
	parts, err := rawParts()
	if err != nil {
		return "", err
	}
	kept := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, strings.ToLower(p))
		}
	}
	if len(kept) == 0 {
		return "", ErrNoIdentity
	}
	sum := sha256.Sum256([]byte(strings.Join(kept, "|")))
	return hex.EncodeToString(sum[:]), nil
}
