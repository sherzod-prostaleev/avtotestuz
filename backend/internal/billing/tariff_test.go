package billing

import "testing"

func TestPricePerDay(t *testing.T) {
	cases := []struct {
		price int64
		days  int32
		want  int64
	}{
		{24900, 7, 3557},
		{59900, 30, 1997},
		{109900, 75, 1465},
		{100, 0, 100}, // guard: no divide-by-zero
	}
	for _, c := range cases {
		if got := pricePerDay(c.price, c.days); got != c.want {
			t.Errorf("pricePerDay(%d,%d)=%d want %d", c.price, c.days, got, c.want)
		}
	}
}

func TestDiscountPercent(t *testing.T) {
	p := func(v int64) *int64 { return &v }
	cases := []struct {
		price int64
		old   *int64
		want  int32
	}{
		{24900, p(34900), 29},
		{59900, p(99900), 40},
		{109900, p(199900), 45},
		{100, nil, 0},   // no old price
		{100, p(50), 0}, // old not higher
	}
	for _, c := range cases {
		if got := discountPercent(c.price, c.old); got != c.want {
			t.Errorf("discountPercent(%d,%v)=%d want %d", c.price, c.old, got, c.want)
		}
	}
}
