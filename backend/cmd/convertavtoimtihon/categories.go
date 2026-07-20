// Category taxonomy for the real avtoimtihon dataset — 13 user-approved
// categories (docs/superpowers/research/2026-07-21-category-taxonomy-proposal.md §3,
// approved 2026-07-21) plus a deterministic citation-based classifier.
//
// The chapter numbers below are the DATASET's own internal PDD numbering
// (older than the 2022 revision) — they are classification signals, not
// legal citations, and must not be "corrected" here.
package main

import (
	"regexp"
	"strconv"

	"avtotest.uz/backend/internal/importer"
)

type categoryDef struct {
	code   string
	uzLatn string
	uzCyrl string
	ru     string
}

// Order = sort order (1-based), largest estimated first (taxonomy §3).
var categoryDefs = []categoryDef{
	{"road_signs_markings", "Yo'l belgilari va chizig'i", "Йўл белгилари ва чизиғи", "Дорожные знаки и разметка"},
	{"priority_intersections", "Chorrahalar va yo'l ustunligi", "Чорраҳалар ва йўл устунлиги", "Перекрёстки и преимущество проезда"},
	{"maneuvering_lane_position", "Manyovr va yo'lda joylashuv", "Манёвр ва йўлда жойлашув", "Манёврирование и расположение на проезжей части"},
	{"vehicle_equipment_lighting", "Transport vositasi jihozi va yorug'lik", "Транспорт воситаси жиҳози ва ёруғлик", "Техническое состояние ТС и освещение"},
	{"stopping_parking", "To'xtash va to'xtab turish", "Тўхташ ва тўхтаб туриш", "Остановка и стоянка"},
	{"overtaking_speed", "Quvib o'tish va tezlik", "Қувиб ўтиш ва тезлик", "Обгон и скорость движения"},
	{"pedestrians_public_transport", "Piyodalar, yo'lovchilar va yo'nalishli transport", "Пиёдалар, йўловчилар ва йўналишли транспорт", "Пешеходы, пассажиры и маршрутный транспорт"},
	{"special_road_zones", "Maxsus yo'l uchastkalari", "Махсус йўл участкалари", "Особые участки дорог"},
	{"traffic_signals_gestures", "Svetofor va tartibga soluvchi ishoralari", "Светофор ва тартибга солувчи ишоралари", "Сигналы светофора и регулировщика"},
	{"towing_special_vehicles", "Shatakka olish va maxsus transport", "Шатакка олиш ва махсус транспорт", "Буксировка и спецтранспорт"},
	{"accidents_first_aid_dynamics", "YHH, tez tibbiy yordam va tormozlash", "ЙТҲ, тез тиббий ёрдам ва тормозлаш", "ДТП, первая помощь и динамика торможения"},
	{"cargo_passenger_carriage", "Yuk va odam tashish", "Юк ва одам ташиш", "Перевозка людей и грузов"},
	{"general_provisions_admin", "Umumiy qoidalar va majburiyatlar", "Умумий қоидалар ва мажбуриятлар", "Общие положения и обязанности"},
}

var chapterToCategory = map[int]string{
	1: "general_provisions_admin", 2: "general_provisions_admin",
	3: "accidents_first_aid_dynamics",
	4: "pedestrians_public_transport", 5: "pedestrians_public_transport",
	6: "towing_special_vehicles",
	7: "traffic_signals_gestures",
	8: "vehicle_equipment_lighting",
	9: "maneuvering_lane_position", 10: "maneuvering_lane_position",
	11: "overtaking_speed", 12: "overtaking_speed",
	13: "stopping_parking",
	14: "priority_intersections", 15: "priority_intersections", 16: "priority_intersections",
	17: "pedestrians_public_transport",
	18: "special_road_zones", 19: "special_road_zones", 20: "special_road_zones", 21: "special_road_zones",
	22: "pedestrians_public_transport",
	23: "vehicle_equipment_lighting",
	24: "towing_special_vehicles",
	25: "general_provisions_admin",
	26: "cargo_passenger_carriage", 27: "cargo_passenger_carriage",
	28: "pedestrians_public_transport",
	29: "general_provisions_admin",
}

var appendixToCategory = map[int]string{
	1: "road_signs_markings", 2: "road_signs_markings",
	3: "vehicle_equipment_lighting",
	4: "cargo_passenger_carriage",
}

var (
	chapterRe  = regexp.MustCompile(`(?i)глав[а-яё]*\s+(\d+)\s+ПДД`)
	appendixRe = regexp.MustCompile(`(?i)приложени[а-яё]*\s*№?\s*(\d+)`)
)

// classifyByCitation resolves a category from the FIRST chapter citation in
// the ru explanation text, else the FIRST appendix citation. Unknown numbers
// are unresolved (never guessed).
func classifyByCitation(ruComment string) (string, bool) {
	if m := chapterRe.FindStringSubmatch(ruComment); m != nil {
		n, _ := strconv.Atoi(m[1])
		code, ok := chapterToCategory[n]
		return code, ok
	}
	if m := appendixRe.FindStringSubmatch(ruComment); m != nil {
		n, _ := strconv.Atoi(m[1])
		code, ok := appendixToCategory[n]
		return code, ok
	}
	return "", false
}

// categoriesForDataset returns the 13 canonical categories in sort order.
// includeFallback appends the legacy umumiy category — only needed while
// unresolved questions still fall back to it (non-strict conversions).
func categoriesForDataset(includeFallback bool) []importer.CanonCategory {
	out := make([]importer.CanonCategory, 0, len(categoryDefs)+1)
	for i, d := range categoryDefs {
		out = append(out, importer.CanonCategory{
			Code: d.code,
			Sort: i + 1,
			Names: map[string]string{
				"uz-Latn": d.uzLatn,
				"uz-Cyrl": d.uzCyrl,
				"ru":      d.ru,
			},
		})
	}
	if includeFallback {
		out = append(out, importer.CanonCategory{
			Code: "umumiy",
			Sort: len(categoryDefs) + 1,
			Names: map[string]string{
				"uz-Latn": "Umumiy savollar",
				"uz-Cyrl": "Умумий саволлар",
				"ru":      "Общие вопросы",
			},
		})
	}
	return out
}
