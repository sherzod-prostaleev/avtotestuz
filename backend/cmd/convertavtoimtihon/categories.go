// Category taxonomy for the official YHQ dataset — 42 official chapters and
// appendix sections (docs/superpowers/specs/2026-08-28-42-topic-taxonomy-design.md).
package main

import (
	"avtotest.uz/backend/internal/importer"
)

type categoryDef struct {
	code   string
	uzLatn string
	uzCyrl string
	ru     string
}

// Order = sort order (1-based), matching official YHQ document structure.
var categoryDefs = []categoryDef{
	{"general_rules", "Umumiy qoidalar", "Умумий қоидалар", "Общие положения"},
	{"driver_duties", "Haydovchilarning umumiy vazifalari", "Ҳайдовчиларнинг умумий вазифалари", "Общие обязанности водителей"},
	{"pedestrian_duties", "Piyodalarning umumiy vazifalari", "Пиёдаларнинг умумий вазифалари", "Общие обязанности пешеходов"},
	{"special_vehicle_priority", "Maxsus transport vositalarining imtiyozlari", "Махсус транспорт воситаларининг имтиёзлари", "Преимущества специальных транспортных средств"},
	{"signs_warning", "Ogohlantiruvchi belgilar", "Огоҳлантирувчи белгилар", "Предупреждающие знаки"},
	{"signs_priority", "Imtiyoz belgilari", "Имтиёз белгилари", "Знаки приоритета"},
	{"signs_prohibitory", "Taqiqlovchi belgilar", "Тақиқловчи белгилар", "Запрещающие знаки"},
	{"signs_mandatory", "Buyuruvchi belgilar", "Буюрувчи белгилар", "Предписывающие знаки"},
	{"signs_information", "Axborot-ko‘rsatgich belgilari", "Ахборот-кўрсатгич белгилари", "Информационно-указательные знаки"},
	{"signs_service", "Servis belgilari", "Сервис белгилари", "Сервисные знаки"},
	{"signs_additional", "Qo‘shimcha axborot belgilari", "Қўшимча ахборот белгилари", "Знаки дополнительной информации"},
	{"markings_horizontal", "Yotiq chiziqlar", "Ётиқ чизиқлар", "Горизонтальная разметка"},
	{"markings_vertical", "Tik chiziqlar", "Тик чизиқлар", "Вертикальная разметка"},
	{"traffic_lights", "Svetofor ishoralari", "Светофор ишоралари", "Сигналы светофора"},
	{"traffic_controller", "Tartibga soluvchining ishoralari", "Тартибга солувчининг ишоралари", "Сигналы регулировщика"},
	{"warning_hazard_signals", "Ogohlantiruvchi va avariya (xavf-xatar) ishoralari", "Огоҳлантирувчи ва авария (хавф-хатар) ишоралари", "Предупредительные и аварийные сигналы"},
	{"starting_manoeuvring", "Harakatlanishni boshlash, manyovr qilish", "Ҳаракатланишни бошлаш, манёвр қилиш", "Начало движения и маневрирование"},
	{"lane_position", "Yo‘lning qatnov qismida transport vositalarining joylashuvi", "Йўлнинг қатнов қисмида транспорт воситаларининг жойлашуви", "Расположение транспортных средств на проезжей части"},
	{"speed_limits", "Harakatlanish tezligi", "Ҳаракатланиш тезлиги", "Скорость движения"},
	{"overtaking", "Quvib o‘tish", "Қувиб ўтиш", "Обгон"},
	{"stopping_and_parking", "To‘xtash va to‘xtab turish", "Тўхташ ва тўхтаб туриш", "Остановка и стоянка"},
	{"intersections_general", "Chorrahalarda harakatlanish", "Чорраҳаларда ҳаракатланиш", "Движение на перекрестках"},
	{"intersections_regulated", "Tartibga solingan chorrahalar", "Тартибга солинган чорраҳалар", "Регулируемые перекрестки"},
	{"intersections_main_straight", "Tartibga solinmagan chorrahalar asosiy yo‘l yo‘nalishi to‘g‘riga", "Тартибга солинмаган чорраҳалар асосий йўл йўналиши тўғрига", "Нерегулируемые перекрестки с главной дорогой в прямом направлении"},
	{"intersections_equal", "Tartibga solinmagan chorrahalar teng ahamiyatli", "Тартибга солинмаган чорраҳалар тенг аҳамиятли", "Нерегулируемые перекрестки равнозначных дорог"},
	{"intersections_main_turns", "Tartibga solinmagan chorrahalar asosiy yo‘l yo‘nalishi o‘zgarishi", "Тартибга солинмаган чорраҳалар асосий йўл йўналиши ўзгариши", "Нерегулируемые перекрестки с изменением направления главной дороги"},
	{"pedestrian_crossings_stops", "Piyodalarning o‘tish joylari va yo‘nalishli transport vositalarining bekatlari", "Пиёдаларнинг ўтиш жойлари ва йўналишли транспорт воситаларининг бекатлари", "Пешеходные переходы и остановки маршрутных транспортных средств"},
	{"railway_crossings", "Temir yo‘l kesishmalari orqali harakatlanish", "Темир йўл кесишмалари орқали ҳаракатланиш", "Движение через железнодорожные переезды"},
	{"motorways", "Avtomagistrallarda harakatlanish", "Автомагистралларда ҳаракатланиш", "Движение по автомагистралям"},
	{"residential_zones", "Turar joy dahalarida harakatlanish", "Турар жой даҳаларида ҳаракатланиш", "Движение в жилых зонах"},
	{"slopes", "Tik balandlik va nishabliklarda harakatlanish", "Тик баландлик ва нишабликларда ҳаракатланиш", "Движение на крутых подъемах и спусках"},
	{"public_transport_priority", "Yo‘nalishli transport vositalarining imtiyozlari", "Йўналишли транспорт воситаларининг имтиёзлари", "Преимущества маршрутных транспортных средств"},
	{"lighting_devices", "Tashqi yoritish asboblaridan foydalanish", "Ташқи ёритиш асбобларидан фойдаланиш", "Использование внешних световых приборов"},
	{"towing", "Mexanik transport vositalarini shatakka olish", "Механик транспорт воситаларини шатакка олиш", "Буксировка механических транспортных средств"},
	{"driver_training", "Transport vositalarini boshqarishni o‘rgatish", "Транспорт воситаларини бошқаришни ўргатиш", "Обучение управлению транспортными средствами"},
	{"passenger_carriage", "Odam tashish", "Одам ташиш", "Перевозка людей"},
	{"cargo_carriage", "Yuk tashish", "Юк ташиш", "Перевозка грузов"},
	{"cyclists_mopeds_animals", "Velosiped, moped va aravalar harakatlanishiga, shuningdek, hayvonlarni haydab o‘tishga doir qo‘shimcha talablar", "Велосипед, мопед ва аравалар ҳаракатланишига, шунингдек, ҳайвонларни ҳайдаб ўтишга доир қўшимча талаблар", "Дополнительные требования к движению велосипедов, мопедов и гужевых повозок, а также к перегону животных"},
	{"officials_duties", "Mansabdor shaxslarning va fuqarolarning yo‘l harakati xavfsizligini taminlash, transport vositalarini yo‘lga chiqarish, raqam va taniqli belgilarini o‘rnatish bo‘yicha majburiyatlari", "Мансабдор шахсларнинг ва фуқароларнинг йўл ҳаракати хавфсизлигини таъминлаш, транспорт воситаларини йўлга чиқариш, рақам ва таниқли белгиларини ўрнатиш бўйича мажбуриятлари", "Обязанности должностных лиц и граждан по обеспечению безопасности дорожного движения, выпуску транспортных средств на линию, установке регистрационных и опознавательных знаков"},
	{"vehicle_defects", "Transport vositalaridan foydalanishni taqiqlovchi shartlar", "Транспорт воситаларидан фойдаланишни тақиқловчи шартлар", "Условия, запрещающие эксплуатацию транспортных средств"},
	{"safety_basics", "Harakat xafsizligi asoslari", "Ҳаракат хавфсизлиги асослари", "Основы безопасности дорожного движения"},
	{"first_aid", "Birinchi tibbiy yordam", "Биринчи тиббий ёрдам", "Первая медицинская помощь"},
}

// categoriesForDataset returns the 42 canonical categories in sort order.
func categoriesForDataset(includeFallback bool) []importer.CanonCategory {
	out := make([]importer.CanonCategory, 0, len(categoryDefs))
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
	return out
}
