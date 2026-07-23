package main

// Authoritative road-sign catalogue for Uzbekistan (YHQ 1-ilova). Sign codes,
// group membership and official names are a published state standard — factual
// data, not authored content — sourced from the official appendix and
// cross-checked against the public catalogue, not from any competitor product.
//
// Names are provided in uz-Latn and ru, both from the standard. uz-Cyrl is
// deliberately omitted for now: Uzbek Latin→Cyrillic has genuinely ambiguous
// cases (word-initial e→э vs е) that would risk shipping a wrong reading, and
// the read path already falls uz-Cyrl back to uz-Latn. Cyrillic will be added
// as a separately reviewed pass, not guessed here.
//
// The list is built group by group; each committed batch is a fully verified
// slice. Groups are complete from the first import so the catalogue's
// structure is stable while signs are filled in.

// group codes must match the frontend filter values exactly
// (frontend/src/app/[locale]/(app)/signs/page.tsx).
var groups = []groupSeed{
	{Code: "warning", Sort: 1, NameLatn: "Ogohlantiruvchi belgilar", NameRu: "Предупреждающие знаки"},
	{Code: "priority", Sort: 2, NameLatn: "Imtiyoz belgilari", NameRu: "Знаки приоритета"},
	{Code: "prohibiting", Sort: 3, NameLatn: "Taqiqlovchi belgilar", NameRu: "Запрещающие знаки"},
	{Code: "mandatory", Sort: 4, NameLatn: "Buyuruvchi belgilar", NameRu: "Предписывающие знаки"},
	{Code: "info", Sort: 5, NameLatn: "Axborot-ko'rsatgich belgilari", NameRu: "Информационно-указательные знаки"},
	{Code: "service", Sort: 6, NameLatn: "Servis belgilari", NameRu: "Знаки сервиса"},
	{Code: "supplementary", Sort: 7, NameLatn: "Qo'shimcha axborot belgilari", NameRu: "Знаки дополнительной информации"},
}

// signs is the growing catalogue. Each entry is verified against the official
// appendix before it is added; a sign absent here simply is not in the
// catalogue yet, which is honest, rather than present with a guessed name.
var signs = []signSeed{
	// 2. Imtiyoz belgilari — priority
	{Code: "2.1", Group: "priority", NameLatn: "Asosiy yo'l", NameRu: "Главная дорога"},
	{Code: "2.2", Group: "priority", NameLatn: "Asosiy yo'lning oxiri", NameRu: "Конец главной дороги"},
	{Code: "2.3.1", Group: "priority", NameLatn: "Ikkinchi darajali yo'l bilan kesishuv", NameRu: "Пересечение со второстепенной дорогой"},
	{Code: "2.3.2", Group: "priority", NameLatn: "Ikkinchi darajali yo'lning o'ngdan tutashuvi", NameRu: "Примыкание второстепенной дороги справа"},
	{Code: "2.3.3", Group: "priority", NameLatn: "Ikkinchi darajali yo'lning chapdan tutashuvi", NameRu: "Примыкание второстепенной дороги слева"},
	{Code: "2.4", Group: "priority", NameLatn: "Yo'l bering", NameRu: "Уступите дорогу"},
	{Code: "2.5", Group: "priority", NameLatn: "To'xtamasdan harakatlanish taqiqlangan", NameRu: "Движение без остановки запрещено"},
	{Code: "2.6", Group: "priority", NameLatn: "Qarama-qarshi harakatning ustunligi", NameRu: "Преимущество встречного движения"},
	{Code: "2.7", Group: "priority", NameLatn: "Qarama-qarshi harakatga nisbatan ustunlik", NameRu: "Преимущество перед встречным движением"},
}
