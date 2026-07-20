package main

import "testing"

func TestClassifyByCitation(t *testing.T) {
	cases := []struct {
		name string
		in   string
		code string
		ok   bool
	}{
		{"chapter cite", "Пункта 2 главы 16 ПДД, гласит: на нерегулируемом перекрестке...", "priority_intersections", true},
		{"chapter lowercase form", "согласно пункту 5 главы 13 ПДД остановка запрещена", "stopping_parking", true},
		{"appendix only", "Приложение №1 к ПДД пункт 3.27: Остановка запрещена...", "road_signs_markings", true},
		{"appendix no numero sign", "Приложение 3 к ПДД: неисправности...", "vehicle_equipment_lighting", true},
		{"chapter beats appendix", "Пункта 1 главы 9 ПДД... см. также Приложение №2", "maneuvering_lane_position", true},
		{"unknown chapter number", "глава 30 ПДД", "", false},
		{"unknown appendix number", "Приложение №7 к ПДД", "", false},
		{"no citation", "При торможении на скользкой дороге возможен занос.", "", false},
		{"empty", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, ok := classifyByCitation(c.in)
			if code != c.code || ok != c.ok {
				t.Fatalf("classifyByCitation(%q) = (%q,%v), want (%q,%v)", c.in, code, ok, c.code, c.ok)
			}
		})
	}
}

func TestCategoriesForDataset(t *testing.T) {
	cats := categoriesForDataset(false)
	if len(cats) != 13 {
		t.Fatalf("want 13 categories, got %d", len(cats))
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
	withFallback := categoriesForDataset(true)
	if len(withFallback) != 14 || withFallback[13].Code != "umumiy" {
		t.Fatalf("includeFallback: want umumiy appended as 14th, got %d entries", len(withFallback))
	}
	// every chapter/appendix mapping target must be a real category code
	for ch, code := range chapterToCategory {
		if !seen[code] {
			t.Fatalf("chapterToCategory[%d] -> unknown code %q", ch, code)
		}
	}
	for ap, code := range appendixToCategory {
		if !seen[code] {
			t.Fatalf("appendixToCategory[%d] -> unknown code %q", ap, code)
		}
	}
}
