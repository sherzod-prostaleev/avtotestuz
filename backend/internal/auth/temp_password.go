package auth

import (
	"crypto/rand"
	"math/big"
)

// Temporary passwords are long enough for HashPassword (min 8) and avoid
// ambiguous characters (0/O, 1/l/I) so support can read them aloud once.
const tempPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%"

const tempPasswordLen = 16

// RandomTempPassword returns a cryptographically secure temporary password.
// Callers must bcrypt-hash before persistence and show plaintext at most once.
func RandomTempPassword() (string, error) {
	max := big.NewInt(int64(len(tempPasswordAlphabet)))
	out := make([]byte, tempPasswordLen)
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = tempPasswordAlphabet[n.Int64()]
	}
	return string(out), nil
}
