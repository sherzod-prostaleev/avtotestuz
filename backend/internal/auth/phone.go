// Package auth owns authentication: phone normalization, password (and legacy
// OTP) flows, JWT access tokens, refresh tokens, and rate limits.
package auth

import (
	"fmt"
	"strings"
)

// NormalizePhone accepts UZ numbers in loose formats and returns +998XXXXXXXXX.
func NormalizePhone(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	switch {
	case len(d) == 12 && strings.HasPrefix(d, "998"):
	case len(d) == 9:
		d = "998" + d
	default:
		return "", fmt.Errorf("invalid uz phone: %q", raw)
	}
	return "+" + d, nil
}
