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

// TestTOTPKEKFallsBackToJWTSecret is the data-preservation guarantee behind
// DATA_ENCRYPTION_KEY: every TOTP secret in production was sealed under the
// JWT secret, so a Service that has not been given a separate data key must
// still decrypt them. Breaking this locks every admin out of their account
// with no migration path back.
func TestTOTPKEKFallsBackToJWTSecret(t *testing.T) {
	jwtSecret := []byte("jwt-signing-secret-at-least-32-bytes!")

	// Ciphertext exactly as a pre-split deployment wrote it.
	legacy, err := encryptSecret(deriveKEK(jwtSecret), []byte("LEGACYBASE32SECRET"))
	if err != nil {
		t.Fatal(err)
	}

	unwired := Service{Secret: jwtSecret}
	plain, err := decryptSecret(unwired.dataKEK(), legacy)
	if err != nil || string(plain) != "LEGACYBASE32SECRET" {
		t.Fatalf("legacy ciphertext must survive: plain=%q err=%v", plain, err)
	}

	// A deployment that adopts the split by setting DATA_ENCRYPTION_KEY to its
	// current JWT_SECRET keeps reading the same rows.
	adopted := Service{Secret: jwtSecret, DataKey: jwtSecret}
	plain, err = decryptSecret(adopted.dataKEK(), legacy)
	if err != nil || string(plain) != "LEGACYBASE32SECRET" {
		t.Fatalf("adopting the same value must be a no-op: plain=%q err=%v", plain, err)
	}

	// And once the keys genuinely differ, the JWT secret no longer opens the
	// data — which is the whole point: rotating it stops costing secrets.
	rotated := Service{Secret: []byte("a-completely-rotated-jwt-secret-32b!!"), DataKey: jwtSecret}
	plain, err = decryptSecret(rotated.dataKEK(), legacy)
	if err != nil || string(plain) != "LEGACYBASE32SECRET" {
		t.Fatalf("data key must survive JWT rotation: plain=%q err=%v", plain, err)
	}
	separate := Service{Secret: jwtSecret, DataKey: []byte("a-separate-data-encryption-key-32b!!!")}
	if _, err := decryptSecret(separate.dataKEK(), legacy); err == nil {
		t.Fatal("a distinct data key must not open JWT-secret ciphertext")
	}
}

// TestTOTPEnrollTokenIsScoped pins the blast radius of the token login hands
// an admin who cannot sign in yet: it must not be usable as a session.
func TestTOTPEnrollTokenIsScoped(t *testing.T) {
	secret := []byte("enroll-secret-bytes-at-least-32-chars")
	id := uuid.New()
	tok, err := IssueTOTPEnroll(secret, id, "a@b.uz")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseTOTPEnroll(secret, tok)
	if err != nil || claims.AdminUserID != id {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	if _, err := ParseAccess(secret, tok); err == nil {
		t.Fatal("enrollment token must not parse as an admin access token")
	}
	if _, err := ParseTOTPChallenge(secret, tok); err == nil {
		t.Fatal("enrollment token must not parse as a TOTP challenge")
	}
	// And the reverse: a challenge token must not authorize enrollment.
	ch, err := IssueTOTPChallenge(secret, id, "a@b.uz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTOTPEnroll(secret, ch); err == nil {
		t.Fatal("challenge token must not parse as an enrollment token")
	}
	if _, err := ParseTOTPEnroll([]byte("some-other-signing-secret-32-bytes!!!"), tok); err == nil {
		t.Fatal("enrollment token must not verify under a foreign signing key")
	}
}

// TestEnrollSecretIsServerDerived covers AD-4: confirmation must not accept a
// secret of the caller's choosing, so the server has to be able to re-derive
// exactly what it handed out.
func TestEnrollSecretIsServerDerived(t *testing.T) {
	svc := Service{Secret: []byte("admin-signing-secret-at-least-32-b!!")}
	id := uuid.New()
	window := enrollWindowAt(time.Now())

	secret := svc.enrollSecret(id, window)
	if secret != svc.enrollSecret(id, window) {
		t.Fatal("same admin and window must re-derive the same secret")
	}
	if secret == svc.enrollSecret(uuid.New(), window) {
		t.Fatal("secret must be bound to the admin id")
	}
	if secret == svc.enrollSecret(id, window+1) {
		t.Fatal("secret must roll over with the window")
	}
	other := Service{Secret: []byte("a-different-admin-signing-secret-32b!")}
	if secret == other.enrollSecret(id, window) {
		t.Fatal("secret must depend on the server key, or a client could compute it")
	}
	if _, err := b32NoPad.DecodeString(secret); err != nil {
		t.Fatalf("secret must be a valid base32 otpauth secret: %v", err)
	}

	// A live code from the derived secret confirms; an unrelated secret the
	// caller invented does not, however valid its own code is.
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got, ok := svc.pendingTOTPSecret(id, code)
	if !ok || got != secret {
		t.Fatalf("pendingTOTPSecret got=%q ok=%v, want the derived secret", got, ok)
	}
	rogue, err := generateTOTPKey("attacker@example.uz", nil)
	if err != nil {
		t.Fatal(err)
	}
	rogueCode, err := totp.GenerateCode(rogue.Secret(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.pendingTOTPSecret(id, rogueCode); ok {
		t.Fatal("a client-chosen secret must never be accepted for enrollment")
	}
}

func TestValidateTOTP(t *testing.T) {
	key, err := generateTOTPKey("ops@example.uz", nil)
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
