package auth

import "testing"

func TestNormalizePhone(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"+998 90 123-45-67", "+998901234567", true},
		{"998901234567", "+998901234567", true},
		{"901234567", "+998901234567", true},
		{"(90) 123 45 67", "+998901234567", true},
		{"+7 900 000 00 00", "", false},
		{"12345", "", false},
		{"", "", false},
	} {
		got, err := NormalizePhone(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("%q → (%q,%v), want %q", tc.in, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Errorf("%q → %q, want error", tc.in, got)
		}
	}
}
