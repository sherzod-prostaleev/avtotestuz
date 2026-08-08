package auth

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRandomTempPassword(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 20; i++ {
		pw, err := RandomTempPassword()
		if err != nil {
			t.Fatal(err)
		}
		if utf8.RuneCountInString(pw) != tempPasswordLen {
			t.Fatalf("len=%d want %d", utf8.RuneCountInString(pw), tempPasswordLen)
		}
		for _, r := range pw {
			if !strings.ContainsRune(tempPasswordAlphabet, r) {
				t.Fatalf("unexpected rune %q in %q", r, pw)
			}
		}
		if _, ok := seen[pw]; ok {
			t.Fatalf("duplicate temporary password generated: %q", pw)
		}
		seen[pw] = struct{}{}
		hash, err := HashPassword(pw)
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		if hash == pw || strings.Contains(hash, pw) {
			t.Fatal("hash must not contain plaintext")
		}
		if !CheckPassword(hash, pw) {
			t.Fatal("temp password must verify against its hash")
		}
	}
}
