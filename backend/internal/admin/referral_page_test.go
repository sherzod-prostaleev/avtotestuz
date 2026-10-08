package admin

import (
	"net/url"
	"testing"
)

func TestReferralPageBoundsAndIgnoresInvalid(t *testing.T) {
	cases := []struct {
		query         string
		limit, offset int32
	}{
		{"", 50, 0},
		{"limit=10&offset=30", 10, 30},
		{"limit=0", 1, 0},
		{"limit=-5&offset=-5", 1, 0},
		{"limit=100000", 200, 0},
		{"limit=4294967297", 50, 0}, // would wrap to 1 via int32(Atoi)
		{"offset=4294967297", 50, 0},
		{"limit=abc&offset=xyz", 50, 0},
	}
	for _, c := range cases {
		q, _ := url.ParseQuery(c.query)
		limit, offset := referralPage(q)
		if limit != c.limit || offset != c.offset {
			t.Errorf("%q: got (%d,%d) want (%d,%d)", c.query, limit, offset, c.limit, c.offset)
		}
	}
}
