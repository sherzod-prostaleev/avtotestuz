# Savol ↔ yo'l belgisi bog'lanishlari auditi — dizayn

**Sana:** 2026-09-06
**Holat:** tasdiqlangan (foydalanuvchi qoidani va yakunni tanladi)
**Oldingi o'xshash loyiha:** `2026-09-03-42-topic-category-audit-design.md`

## Muammo

`backend/seed/avtoimtihon/question_signs.json` — 285 savolni 170 ta belgi kodiga
bog'laydigan xarita (686 bog'lanish). U to'liq **onless.uz** ma'lumotidan
qurilgan (commit `5db6fa7`, "Rebuild practice-by-sign links from Onless
mappings") va hech kim uni rasm bilan solishtirib tekshirmagan.

Bu xarita `question_sign` jadvalini to'ldiradi, undan esa ikki narsa ishlaydi:

- **Belgi bo'yicha mashq** — `/signs` sahifasida har bir belgi kartochkasida
  savol soni ko'rsatiladi; bosilganda aynan o'sha belgining savollari bilan
  mashq sessiyasi boshlanadi (`ListSigns.question_count`,
  `RandomQuestionIDsBySign`).
- **Savol yonidagi belgilar ro'yxati** — `ListQuestionSigns` /
  `ListQuestionSignsByQuestionIDs`.

Savol soni 0 bo'lgan belgi kartochkasi umuman bosilmaydi — faqat tavsif modali
ochiladi.

### Tasdiqlangan zarar

| Savol | Rasmda haqiqatda | Xaritada yozilgan | Xulosa |
|---|---|---|---|
| `avtoimtihon-105` (`i11_5.webp`) | **4.3** «Aylanma harakat» | `3.19`, `5.8.1` | Ikkalasi ham rasmda YO'Q |

To'g'ri ishlaganlari ham bor (`-1072` → 2.4 + 7.13 ✓, `-1091` → 3.27 + 7.18 ✓),
demak xarita butunlay yaroqsiz emas — lekin ishonchsiz, har bittasi qayta
ko'rilishi shart.

### Qamrov bo'shliqlari

- 743 rasmli savoldan **465 tasi umuman bog'lanmagan**.
- Faqat `signs_*` kategoriyalarining o'zida: 245 rasmli savoldan 86 tasi
  bog'lanmagan.
- 285 belgidan **115 tasida bironta ham savol yo'q**.
- 7 ta rasmsiz savol bog'langan (matnda belgi tilga olingani uchun) — bu qoida
  butun korpusga izchil qo'llanmagan.

## Maqsad

`question_signs.json` ni **noldan, rasmlarni o'z ko'zim bilan ko'rib** qayta
qurish. Har bir belgi aynan o'ziga tegishli savollarni olsin; hech bir savol
o'zida yo'q belgiga bog'lanmasin.

Doiradan tashqarida: savol matnini, javoblarini, kategoriyasini yoki rasmini
o'zgartirish; belgilar katalogining o'zini (285 belgi) qayta ko'rib chiqish;
mashq sessiyasi hajmi UI'si.

## Bog'lash qoidasi

Foydalanuvchi tanladi: **rasmda ko'ringan har bir belgi**.

Aniq ta'rifi:

1. **Rasmda aniq ko'rinadigan har bir yo'l belgisi** bog'lanadi — u sahnada
   ustunda tursinmi yoki javob varianti sifatida (1, 2, 3, 4, 5 deb
   raqamlangan) ko'rsatilsinmi, farqi yo'q. Javob variantlari holatida
   **hammasi** bog'lanadi, faqat to'g'ri javob emas: noto'g'ri variantni
   tanimaslik ham o'sha belgini bilishning bir qismi.
2. **Matnda kodi yoki rasmiy nomi aytilgan belgi** ham bog'lanadi (`3.24`,
   «Avtomagistral», «To'xtash joyi») — rasmsiz savollarda ham.
3. **Bog'lanmaydi:** taniqlik belgilari («Nogiron», «Shiplar», «Yosh haydovchi»
   — bular avtomobil ustidagi stikerlar, yo'l belgisi emas), yo'l razmetkasi,
   svetofor, reklama lavhalari, ko'cha nomi lavhalari.
4. **O'qib bo'lmaydigan belgi** (juda mayda, xira, burchakda kesilgan)
   bog'lanmaydi, lekin hisobotdagi `unreadable` ro'yxatiga tushadi.
5. Katalogda (285 kod) mavjud bo'lmagan belgi bog'lanmaydi — hisobotdagi
   `not_in_catalog` ro'yxatiga tushadi.

### Qoidaning ma'lum oqibati

`2.1` va `2.4` kabi keng tarqalgan belgilarda savol soni hozirgi 42 dan
~90 gacha o'sishi mumkin. `/signs` kartochkasi bosilganda mashq sessiyasi
aynan shu son bilan boshlanadi (`practiceHref` → `count=question_count`),
serverda pool cheklovi yo'q — faqat kunlik allowance clamp qiladi. Bu **shu
loyihaga kirmaydi**, hisobotda alohida tavsiya sifatida qayd etiladi.

## Manba ishonchi

| Daraja | Manba | Roli |
|---|---|---|
| 1 | Savol rasmining o'zi | Yagona haqiqat manbai |
| 1 | `backend/seed/signs/` — 285 belgi rasmi + uz-Latn/ru nomlari | Kod aniqlash etaloni (davlat standarti, avval tekshirilgan) |
| 2 | Mavjud `question_signs.json` (onless) | **Manba emas, da'vogar.** Men bilan kelishmagan har bir joyda rasm ikkinchi marta ochiladi |
| 3 | onless.uz jonli sayt | Faqat rasmdan aniqlab bo'lmagan yakka holatlar uchun; Chrome subagentlari **hech qachon parallel emas** |

Foydalanuvchining aniq ko'rsatmasi: qaror mening ko'zim bilan qabul qilinadi,
kodga ham, boshqa birovga ham ishonilmaydi. Shu sababli **rasmni ko'rish
subagentga topshirilmaydi** — asosiy sessiyaning o'zi ko'radi.

## Konveyer

### 0-bosqich — etalon

285 belgi rasmini guruh-guruh ko'rib chiqaman va kod ↔ ko'rinish jadvalini
yozib olaman. Chalkashadigan oilalar alohida qayd etiladi: `2.3.x`,
`3.27`/`3.28`, `5.16.1`/`5.16.2`, `7.4.x`, `1.11.x`, `4.1.x`, `5.8.x`.

Chiqish: `scratch/sign-audit/catalog-notes.md`.

### 1-bosqich — 743 rasmli savol

Partiyalab (bir turda ~8–12 savol) ko'rib chiqaman. Har bir savol uchun
matn + javoblar + rasm birga baholanadi. Natija **darhol** JSONL'ga yoziladi,
shuning uchun kontekst siqilishi hech qanday ishni yo'qotmaydi.

Chiqish: `scratch/sign-audit/pass1.jsonl` — har qatorda
`{ext_id, codes, confidence, unreadable, not_in_catalog, note}`.

### 2-bosqich — 531 rasmsiz savol

Matn skani: kod shabloni (`\d\.\d+(\.\d+)?`) va 285 belgining rasmiy nomlari
bo'yicha nomzodlar ajratiladi, so'ng har bir nomzod savolni o'zim o'qib
tasdiqlayman.

Chiqish: `scratch/sign-audit/pass2-text.jsonl`.

### 3-bosqich — nishonli ikkinchi qarash

Foydalanuvchi tanlagan tekshiruv darajasi. Quyidagilarning **hammasi** qayta
ochiladi:

- (a) natijam onless xaritasi bilan kelishmagan har bir rasm;
- (b) 1-bosqichda `confidence != high` deb belgilangan har bir rasm;
- (c) bironta belgi topilmagan rasmlar (o'tkazib yuborilmadimi?);
- (d) chalkashadigan oiladagi kod berilgan rasmlar.

Chiqish: `scratch/sign-audit/pass3.jsonl` (yakuniy qaror shu yerda ustun).

### 4-bosqich — teskari tekshiruv

Har bir 285 belgi uchun unga bog'langan savollar ro'yxati chiqariladi:

- savoli 0 ta bo'lgan belgilar — haqiqatan hech qayerda uchramaydimi?
- g'ayritabiiy ko'p savolli belgilar — noto'g'ri kod yopishtirilmadimi?

### 5-bosqich — qo'llash

Yangi `backend/seed/avtoimtihon/question_signs.json` yoziladi (mavjud fayl
formati saqlanadi: `ext_id → sorted unique codes`). Docker'dagi dev bazada
`linkquestionsigns` yugurtiriladi: `unknown_signs=0`, `missing_questions=0`
bo'lishi shart.

### 6-bosqich — yakuniy dasturiy solishtirish

42-mavzu auditining saboqi: yakuniy fayl "niyat qilingan holat"ga **to'liq**
solishtiriladi (285 tasi emas, 1274 savolning hammasi bo'yicha), qisman emas.
Bosqich chiqishlaridan qayta qurilgan xarita commit qilinayotgan fayl bilan
bayt-baytga bir xil bo'lishi kerak.

## Hisobot

`scratch/sign-audit/report.md` (gitignored):

- qo'shilgan / o'chirilgan / o'zgarmagan bog'lanishlar soni;
- onless bilan kelishmovchiliklar ro'yxati va har birining sababi;
- har bir belgi bo'yicha yakuniy savol soni (oldingi son bilan yonma-yon);
- `unreadable` va `not_in_catalog` ro'yxatlari;
- savoli hamon 0 bo'lib qolgan belgilar ro'yxati.

## Yakun

Foydalanuvchi tanladi: **commit + push + prod deploy**.

- `main`ga commit + push (bu loyihada feature branch ishlatilmaydi).
- Prod: `linkquestionsigns` konteynerda qayta yugurtiriladi. `question_sign` —
  sof bog'lovchi jadval, unga hech qanday FK ishora qilmaydi va buyruq bitta
  tranzaksiyada `DELETE`+`INSERT` qiladi, shuning uchun **o'quvchi progressiga
  umuman tegmaydi** (42-mavzu auditidagi `category_mastery` xavfi bu yerda yo'q).
- `deploy/smoke.sh` bilan yakunlanadi.

## Xavflar

| Xavf | Yumshatish |
|---|---|
| Kontekst siqilishi — 743 rasm bitta oynaga sig'maydi | Har partiyadan keyin darhol JSONL'ga yozish; rejada qayerdan davom etish aniq ko'rsatilgan |
| Chalkash kodlar (3.27 vs 3.28) | 0-bosqich etaloni + 3-bosqich (d) nishoni |
| Prod importerni qayta yugurtirish zarurati | Kerak emas: faqat `linkquestionsigns` yugurtiriladi, `cmd/importer` emas — demak MinIO bucket tuzog'i bu yerda qo'llanmaydi |
| Belgi savollari soni keskin oshib, mashq sessiyasi juda uzun bo'lishi | Doiradan tashqarida, hisobotda tavsiya sifatida qayd etiladi |
