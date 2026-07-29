# Telegram guruh quizi — dizayn (2026-07-29)

Driver Go Telegram botining `/quiz` rejimini bir kishilik savol-javobdan
ko'p kishilik, vaqt chegarali guruh musobaqasiga aylantirish.

Manba: foydalanuvchi brifi 2026-07-29. Hozirgi holat `internal/bot/quiz.go`
kodidan o'qib tekshirilgan, raqamlar jonli API'dan o'lchangan.

---

## 1. Muammo — kodda tasdiqlangan

Bot hozir ishlaydi, lekin guruhda musobaqa sifatida ishlamaydi.

### 1.1 Uzun javoblar kesiladi

`quiz.go:24` — `quizButtonMaxRunes = 60`. `answerMarkup()` (`quiz.go:372`)
har bir javobni 60 runega qisqartirib, oxiriga `…` qo'yadi. Foydalanuvchi
javobning yarmini ko'radi va bilmasdan tanlaydi.

### 1.2 Guruhda birinchi bosgan hamma uchun javob beradi

`HandleCallback` (`quiz.go:155-160`) `cq.From.ID` ni biladi, lekin uni
`handleAnswer(ctx, chatID, messageID, idx)` ga **uzatmaydi**. Javob chatga
yoziladi, foydalanuvchiga emas. Birinchi bosilgan javobdan keyin
`EditMessageReplyMarkup` klaviaturani o'chiradi (`quiz.go:317`) — qolganlar
javob bera olmaydi.

### 1.3 Kim nechta to'g'ri bilgani saqlanmaydi

Migratsiya 0039 dagi `telegram_quiz_session` da `asked_count` va
`correct_count` — **sessiyaga**, ya'ni butun chatga tegishli bitta juft
raqam. Ishtirokchilar jadvali yo'q.

### 1.4 Vaqt chegarasi yo'q

Savol cheksiz ochiq turadi. `quizMinInterval = 3s` (`quiz.go:32`) — bu
savollar orasidagi tanaffus, javob vaqti emas.

---

## 2. Qulflangan qarorlar

| # | Qaror | Qiymat |
|---|---|---|
| D1 | Savol formati | Rasm (alohida xabar) + Telegram **native quiz poll** |
| D2 | O'yin uzunligi | **10 savol** |
| D3 | Savolga vaqt | **10 sekund** (sozlanadigan, deploy'siz o'zgaradi) |
| D4 | G'olib | To'g'ri javob soni; tenglikda — o'rtacha javob vaqti tezrog'i |
| D5 | Korpus qamrovi | Faqat barcha javoblari ≤100 belgi bo'lgan savollar (~80%) |
| D6 | Statistika | O'yin oxirida to'liq reyting: kim nechta to'g'ri, o'rtacha vaqt |

### D5 asosi — o'lchangan

15 bilet, 300 savol, 1010 javob tahlil qilindi (jonli API):

| Ko'rsatkich | Natija |
|---|---|
| Savol matni eng uzuni | 222 belgi → poll limiti 300 ga **hammasi sig'adi** |
| Javob matni eng uzuni | 414 belgi |
| Kamida bitta uzun javobli savol | **20.0%** |
| So'rovnomaga yaroqli savol | **80.0%** |

1231 savollik korpusdan ~985 tasi yaroqli. 10 savollik o'yin uchun yetarli.
Chiqarilganlar ilovada to'liq qoladi — faqat botda chiqmaydi.

---

## 3. Telegram Bot API cheklovlari

Amalga oshirishda joriy rasmiy hujjatga qarshi qayta tasdiqlanadi.

