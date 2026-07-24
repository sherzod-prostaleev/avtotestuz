package click

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
)

// TestComputeSign proves computeSign matches Click's documented sign_string
// formula by hand-computing the MD5 of the literal concatenated string in
// the test itself (rather than transcribing a magic hex literal), so any
// drift in the formula (field order, missing separator, etc.) fails loudly.
func TestComputeSign(t *testing.T) {
	req := clickRequest{
		ClickTransID:    "123",
		MerchantTransID: "abc",
		Amount:          "5000",
		Action:          "0",
		SignTime:        "1700000000",
	}
	serviceID := "10"
	secretKey := "secret"

	// Action "0" (Prepare): merchant_prepare_id is NOT included.
	raw := "123" + "10" + "secret" + "abc" + "" + "5000" + "0" + "1700000000"
	sum := md5.Sum([]byte(raw))
	want := hex.EncodeToString(sum[:])

	got := computeSign(req, serviceID, secretKey)
	if got != want {
		t.Fatalf("computeSign() = %q, want %q", got, want)
	}

	// Action "1" (Complete): merchant_prepare_id IS included, so the sign
	// must differ from the Action "0" case above.
	completeReq := clickRequest{
		ClickTransID:      "123",
		MerchantTransID:   "abc",
		Amount:            "5000",
		Action:            "1",
		SignTime:          "1700000000",
		MerchantPrepareID: "77",
	}
	rawComplete := "123" + "10" + "secret" + "abc" + "77" + "5000" + "1" + "1700000000"
	sumComplete := md5.Sum([]byte(rawComplete))
	wantComplete := hex.EncodeToString(sumComplete[:])

	gotComplete := computeSign(completeReq, serviceID, secretKey)
	if gotComplete != wantComplete {
		t.Fatalf("computeSign() (Action=1) = %q, want %q", gotComplete, wantComplete)
	}
	if gotComplete == got {
		t.Fatalf("expected Action=1 sign (with merchant_prepare_id) to differ from Action=0 sign, both = %q", got)
	}
}

// TestValidSign_EmptySecretAlwaysFails proves an unconfigured Click
// integration (empty SecretKey) never accepts any signature, by design.
func TestValidSign_EmptySecretAlwaysFails(t *testing.T) {
	req := clickRequest{
		ClickTransID:    "123",
		MerchantTransID: "abc",
		Amount:          "5000",
		Action:          "0",
		SignTime:        "1700000000",
		SignString:      "irrelevant",
	}
	if validSign(req, "10", "") {
		t.Fatal("validSign() = true with empty secretKey, want false")
	}

	// Even the correctly computed sign_string must fail against an empty
	// secret key.
	req.SignString = computeSign(req, "10", "secret")
	if validSign(req, "10", "") {
		t.Fatal("validSign() = true with empty secretKey even though sign_string matches a real key, want false")
	}
}
