package billing

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const panCipherPrefix = "enc:v1:"

func (s Service) panKEK() ([]byte, error) {
	if len(s.Secret) == 0 {
		return nil, fmt.Errorf("PAN encryption key is not configured")
	}
	// Domain separation prevents ciphertext from sharing a key stream domain
	// with Telegram credentials even though both derive from the deployment
	// master secret.
	sum := sha256.Sum256(append([]byte("drivergo:pan:v1:"), s.Secret...))
	return sum[:], nil
}

// EncryptPAN normalizes and encrypts a card number. The last four digits stay
// outside the ciphertext solely for masked display and operational matching;
// the full PAN is never present in the stored value.
func (s Service) EncryptPAN(raw string) (stored, last4 string, err error) {
	digits, last4, err := NormalizeCardPAN(raw)
	if err != nil {
		return "", "", err
	}
	kek, err := s.panKEK()
	if err != nil {
		return "", "", err
	}
	ciphertext, err := encryptSecret(kek, []byte(digits))
	if err != nil {
		return "", "", err
	}
	return panCipherPrefix + last4 + ":" + ciphertext, last4, nil
}

// DecryptPAN supports legacy plaintext only during the data-preserving rollout.
// encryptpan rewrites those rows immediately after the new binary is deployed.
func (s Service) DecryptPAN(stored string) (string, error) {
	if !strings.HasPrefix(stored, panCipherPrefix) {
		digits, _, err := NormalizeCardPAN(stored)
		return digits, err
	}
	parts := strings.SplitN(strings.TrimPrefix(stored, panCipherPrefix), ":", 2)
	if len(parts) != 2 || len(parts[0]) != 4 || parts[1] == "" {
		return "", fmt.Errorf("invalid PAN ciphertext envelope")
	}
	kek, err := s.panKEK()
	if err != nil {
		return "", err
	}
	plain, err := decryptSecret(kek, parts[1])
	if err != nil {
		return "", err
	}
	digits, last4, err := NormalizeCardPAN(string(plain))
	if err != nil || last4 != parts[0] {
		return "", fmt.Errorf("PAN ciphertext integrity check failed")
	}
	return digits, nil
}

func PANNeedsEncryption(stored string) bool {
	return !strings.HasPrefix(stored, panCipherPrefix)
}

// MaskStoredPAN masks either an encrypted envelope or a legacy plaintext PAN.
func MaskStoredPAN(stored string) string {
	if strings.HasPrefix(stored, panCipherPrefix) {
		parts := strings.SplitN(strings.TrimPrefix(stored, panCipherPrefix), ":", 2)
		if len(parts) == 2 && len(parts[0]) == 4 {
			return MaskCardNumber(parts[0])
		}
		return "****"
	}
	return MaskCardNumber(stored)
}
