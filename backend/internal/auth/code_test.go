package auth

import (
	"regexp"
	"strings"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	re := regexp.MustCompile(`^\d{6}$`)
	for i := 0; i < 50; i++ {
		c, err := GenerateCode()
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(c) {
			t.Fatalf("bad code %q", c)
		}
	}
}

func TestHashVerifyCode(t *testing.T) {
	code, err := GenerateCode()
	if err != nil {
		t.Fatal(err)
	}
	stored, err := HashCode(code)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored, "$") {
		t.Fatalf("stored format: %q", stored)
	}
	if !VerifyCode(stored, code) {
		t.Fatal("verify must succeed for correct code")
	}
	if VerifyCode(stored, "000000") && code != "000000" {
		t.Fatal("verify must fail for wrong code")
	}
	if VerifyCode("garbage", code) {
		t.Fatal("verify must fail for malformed stored value")
	}
	// salted: same code twice → different stored values
	second, err := HashCode(code)
	if err != nil {
		t.Fatal(err)
	}
	if second == stored {
		t.Fatal("hash must be salted")
	}
}

func TestNewReferralCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		c, err := NewReferralCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != 8 {
			t.Fatalf("len(%q)=%d", c, len(c))
		}
		for _, r := range c {
			if !strings.ContainsRune(crockford, r) {
				t.Fatalf("bad char %q in %q", r, c)
			}
		}
		if seen[c] {
			t.Fatalf("duplicate referral code in 1000 draws: %q", c)
		}
		seen[c] = true
	}
}
