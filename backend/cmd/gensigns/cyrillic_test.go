package main

import "testing"

func TestToCyrillic(t *testing.T) {
	cases := map[string]string{
		// o'/g' digraphs and the y+o' priority case
		"yo'l":      "йўл",
		"O'ng":      "Ўнг",
		"o'ngga":    "ўнгга",
		"g'ildirak": "ғилдирак",
		// tutuq belgisi ' → ъ
		"Ta'mirlash": "Таъмирлаш",
		"Sun'iy":     "Сунъий",
		"Jome'":      "Жомеъ",
		// word-initial e → э, medial e → е
		"Eng":      "Энг",
		"Ekologik": "Экологик",
		"tezlik":   "тезлик",
		"Reversiv": "Реверсив",
		// iotated y+vowel
		"Piyodalar":   "Пиёдалар",
		"Yengil":      "Енгил",
		"yashaydigan": "яшайдиган",
		// digraphs sh/ch and letters h/q/x
		"Shlagbaumli":   "Шлагбаумли",
		"Chapga":        "Чапга",
		"harakatlanish": "ҳаракатланиш",
		"Qayrilish":     "Қайрилиш",
		"Xavf":          "Хавф",
		// full names
		"Asosiy yo'l":           "Асосий йўл",
		"Aholi yashaydigan joy": "Аҳоли яшайдиган жой",
		// guillemets pass through, inner o' still converts
		"«o'rta»": "«ўрта»",
		// loanword edge cases
		"Aeroport":          "Аэропорт",
		"Mototsikllar":      "Мотоцикллар",
		"obyektlari":        "объектлари",
		"Turizm obyektlari": "Туризм объектлари",
	}
	for in, want := range cases {
		if got := toCyrillic(in); got != want {
			t.Errorf("toCyrillic(%q) = %q; want %q", in, got, want)
		}
	}
}
