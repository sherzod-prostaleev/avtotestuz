package main

import "testing"

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