| Maydon | Cheklov | Bizga ta'siri |
|---|---|---|
| `sendPoll.question` | 1–300 belgi | Hammasi sig'adi (max 222) |
| `sendPoll.options[]` | 2–10 ta, har biri 1–100 belgi | D5 filtri shundan |
| `type: "quiz"` | `correct_option_id` majburiy | Bor |
| `explanation` | 0–200 belgi | Savol izohi shu yerga (agar bor bo'lsa) |
| `open_period` | 5–600 sekund | 10 sekund yaroqli |
| `is_anonymous: false` | `poll_answer` da user kim ekanini bilish uchun **shart** | Guruh reytingi shunga bog'liq |
| `poll_answer` update | `allowed_updates` ga qo'shilishi shart | Hozir yo'q — qo'shiladi |
| `message_effect_id` | **Faqat shaxsiy chat** | Guruhda boshqa effekt ishlatiladi |
| Poll'ga rasm | **Biriktirib bo'lmaydi** | Shuning uchun rasm alohida xabar |

---

## 4. Ma'lumotlar modeli — migratsiya 0046 (faqat qo'shimcha)

Mavjud jadval yoki ustun **o'zgartirilmaydi va o'chirilmaydi**.

```sql
-- Ishtirokchi hisobi: har bir o'yinda har bir odam uchun bitta qator.
CREATE TABLE telegram_quiz_participant (
  session_id     uuid NOT NULL REFERENCES telegram_quiz_session(id) ON DELETE CASCADE,
  tg_user_id     bigint NOT NULL,
  display_name   text NOT NULL DEFAULT '',
  answered_count int NOT NULL DEFAULT 0,
  correct_count  int NOT NULL DEFAULT 0,
  total_ms       bigint NOT NULL DEFAULT 0,   -- javob vaqtlari yig'indisi
  first_seen_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, tg_user_id)
);

CREATE INDEX telegram_quiz_participant_rank_idx
  ON telegram_quiz_participant (session_id, correct_count DESC, total_ms ASC);

-- Poll ↔ savol bog'lanishi: poll_answer update'i faqat poll_id beradi.
CREATE TABLE telegram_quiz_poll (
  poll_id      text PRIMARY KEY,
  session_id   uuid NOT NULL REFERENCES telegram_quiz_session(id) ON DELETE CASCADE,
  question_id  uuid REFERENCES question(id) ON DELETE SET NULL,
  question_no  int NOT NULL,
  correct_idx  int NOT NULL,
  opened_at    timestamptz NOT NULL DEFAULT now(),
  closed       boolean NOT NULL DEFAULT false
);

CREATE INDEX telegram_quiz_poll_session_idx ON telegram_quiz_poll (session_id, question_no);
```

Mavjud `telegram_quiz_session` ga qo'shiladigan ustunlar (ALTER … ADD COLUMN,
hammasi DEFAULT bilan — eski qatorlar buzilmaydi):

```sql
ALTER TABLE telegram_quiz_session
  ADD COLUMN total_questions int NOT NULL DEFAULT 10,
  ADD COLUMN question_no     int NOT NULL DEFAULT 0,
  ADD COLUMN mode            text NOT NULL DEFAULT 'solo';  -- solo | group
```

### Sozlamalar (kodga qotirilmaydi)

`limit_config` jadvaliga qo'shiladi — admin panelning `/settings/limits`
sahifasidan deploy'siz o'zgartiriladi:

| Kalit | Boshlang'ich | Ma'nosi |
|---|---|---|
| `tg_quiz_seconds` | 10 | Bitta savolga sekund (5–600) |
| `tg_quiz_questions` | 10 | O'yindagi savollar soni |

---

## 5. O'yin oqimi

### 5.1 Boshlanishi

```
/quiz  →  faol sessiya bormi?
          ├─ ha  → "O'yin allaqachon ketyapti (3/10-savol)"
          └─ yo'q → sessiya yaratiladi (mode = chat turiga qarab solo|group)
                    "🚦 Quiz boshlandi! 10 savol · har biriga 10 sekund"
                    3 sekund sanoq → 1-savol
```

### 5.2 Har bir savol — ikki xabar

1. **Rasm** (`sendPhoto`), izohsiz yoki qisqa izoh bilan.
   Rasm bo'lmasa bu qadam tashlab ketiladi.
