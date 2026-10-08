package account

import "testing"

func TestPaymentHistoryLimit(t *testing.T) {
	cases := []struct {
		raw     string
		want    int32
		wantErr bool
	}{
		{"", 20, false},
		{"20", 20, false},
		{"0", 20, false},
		{"-3", 20, false},
		{"100", 100, false},
		{"5000", 100, false},
		{"4294967297", 0, true}, // would wrap to 1 via int32(Atoi)
		{"abc", 0, true},
	}
	for _, c := range cases {
		got, err := paymentHistoryLimit(c.raw)
		if (err != nil) != c.wantErr || (err == nil && got != c.want) {
			t.Errorf("%q: got (%d,%v) want (%d, err=%v)", c.raw, got, err, c.want, c.wantErr)
		}
	}
}
