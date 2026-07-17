// Package fixture builds a clearly-marked [NAMUNA] sample dataset satisfying
// every content invariant. NOT real YHQ content — dev/test only.
package fixture

import (
	"encoding/base64"
	"fmt"

	"avtotest.uz/backend/internal/importer"
)

// 1x1 transparent PNG.
const tinyPNGb64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func tinyPNG() []byte {
	b, _ := base64.StdEncoding.DecodeString(tinyPNGb64)
	return b
}

func tr(latn, cyrl, ru string) map[string]string {
	return map[string]string{"uz-Latn": latn, "uz-Cyrl": cyrl, "ru": ru}
}

// Sample returns 40 questions (2 variants × 20), 4 categories, 2 sign groups,
// 4 signs, 1 explanation, with tiny PNG images for signs and every 4th question.
func Sample() (importer.Dataset, importer.MapSource) {
	images := importer.MapSource{}
	ds := importer.Dataset{
		Categories: []importer.CanonCategory{
			{Code: "signs", Sort: 1, Names: tr("[NAMUNA] Yo'l belgilari", "[НАМУНА] Йўл белгилари", "[ОБРАЗЕЦ] Дорожные знаки")},
			{Code: "rules", Sort: 2, Names: tr("[NAMUNA] Harakat qoidalari", "[НАМУНА] Ҳаракат қоидалари", "[ОБРАЗЕЦ] Правила движения")},
			{Code: "priority", Sort: 3, Names: tr("[NAMUNA] Ustunlik", "[НАМУНА] Устунлик", "[ОБРАЗЕЦ] Преимущество")},
			{Code: "safety", Sort: 4, Names: tr("[NAMUNA] Xavfsizlik", "[НАМУНА] Хавфсизлик", "[ОБРАЗЕЦ] Безопасность")},
		},
		SignGroups: []importer.CanonSignGroup{
			{Code: "prohibiting", Sort: 1, Names: tr("[NAMUNA] Taqiqlovchi", "[НАМУНА] Тақиқловчи", "[ОБРАЗЕЦ] Запрещающие")},
			{Code: "info", Sort: 2, Names: tr("[NAMUNA] Axborot", "[НАМУНА] Ахборот", "[ОБРАЗЕЦ] Информационные")},
		},
	}
	signCodes := []struct{ code, group string }{
		{"3.27", "prohibiting"}, {"3.1", "prohibiting"}, {"7.18", "info"}, {"7.1.1", "info"},
	}
	for i, s := range signCodes {
		img := fmt.Sprintf("images/sign-%s.png", s.code)
		images[img] = tinyPNG()
		ds.Signs = append(ds.Signs, importer.CanonSign{
			Code: s.code, Group: s.group, Image: img, Sort: i + 1,
			Names:        tr("[NAMUNA] Belgi "+s.code, "[НАМУНА] Белги "+s.code, "[ОБРАЗЕЦ] Знак "+s.code),
			Descriptions: tr("[NAMUNA] Tavsif "+s.code, "[НАМУНА] Тавсиф "+s.code, "[ОБРАЗЕЦ] Описание "+s.code),
		})
	}
	cats := []string{"signs", "rules", "priority", "safety"}
	for n := 1; n <= 40; n++ {
		ext := fmt.Sprintf("nmn-%04d", n)
		q := importer.CanonQuestion{
			ExtID:    ext,
			Category: cats[n%len(cats)],
			Texts: tr(
				fmt.Sprintf("[NAMUNA] Savol %d — bu haqiqiy YHQ savoli EMAS (dev uchun).", n),
				fmt.Sprintf("[НАМУНА] Савол %d — бу ҳақиқий ЙҲҚ саволи ЭМАС (dev учун).", n),
				fmt.Sprintf("[ОБРАЗЕЦ] Вопрос %d — это НЕ настоящий вопрос ПДД (для dev).", n),
			),
			Source: "fixture",
		}
		if n%4 == 0 {
			img := fmt.Sprintf("images/q-%04d.png", n)
			images[img] = tinyPNG()
			q.Image = img
			q.Signs = []string{"3.27", "7.18"}
		}
		correct := n%4 + 1
		for p := 1; p <= 4; p++ {
			q.Answers = append(q.Answers, importer.CanonAnswer{
				Position: p, Correct: p == correct,
				Texts: tr(
					fmt.Sprintf("[NAMUNA] Javob %d.%d", n, p),
					fmt.Sprintf("[НАМУНА] Жавоб %d.%d", n, p),
					fmt.Sprintf("[ОБРАЗЕЦ] Ответ %d.%d", n, p),
				),
			})
		}
		ds.Questions = append(ds.Questions, q)
	}
	for v := 1; v <= 2; v++ {
		var ids []string
		for n := (v-1)*20 + 1; n <= v*20; n++ {
			ids = append(ids, fmt.Sprintf("nmn-%04d", n))
		}
		ds.Variants = append(ds.Variants, importer.CanonVariant{Number: v, Questions: ids})
	}
	ds.Explanations = []importer.CanonExplanation{{
		Question:  "nmn-0004",
		LegalRefs: []map[string]string{{"code": "YHQ 0.0", "title": "[NAMUNA] modda"}},
		Blocks: map[string][]map[string]any{
			"uz-Latn": {
				{"type": "important", "md": "[NAMUNA] Bu izoh namunasi — haqiqiy huquqiy tahlil emas."},
				{"type": "answers", "items": []any{
					map[string]any{"position": 1, "correct": false, "md": "[NAMUNA] 1 noto'g'ri"},
					map[string]any{"position": 2, "correct": false, "md": "[NAMUNA] 2 noto'g'ri"},
					map[string]any{"position": 3, "correct": false, "md": "[NAMUNA] 3 noto'g'ri"},
					map[string]any{"position": 4, "correct": true, "md": "[NAMUNA] 4 to'g'ri"},
				}},
			},
		},
	}}
	return ds, images
}