2. **So'rovnoma** (`sendPoll`), rasmga reply qilib:
   - `question`: savol matni (≤300, hammasi sig'adi)
   - `options`: javoblar (≤100 har biri, D5 filtri kafolatlaydi)
   - `type: "quiz"`, `correct_option_id`, `is_anonymous: false`
   - `open_period`: `tg_quiz_seconds`
   - `explanation`: savol izohi ≤200 belgi (bo'lsa)

`poll_id` → `telegram_quiz_poll` ga yoziladi.

### 5.3 Javob qabul qilish

`poll_answer` update keladi: `poll_id`, `user`, `option_ids`.

```
poll_id bo'yicha telegram_quiz_poll topiladi
  └─ topilmasa (eski o'yin) → jimgina e'tiborsiz qoldiriladi
participant UPSERT (session_id, tg_user_id)
  display_name   ← poll_answer.user.first_name (bo'sh bo'lsa username, u ham
                   bo'sh bo'lsa "Ishtirokchi")
  answered_count += 1
  correct_count  += (option_ids[0] == correct_idx ? 1 : 0)
  total_ms       += now() - opened_at
```

`mode` (`solo` | `group`) **chat turiga qarab** belgilanadi (`IsGroupChat`),
ishtirokchilar soniga qarab emas. Bitta kishilik guruh ham `group` rejimida
qoladi va reyting formatini oladi.

Shaxsiy chatda ham xuddi shu `telegram_quiz_participant` jadvali ishlatiladi
— bitta qator bilan. Bu ikkinchi hisoblash yo'lini yozishdan qutqaradi:
solo va guruh bir xil kod, faqat yakuniy xabar formati farq qiladi.

Bitta tranzaksiyada. Telegram bir foydalanuvchidan bitta poll uchun bitta
javob beradi (quiz poll'da javobni o'zgartirib bo'lmaydi) — takroriy
`poll_answer` kelsa PRIMARY KEY konflikti `DO NOTHING` bilan yutiladi.

### 5.4 Savoldan savolga o'tish

`open_period` tugagach Telegram poll'ni o'zi yopadi. Bot taymer bo'yicha
(`opened_at + seconds + 1s`) keyingi savolni yuboradi. Taymer yo'qolsa
(deploy, restart) — `/next` qo'lda davom ettiradi.

### 5.5 Yakun

10-savoldan keyin sessiya `active = false` va yakuniy xabar yuboriladi.

**Guruh:**
```
🏆 O'yin tugadi!

🥇 Aziz          9/10  ·  4.2s
🥈 Malika        8/10  ·  6.1s
🥉 Bekzod        8/10  ·  9.7s
 4. Nodira       6/10  ·  7.4s
 5. Jasur        5/10  ·  8.8s

👥 5 ishtirokchi · 10 savol

🎉 Tabriklaymiz, Aziz!
```
+ g'olibga animatsion stiker, yakuniy xabarga 🎉 reaksiyasi.

**Shaxsiy chat:**
```
✅ Natijangiz: 8/10 · o'rtacha 5.4s
```
+ `message_effect_id` = 🎉 (salyut effekti — faqat shaxsiy chatda ishlaydi).

Ikkalasida ham ostida CTA: «Ilovada ochish».

---

## 6. Vizual effektlar

| Hodisa | Guruh | Shaxsiy |
|---|---|---|
| To'g'ri javob | Telegram quiz poll'ining o'z konfettisi (avtomatik) | Xuddi shu |
| O'yin yakuni | Yakuniy xabarga 🎉 reaksiya (+ ixtiyoriy stiker) | `message_effect_id` 🎉 salyut |
| Xato javob | Poll'ning o'z belgisi + `explanation` matnida to'g'ri javob izohi | Xuddi shu |

Animatsion stiker **kodga qotirilmaydi**: Telegram `file_id` lari tekshirilmasa
ishonchsiz. `tg_quiz_winner_sticker` sozlamasi (bo'sh = o'tkazib yuboriladi)
orqali beriladi, keyinroq haqiqiy stiker tanlab qo'yiladi. Kafolatlangan
effekt — `setMessageReaction` 🎉, u har doim ishlaydi.

`message_effect_id` guruhda **ishlamaydi** — Telegram cheklovi, aylanib
o'tib bo'lmaydi. Shuning uchun guruhda stiker + reaksiya ishlatiladi.

---

## 7. Chetki holatlar

| Holat | Xatti-harakat |
|---|---|
| Hech kim javob bermadi | Savol o'tkaziladi, keyingisiga o'tiladi |
| Bir kishi ham ishtirok etmadi (guruh) | Yakunda: «Hech kim qatnashmadi» — reyting yuborilmaydi |
| O'yin o'rtasida bot restart bo'ldi | Taymer yo'qoladi; `/next` davom ettiradi; 30 daqiqa jimlikdan keyin sessiya eskiradi (mavjud `quizIdleTTL`) |
| `/stop` | Sessiya yopiladi, shu paytgacha to'plangan reyting yuboriladi |
| Yaroqli savol topilmadi | «Savol topilmadi» xabari, sessiya yopiladi |
| Eski poll'ga javob | `telegram_quiz_poll` da topilmaydi → e'tiborsiz |
| `telegram_quiz` flagi o'chiq | `/quiz` rad javob beradi (mavjud xatti-harakat saqlanadi) |

---

## 8. Testlash

Yangi test fayllari `internal/bot/` ichida, mavjud uslubda (`quiz_test.go`
namunasi — soxta Telegram serveri `httptest` bilan).

| Test | Nimani isbotlaydi |
|---|---|
| `TestPollPayloadLimits` | Yuborilayotgan poll savol ≤300, variantlar ≤100 — chegaradagi savol bilan |
| `TestCorpusFilterExcludesLongAnswers` | Uzun javobli savol tanlanmaydi |
| `TestPollAnswerRecordsPerUser` | Ikki xil `tg_user_id` ikkita alohida qator yaratadi |
| `TestPollAnswerIdempotent` | Bir xil (poll, user) ikki marta kelsa hisob bir marta oshadi |
| `TestRankingTieBreakBySpeed` | Teng to'g'ri javobda tezrog'i yuqorida |
| `TestGroupFinalStatsShape` | Yakuniy xabarda hamma ishtirokchi bor |
| `TestMessageEffectOnlyInPrivate` | Guruhga `message_effect_id` yuborilmaydi |
| `TestStopMidGameSendsRanking` | `/stop` reyting bilan tugaydi |

**Har bir test avval sindirib ko'riladi** — tuzatishdan oldingi kodga
qarshi ishga tushirilib, qizil bo'lishiga ishonch hosil qilinadi.

---

## 9. Qamrovdan tashqari (bu ishda qilinmaydi)

- Ilova ichidagi mashq, bilet, imtihon oqimlariga tegilmaydi
- GRAND MOCK dizayni va umumiy CSS tokenlar (`--success`, `--danger`,
  `--accent`, `--ring`) — tegilmaydi
- To'lov, premium, referral mantiqiga tegilmaydi
- Admin panel M3-7 ishi — to'xtatilgan, bu specga kirmaydi
- Uzun javoblarni qo'lda qisqartirish (kontent ishi) — keyinroq
- Ko'p tilli quiz: hozir `quizLocale = "uz-Latn"` saqlanadi

---

## 10. Yetkazib berish tartibi

Har bir bosqich alohida PR, alohida tekshiriladi.

| # | Bosqich | Mazmun |
|---|---|---|
| 1 | Poydevor | Migratsiya 0046, sqlc so'rovlari, `sendPoll` + `poll_answer` mijoz metodlari, `allowed_updates` yangilanishi |
| 2 | Bir kishilik oqim | Rasm + poll formati, 10 savol, taymer, yakuniy natija, shaxsiy chat effekti |
| 3 | Guruh oqimi | Ishtirokchi hisobi, reyting, g'olib tabrigi, to'liq statistika |
| 4 | Sozlash | `limit_config` kalitlari + admin panelda ko'rinishi |

Tekshiruv har bosqichda: `go build ./...`, `go vet ./...`, `go test ./...`,
va frontend tegilsa `npx tsc --noEmit`, `npx vitest run`, `npm run build`.

`main` ga to'g'ridan-to'g'ri push yo'q. Merge va deploy — faqat
foydalanuvchi aniq aytganda.
