package main

import "testing"

// The frontend filters signs by these exact group codes; a mismatch would
// silently hide a whole group, so pin the set here.
var frontendGroupCodes = []string{
	"warning", "priority", "prohibiting", "mandatory", "info", "service", "supplementary",
}

func TestBuildEmitsAllSevenGroups(t *testing.T) {
	ds, err := build()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, g := range ds.SignGroups {
		got[g.Code] = true
	}
	for _, code := range frontendGroupCodes {
		if !got[code] {
			t.Errorf("group %q missing from dataset", code)
		}
	}
	if len(ds.SignGroups) != len(frontendGroupCodes) {
		t.Errorf("group count = %d, want %d", len(ds.SignGroups), len(frontendGroupCodes))
	}
}

func TestBuildSignsAreWellFormed(t *testing.T) {
	ds, err := build()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Signs) == 0 {
		t.Fatal("no signs emitted")
	}
	groupCodes := map[string]bool{}
	for _, g := range ds.SignGroups {
		groupCodes[g.Code] = true
	}
	seen := map[string]bool{}
	for _, s := range ds.Signs {
		if !groupCodes[s.Group] {
			t.Errorf("sign %q references unknown group %q", s.Code, s.Group)
		}
		if seen[s.Code] {
			t.Errorf("duplicate sign code %q", s.Code)
		}
		seen[s.Code] = true
		// Both authoritative locales must be present; uz-Cyrl is intentionally
		// absent and left to read-path fallback.
		if s.Names["uz-Latn"] == "" || s.Names["ru"] == "" {
			t.Errorf("sign %q missing a required locale name: %+v", s.Code, s.Names)
		}
	}
}

func TestBuildRejectsDuplicateSignCode(t *testing.T) {
	// Guards the invariant build() enforces; the real data must never trip it.
	ds, err := build()
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]int{}
	for _, s := range ds.Signs {
		codes[s.Code]++
	}
	for code, n := range codes {
		if n > 1 {
			t.Fatalf("sign code %q appears %d times", code, n)
		}
	}
}
