package admin

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

func TestTOTPEncryptRoundTrip(t *testing.T) {
	kek := deriveKEK([]byte("test-admin-secret-key!!"))
	enc, err := encryptSecret(kek, []byte("BASE32SECRETVALUE"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := decryptSecret(kek, enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "BASE32SECRETVALUE" {
		t.Fatalf("got %q", plain)
	}
}

func TestTOTPChallengeRoundTrip(t *testing.T) {
	secret := []byte("challenge-secret-bytes-32chars!!")
	id := uuid.New()
	tok, err := IssueTOTPChallenge(secret, id, "a@b.uz")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseTOTPChallenge(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.AdminUserID != id || claims.Email != "a@b.uz" {
		t.Fatalf("%+v", claims)
	}
	if _, err := ParseAccess(secret, tok); err == nil {
		t.Fatal("challenge must not parse as access token")
	}
}

func TestValidateTOTP(t *testing.T) {
	key, err := generateTOTPKey("ops@example.uz")
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !validateTOTP(key.Secret(), code) {
		t.Fatal("expected valid code")
	}
	if validateTOTP(key.Secret(), "000000") {
		t.Fatal("expected invalid code")
	}
}
