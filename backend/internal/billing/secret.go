package billing

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

func deriveKEK(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	return sum[:]
}

func encryptSecret(kek, plaintext []byte) (string, error) {
	if len(kek) == 0 {
		return "", fmt.Errorf("missing encryption key")
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func decryptSecret(kek []byte, enc string) ([]byte, error) {
	if enc == "" {
		return nil, fmt.Errorf("empty ciphertext")
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

// dataKey resolves the master secret every at-rest KEK in this package is
// derived from: DATA_ENCRYPTION_KEY when the deployment sets one, otherwise
// the JWT secret, which is what sealed the PANs and Telegram credentials
// already in the database. Rotating JWT_SECRET is only safe once DataSecret
// is set — until then this fallback is load-bearing for existing rows.
func (s Service) dataKey() []byte {
	if len(s.DataSecret) > 0 {
		return s.DataSecret
	}
	return s.Secret
}

func (s Service) kek() []byte {
	key := s.dataKey()
	if len(key) == 0 {
		return nil
	}
	return deriveKEK(key)
}
