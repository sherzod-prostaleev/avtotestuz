package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// GenerateCode returns a 6-digit OTP code from crypto/rand.
func GenerateCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

// HashCode stores as "salthex$sha256hex(salt||code)" — salted so identical
// codes never share a digest.
func HashCode(code string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	sum := sha256.Sum256(append(salt, []byte(code)...))
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(sum[:])
}

// VerifyCode compares in constant time.
func VerifyCode(stored, code string) bool {
	parts := strings.SplitN(stored, "$", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append(salt, []byte(code)...))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(parts[1])) == 1
}

// crockford excludes I/L/O/U to avoid visual ambiguity in shared codes.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewReferralCode returns an 8-char human-friendly unique-ish code;
// callers must retry on DB unique violation.
func NewReferralCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 8)
	for i, v := range b {
		out[i] = crockford[int(v)%len(crockford)]
	}
	return string(out)
}
