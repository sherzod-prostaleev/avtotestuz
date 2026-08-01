package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTRoundtrip(t *testing.T) {
	secret := []byte("test-secret")
	id := uuid.New()
	tok, err := IssueAccess(secret, id, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseAccess(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.ProfileID != id || claims.Role != "user" {
		t.Fatalf("claims=%+v", claims)
	}
	if _, err := ParseAccess([]byte("wrong"), tok); err == nil {
		t.Fatal("wrong secret must fail")
	}
	expired, _ := IssueAccess(secret, id, "user", -time.Second)
	if _, err := ParseAccess(secret, expired); err == nil {
		t.Fatal("expired must fail")
	}
	if _, err := ParseAccess(secret, "not.a.token"); err == nil {
		t.Fatal("garbage must fail")
	}
}

func TestRefreshToken(t *testing.T) {
	raw, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 43 {
		t.Fatalf("raw too short: %d", len(raw))
	}
	h := HashToken(raw)
	if len(h) != 64 || h != HashToken(raw) {
		t.Fatalf("hash not deterministic 64-hex: %q", h)
	}
	second, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if second == raw {
		t.Fatal("tokens must be unique")
	}
}
