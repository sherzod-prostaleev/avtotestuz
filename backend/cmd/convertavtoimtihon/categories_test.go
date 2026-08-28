package main

import (
	"regexp"
	"strings"
	"testing"
)

// A Cyrillic name that still holds Latin words is a half-finished
// transliteration, not a name: public_transport_priority shipped as
// "Йўналишли transport vositalarining imtiyozlari" and would have overwritten
// the correct seed entry the next time the converter ran.
var latinWord = regexp.MustCompile(`[A-Za-z]{2,}`)

func TestCategoryNamesUseTheRightScript(t *testing.T) {
	for _, c := range categoriesForDataset(false) {
		if got := latinWord.FindAllString(c.Names["uz-Cyrl"], -1); got != nil {
			t.Errorf("%s: uz-Cyrl name has Latin words %v: %q", c.Code, got, c.Names["uz-Cyrl"])
		}
		if got := latinWord.FindAllString(c.Names["ru"], -1); got != nil {
			t.Errorf("%s: ru name has Latin words %v: %q", c.Code, got, c.Names["ru"])
		}
		if strings.ContainsAny(c.Names["uz-Latn"], "ЁёІЇАБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдежзийклмнопрстуфхцчшщъыьэюяқўғҳ") {
			t.Errorf("%s: uz-Latn name has Cyrillic: %q", c.Code, c.Names["uz-Latn"])
		}
	}
}

func TestCategoriesForDataset(t *testing.T) {
	cats := categoriesForDataset(false)
	if len(cats) != 42 {
		t.Fatalf("want 42 categories, got %d", len(cats))
	}
	seen := map[string]bool{}
	for i, c := range cats {
		if c.Sort != i+1 {
			t.Fatalf("category %s: sort %d, want %d (sorted, 1-based)", c.Code, c.Sort, i+1)
		}
		if seen[c.Code] {
			t.Fatalf("duplicate code %s", c.Code)
		}
		seen[c.Code] = true
		for _, loc := range []string{"uz-Latn", "uz-Cyrl", "ru"} {
			if c.Names[loc] == "" {
				t.Fatalf("category %s: missing %s name", c.Code, loc)
			}
		}
	}
}
