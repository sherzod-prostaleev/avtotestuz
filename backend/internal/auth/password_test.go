package auth

import (
	"errors"
	"testing"
)

func TestHashPasswordAndCheck(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || hash == "secret123" {
		t.Fatal("expected bcrypt hash, not plaintext")
	}
	if !CheckPassword(hash, "secret123") {
		t.Fatal("expected matching password to verify")
	}
	if CheckPassword(hash, "wrongpass") {
		t.Fatal("expected mismatch to fail")
	}
}

func TestHashPasswordRejectsShort(t *testing.T) {
	if _, err := HashPassword("short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("want ErrWeakPassword, got %v", err)
	}
}
