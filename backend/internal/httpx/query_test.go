package httpx

import "testing"

func TestParseInt32RejectsOutOfRange(t *testing.T) {
	for _, raw := range []string{"4294967297", "2147483648", "-2147483649", "", "abc", "1.5"} {
		if _, err := ParseInt32(raw); err == nil {
			t.Errorf("ParseInt32(%q) accepted, want error", raw)
		}
	}
	for raw, want := range map[string]int32{"0": 0, "12": 12, "2147483647": 2147483647, "-5": -5} {
		got, err := ParseInt32(raw)
		if err != nil || got != want {
			t.Errorf("ParseInt32(%q) = %d, %v; want %d", raw, got, err, want)
		}
	}
}
