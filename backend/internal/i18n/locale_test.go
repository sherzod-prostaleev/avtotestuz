package i18n

import (
	"net/http/httptest"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		q    string
		want string
		ok   bool
	}{
		{"", "uz-Latn", true},
		{"?locale=ru", "ru", true},
		{"?locale=uz-Cyrl", "uz-Cyrl", true},
		{"?locale=kaa", "kaa", true},
		{"?locale=en", "", false},
	} {
		r := httptest.NewRequest("GET", "/x"+tc.q, nil)
		got, ok := Parse(r)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%q → (%q,%v), want (%q,%v)", tc.q, got, ok, tc.want, tc.ok)
		}
	}
}
