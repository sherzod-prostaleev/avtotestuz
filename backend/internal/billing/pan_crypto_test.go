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

func TestDecryptPANAcceptsLegacyPlaintextForRollingMigration(t *testing.T) {
	svc := Service{Secret: []byte("test-pan-master-secret-at-least-32-bytes")}
	plain, err := svc.DecryptPAN("8600123456789012")
	if err != nil || plain != "8600123456789012" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
}
