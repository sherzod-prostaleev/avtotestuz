package billing

import (
	"strings"
	"testing"
)

func TestPANEncryptionRoundTripAndMask(t *testing.T) {
	svc := Service{Secret: []byte("test-pan-master-secret-at-least-32-bytes")}
	stored, last4, err := svc.EncryptPAN("8600 1234 5678 9012")
	if err != nil {
		t.Fatal(err)
	}
	if last4 != "9012" || !strings.HasPrefix(stored, panCipherPrefix+"9012:") {
		t.Fatalf("stored=%q last4=%q", stored, last4)
	}
	if strings.Contains(stored, "8600123456789012") {
		t.Fatal("ciphertext envelope leaked plaintext PAN")
	}
	plain, err := svc.DecryptPAN(stored)
	if err != nil || plain != "8600123456789012" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
	if got := MaskStoredPAN(stored); got != "**** **** **** 9012" {
		t.Fatalf("mask=%q", got)
	}
}

// TestPANKeyFallsBackToJWTSecret is the data-preservation guarantee behind
// DATA_ENCRYPTION_KEY. Every stored PAN and Telegram credential in production
// was sealed under JWT_SECRET, so a Service without a separate data key — the
// state of every deployment that has not set one, and of any Service value the
// integrator has not rewired yet — must still decrypt them. Getting this wrong
// makes card data unreadable with no way back.
func TestPANKeyFallsBackToJWTSecret(t *testing.T) {
	jwtSecret := []byte("test-pan-master-secret-at-least-32-bytes")

	// Ciphertext exactly as a pre-split deployment wrote it.
	legacy, _, err := (Service{Secret: jwtSecret}).EncryptPAN("8600 1234 5678 9012")
	if err != nil {
		t.Fatal(err)
	}
	legacyTG, err := encryptSecret((Service{Secret: jwtSecret}).kek(), []byte("telethon-session"))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		svc  Service
	}{
		{"data key not wired yet", Service{Secret: jwtSecret}},
		{"split adopted with the current JWT secret", Service{Secret: jwtSecret, DataSecret: jwtSecret}},
		{"JWT secret rotated afterwards", Service{Secret: []byte("a-completely-rotated-jwt-secret-32-bytes"), DataSecret: jwtSecret}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plain, err := tc.svc.DecryptPAN(legacy)
			if err != nil || plain != "8600123456789012" {
				t.Fatalf("PAN plain=%q err=%v", plain, err)
			}
			session, err := decryptSecret(tc.svc.kek(), legacyTG)
			if err != nil || string(session) != "telethon-session" {
				t.Fatalf("telegram session=%q err=%v", session, err)
			}
		})
	}

	// The separation itself: a genuinely different data key must not open
	// JWT-secret ciphertext, which is what makes rotating JWT_SECRET safe.
	separate := Service{Secret: jwtSecret, DataSecret: []byte("a-separate-data-encryption-key-32-bytes")}
	if _, err := separate.DecryptPAN(legacy); err == nil {
		t.Fatal("a distinct data key must not decrypt JWT-secret PAN ciphertext")
	}
	stored, _, err := separate.EncryptPAN("8600123456789012")
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := separate.DecryptPAN(stored); err != nil || plain != "8600123456789012" {
		t.Fatalf("round trip under the data key: plain=%q err=%v", plain, err)
	}
}

func TestDecryptPANAcceptsLegacyPlaintextForRollingMigration(t *testing.T) {
	svc := Service{Secret: []byte("test-pan-master-secret-at-least-32-bytes")}
	plain, err := svc.DecryptPAN("8600123456789012")
	if err != nil || plain != "8600123456789012" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
}
