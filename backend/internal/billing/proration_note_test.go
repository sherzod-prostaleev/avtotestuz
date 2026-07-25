package billing

import "testing"

func TestParseProrationNote(t *testing.T) {
	note := formatPromoProratedNote("SAVE50", "promo_limit_reached", 12, 30, 29950, 59900)
	info, ok := ParseProrationNote(note)
	if !ok {
		t.Fatalf("ParseProrationNote(%q) = false", note)
	}
	if !info.Applied || info.GrantedDays != 12 || info.TariffDays != 30 || info.Reason != "promo_limit_reached" {
		t.Fatalf("parsed = %+v", info)
	}
	if _, ok := ParseProrationNote(""); ok {
		t.Fatal("empty note should not parse")
	}
	if _, ok := ParseProrationNote("unrelated note"); ok {
		t.Fatal("unrelated note should not parse")
	}
}
