// Package bot implements the Telegram bot foundation: a secure account-link
// flow between an authenticated web session and a Telegram user, plus a
// minimal command dispatcher (/start, /link, /status). See
// docs/superpowers/specs/2026-07-25-m4-06-telegram-bot-design.md.
package bot

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// newOpaqueToken returns a 256-bit random token, base64url-encoded without
// padding. That alphabet ([A-Za-z0-9_-]) is exactly what Telegram allows in
// a /start deep-link payload, and the length (43 chars) is well under its
// 64-byte limit. Shaped like auth.NewRefreshToken but kept local: a link
// token is a different domain concept (single profile-scoped redemption,
// not a rotating session credential) and naming it a "refresh token" would
// be misleading at every call site.
func newOpaqueToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// hashToken is how link tokens are stored — never the raw value, matching
// refresh_token's HashToken convention.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
