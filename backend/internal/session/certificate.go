package session

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

// newCertificateShareCode returns a short, URL-safe public id (10 chars).
func newCertificateShareCode() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// Crockford-ish: std base32 without padding, trimmed to 10.
	code := strings.TrimRight(base32.StdEncoding.EncodeToString(b[:]), "=")
	if len(code) > 10 {
		code = code[:10]
	}
	return strings.ToLower(code), nil
}
