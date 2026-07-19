# AvtoTest Platform — Master Dizayn Hujjati (v2)

**Sana:** 2026-07-17
**Miqyos:** Butun platforma (startap) + M1 batafsil
**Status:** Ko'rib chiqilmoqda (foydalanuvchi tasdig'i kutilmoqda)

---

## 1. Vizyon

O'zbekiston haydovchilik nazariy imtihoniga tayyorlaydigan **pullik onlayn maktab-platforma**. Bozordagi yetakchi (onless.uz) funksional jihatdan o'rganildi (21 skrinshot tahlili asosida) — maqsad uni nusxalash emas, **10x yaxshiroq mahsulot**: ilmiy o'quv dvigateli, chuqurroq huquqiy izohlar, halolroq va arzonroq narx, super-admin nazorat, investor-tayyor metrikalar, 100 000+ o'quvchiga chidaydigan arxitektura.

**Biznes model:** freemium B2C (muddatli obuna-passlar) + keyinchalik B2B (avtomaktablar litsenziyasi). Referal + promo-kodlar bilan o'sish.

---

## 2. Tasdiqlangan qarorlar

| # | Qaror | Tanlov |
|---|---|---|
| D1 | Frontend (ilova) | Flutter — bitta kod bazasi: Web → Android/iOS/desktop (M6) |
| D2 | Backend | Go + PostgreSQL + Redis + S3-mos storage |
| D3 | Rejim | To'liq online, server-driven; test paytida uzilishga chidamlilik (lokal buffer + resume) |
| D4 | Boshlanish | M1 = Web o'quv yadro |
| D5 | Data manbai | Foydalanuvchining ruxsat etilgan kontenti; import + validation (hash/checksum). Jonli scraping YO'Q (403/huquq) |
| D6 | Tillar | uz-Latn, uz-Cyrl, ru (M1); kaa — model tayyor, tekshirilgan manba topilgach |
| D7 | Test rejimlari | Bilet (20 savol — real manba 10-savollik "ticket"lardan juftlab tuziladi, ketma-ket ochilish), Imtihon simulyatsiyasi, Mavzu/kategoriya mashqi, Xatolar banki |
| D8 | Imtihon qoidasi | 20 savol / 25 daqiqa / ≤2 xato = o'tdi / 3-xatoda to'xtash (rasmiy qoida, manbalar §4) |
| D9 | Learning engine | FSRS, weak-area fokus, interleaving, mastery map, Leitner, streak + kunlik maqsad |
| D10 | Izohlar | AI-qoralama → ekspert tasdig'i (`draft→pending→verified`); faqat `verified` ko'rsatiladi |
| D11 | Auth | Telefon raqam + Telegram orqali kod (asosiy), SMS zaxira (Eskiz/PlayMobile); JWT |
| D12 | To'lovlar | Payme + Click (+ Uzcard/Humo) — **sandbox rejimda quriladi**, yuridik shaxs ochilgach production |
| D13 | Free-tier | 1-bilet + kunlik cheklangan mashq + belgilar katalogi bepul; qolgani obuna. Limitlar admin-panelda sozlanadi. M2'dan: mehmon-demo — 1-bilet ro'yxatdan o'tmasdan ham yechiladi (progress saqlash/sync uchun ro'yxat talab qilinadi; raqobatchilarda «darhol boshlash» standart, bu voronkaning kirish nuqtasi) |
| D14 | Tariflar | Muddatli passlar (masalan 7/15/30/45/75 kun) — narxlar admin-panelda tahrirlanadi, onless'dan arzonroq pozitsiya |
| D15 | O'sish | Referal dastur + promo-kodlar, anti-fraud bilan |
| D16 | Strategik kanal | M5 = B2B avtomaktablar (guruh litsenziya, o'qituvchi dashboard) |
| D17 | SEO | Ommaviy sahifalar (landing, narxlar, jarimalar, belgilar) — alohida yengil SSG sayt; ilova — Flutter |

---

## 3. Raqobat tahlili — onless.uz funksional xaritasi

Skrinshotlardan aniqlangan (bizga etalon va «yutish nuqtalari»):

- **O'quv:** 63 bilet (ketma-ket ochilish: oldingi biletdan ≥10/20 to'g'ri), imtihon rejimi 20/25, mavzu mashqi, xatolar banki, saqlanganlar, 258 belgi katalogi (9 kategoriya, belgi↔savol bog'lanishi), rasmli savollar filtri, GRAND MOCK (85% o'zlashtirishda ochiladi), kunlik maqsad (10 savol).
- **Imtihon UI:** F1–F4 klaviatura (real imtihon pariteti), 1–20 navigator, test ichida til almashtirish, shrift +/−, fullscreen, savol saqlash.
- **Ekspert tahlili:** belgi-chiplar, MUHIM/ESLATMA/OGOHLANTIRISH/MASLAHAT bloklari, YHQ qoidasi, har javob tahlili, biriktirilgan belgilar, foydalilik ovozi.
- **Monetizatsiya:** 5 tarif (28 990–103 990 so'm, 7–75 kun), «kuniga ~X so'm» freyming, eski narx + tejash, promo-kod, Payme + Uzcard/Humo, VIP-chegirma taymeri, telefon-konsultatsiya.
- **Marketplace:** mentorlar (299k/oy), kurator (kafolat + haftalik hisobot), akademiya (30 kun, sertifikat), «mentor bo'lish».
- **Growth:** Battle Arena (kod bilan taklif), Telegram Quiz (bot, 6 raqamli kod), Telegram kanal.
- **Kabinet:** profil (viloyat/tuman), desktop ilova + API kalit, to'lovlar tarixi, soliq cheklari, 4 til, dark/light.
- **Ommaviy sayt:** landing, Narxlar, Jarimalar (SEO), Kabinet.

**Bizning ustunliklarimiz (10x):** FSRS-based aqlli o'rganish + «imtihonga tayyorlik %» prognozi (ularda yo'q); har izohda YHQ modda-havolasi va verified-workflow; referal dasturi (ularda ko'rinmaydi); super-admin (kontent+narx+promo+foydalanuvchi+to'lov+analitika); investor-metrikalar kun 1'dan; B2B kanal; arzonroq halol narx; tezlik va sifat.

### 3.1. avtoimtihon.uz funksional xaritasi (2026-07-19 sayt-tahlili)

Real savol-to'plamimiz aynan shu saytning strukturasiga mos (prototip DB klassi `AvtoImtihonDatabase`), shuning uchun bu tahlil import-invariantlarga bevosita ta'sir qiladi:

- **Model:** to'liq bepul, reklama bilan (Yandex Ads) monetizatsiya; akkauntsiz — butun progress brauzerda (localStorage/IndexedDB), qurilmalar aro sync yo'q (shu sabab «progressni tozalash» ogohlantirishlari).
- **Kontent:** 124 bilet × 10 savol (jami 1235), javob variantlari 2–5 ta; har savolda oddiy matnli izoh (`comment`, aksariyati YHQ modda-havolali); savollarning ~58% rasmli.
- **Rejimlar:** biletlar (raqam bo'yicha qidiruv + bajarilgan/jarayonda/boshlanmagan filtri), mashq (20 tasodifiy, xatoda to'xtamaydi), imtihon (20 savol/25 daq/≤2 xato/3-xatoda stop, vaqt tugash alerti), xatolar banki, tarix (sana/natija/ball/vaqt jadvali, tozalash).
- **UI:** natija-ko'rikda har savol tahlili (sening javobing/to'g'ri javob/izoh), izohni ko'rsatish/yashirish, rasm-zoom, fullscreen, savol-navigator paneli, 3 til (uz-Latn/uz-Cyrl/ru), dark/light.
- **Distributsiya:** PWA offline o'rnatish (Windows'da «desktop ilova» sifatida), Android/iOS «tez kunda»; Telegram kanal + Instagram; SEO sahifalar (qoidalar/jarimalar/maslahatlar).
- **YO'Q (bizning ustunlik zonalari o'z kuchida):** belgilar katalogi, o'quv dvigateli/takrorlash, akkaunt/sync, statistika-tahlil, to'lov/premium kontent, verified-izoh sifat sikli.

---

## 4. Tekshirilgan faktlar

Rasmiy nazariy imtihon: 20 savol, 25 daqiqa, o'tish 18/20 (≤2 xato), 3-xatoda to'xtash, natija 2 oy amal qiladi. Manbalar: osonprava.uz/uz/blog/nazariy-imtihon-qoidalari, yim.uz/savollar-va-javoblar, gov.uz/oz/advice/73/document/144. Bilet soni datadan olinadi (onless 63, avtoimtihon 124 — modelga qat'iy son yozilmaydi).

**Real to'plam faktlari (2026-07-19, avtoimtihon-strukturali eksport tekshirildi):** 1235 savol, manbada 124 "ticket" × 10 savol (oxirgisi 5 ta), javob variantlari **2–5 ta** (taqsimot: 2→196, 3→638, 4→282, 5→119), 716 savol rasmli (`quiz-images/`, webp), 1219 savolda YHQ-havolali `comment` izohi, `correct_answer` 1-based indeks. **Qaror (tasdiqlangan):** bizning tizimda bilet hajmi qat'iy **20 savol** bo'lib qoladi (D7, rasmiy imtihon formatiga mos) — mapper ketma-ket ikkita 10-savollik "ticket"ni bitta 20-savollik biletga juftlaydi (61 to'liq bilet + 15 ta juftlanmagan qoldiq savol, alohida bilet raqamisiz, lekin mashq/xatolar/FSRS'da to'liq ishlatiladi). Javob soni esa — real imtihon ma'lumoti bo'lgani uchun — modelga qat'iy yozilmaydi, datadan (2–5).

---

## 5. Milestone xaritasi

| M | Nomi | Tarkib |
|---|---|---|
| **M1** | O'quv yadro (web) | Auth (telefon+TG kod), kontent+belgilar import, 4 rejim, bilet-ochilish, imtihon simulyator UI (F1–F5), FSRS engine, izoh ko'rsatish + AI-draft, saqlanganlar, kunlik maqsad/streak, mastery stats, 3 til, free-limitlar, event-logging, entitlement modeli (M1'da VIP'ni ichki CLI/SQL orqali berish mumkin; boshqaruv paneli M3'da) |
| **M2** | Monetizatsiya | Tariflar, Payme/Click adapterlar (sandbox→prod), promo + referal + anti-fraud, to'lovlar tarixi, cheklar, VIP-gating, GRAND MOCK, mehmon-demo rejimi (ro'yxatsiz 1-bilet, D13 — voronka kirish nuqtasi), ommaviy SSG sayt (landing/narxlar/jarimalar/belgilar SEO) |
| **M3** | Super Admin | Kontent-studio + verify workflow + import/export, foydalanuvchilar, billing/refund/rekonsilyatsiya, narx/promo/limit boshqaruvi, izoh-sifat navbati, analitika (investor dashboard), RBAC, audit, broadcast, support-inbox |
| **M4** | Growth | Battle Arena (real-time), Telegram bot (quiz, bildirishnoma), leaderboard, push/kampaniyalar |
| **M5** | B2B | Avtomaktab tashkilotlari, guruh litsenziyalari, o'qituvchi dashboardi, guruh statistikasi |
| **M6** | Multiplatforma | Flutter'dan Android/iOS/Windows/macOS/Linux buildlar, platforma sayqali, PWA o'rnatish (web A2HS install-banner — store'largacha distributsiya kanali; kontent server-driven qoladi, D3 o'zgarmaydi) |
| **M7** | Miqyos/mustahkamlash | Load-test, monitoring/alerting, xavfsizlik auditi, DR/backup drill |

Har milestone o'z spec → reja → implementatsiya siklida. Quyida platforma-arxitektura + M1 batafsil.

---

## 6. Tizim arxitekturasi

```
                    ┌────────────────────────────┐
                    │  PUBLIC SITE (SSG, SEO)     │  landing · narxlar ·
                    │  Astro — statik, tez        │  jarimalar · belgilar
                    └────────────┬───────────────┘
                                 │ build-time API
┌────────────────────────────────┼────────────────────────────────┐
│  FLUTTER APP (M1: Web · M6: mobil/desktop)                       │
│  o'quvchi kabineti · test yechish · stats · sozlamalar           │
│  M3: Admin panel (Flutter web, alohida entry)                    │
└────────────────────────────────┬────────────────────────────────┘
                                 │ HTTPS · REST/JSON · JWT
┌────────────────────────────────▼────────────────────────────────┐
│  GO API (monolit, modulli — keyin bo'linishga tayyor)            │
│  auth · content · session/scoring · learn(FSRS) · billing        │
│  promo/referral · notify(Telegram/SMS) · admin · events          │
│  middleware: JWT · RBAC · rate-limit · validate · audit · log    │
└──────┬──────────────┬──────────────┬──────────────┬─────────────┘
       ▼              ▼              ▼              ▼
 ┌──────────┐   ┌──────────┐   ┌──────────────┐  ┌─────────────────┐
 │PostgreSQL│   │  Redis   │   │ S3-mos (rasm)│  │ Tashqi servislar │
 │(+replica │   │kesh·rate │   │  + CDN       │  │ Payme·Click      │
 │ M7'da)   │   │limit·OTP │   └──────────────┘  │ Telegram Gateway │
 └────▲─────┘   └──────────┘                     │ Eskiz SMS        │
      │ import pipeline (normalize·validate·hash)└─────────────────┘
```

**Texnologiyalar:** Go 1.22+ (chi router, pgx+sqlc, golang-migrate, zap, go-redis), PostgreSQL 16, Redis 7, MinIO→S3-mos, Flutter 3.x (Riverpod, go_router, Dio, freezed, intl/ARB, Material 3), Astro (ommaviy sayt), Docker + docker-compose (dev), GitHub Actions (CI/CD).

**Hosting/compliance:** O'zbekiston shaxsiy-ma'lumotlar qonuni — fuqarolar shaxsiy ma'lumotlari O'zbekiston hududidagi serverda saqlanadi (UZ data-center VPS; rasmlar/statik — CDN mumkin, ular shaxsiy ma'lumot emas). Kunlik backup + off-site nusxa.

---

## 7. Domain model / DB sxemasi

### 7.1. Kontent (til-neytral yadro + tarjimalar, hammasi verify-statusli)

```sql
category(id uuid PK, code text UNIQUE, sort_order int, created_at)
category_translation(category_id FK, locale, name, status, PK(category_id, locale))

image(id uuid PK, storage_key, sha256 UNIQUE, width, height, mime, source, created_at)

question(id uuid PK, category_id FK, image_id FK NULL, correct_answer_id uuid NULL,
         content_hash, checksum, source, validation_status, created_at, updated_at)
question_translation(question_id FK, locale, text, status, source, PK(question_id, locale))

answer(id uuid PK, question_id FK, position smallint, is_correct bool,
       image_id FK NULL, UNIQUE(question_id, position))
answer_translation(answer_id FK, locale, text, status, PK(answer_id, locale))

variant(id uuid PK, number int UNIQUE, sort_order int)          -- bilet
variant_question(variant_id FK, question_id FK, position smallint, PK(variant_id, position))

-- Yo'l belgilari katalogi
sign_group(id uuid PK, code text UNIQUE, sort_order int)         -- taqiqlovchi, buyuruvchi...
sign_group_translation(sign_group_id FK, locale, name, status, PK(sign_group_id, locale))
sign(id uuid PK, group_id FK, code text UNIQUE,                  -- '3.27', '7.18'
     image_id FK, sort_order int)
sign_translation(sign_id FK, locale, name, description, status, PK(sign_id, locale))
question_sign(question_id FK, sign_id FK, PK(question_id, sign_id))  -- savol↔belgi

-- Tuzilmali izoh (ekspert tahlili)
explanation(id uuid PK, question_id FK UNIQUE, legal_refs jsonb, created_at)
explanation_translation(explanation_id FK, locale,
     blocks jsonb,          -- tartiblangan bloklar: intro/muhim/eslatma/ogohlantirish/
                            -- maslahat/javob-tahlili(har variant, 2–5)/xulosa; belgi-chip
                            -- havolalari sign.code orqali
     status,                -- 'draft'(AI) | 'pending' | 'verified'
     verified_by uuid NULL, verified_at NULL, source, PK(explanation_id, locale))
explanation_feedback(profile_id FK, explanation_id FK, helpful bool, created_at,
     PK(profile_id, explanation_id))    -- "Tushunarsiz" oqimi admin-navbatga
```

**Invariantlar (import validation):** har savolda **2–5 javob** / aynan 1 to'g'ri (eski «aynan 4» invarianti real to'plamning ~77%'ini kvarantinga yuborardi — §4 taqsimotga qarab yumshatildi); har bilet aynan **20 savol** (o'zgarmadi — real manbaning 10-savollik "ticket"lari mapper tomonidan juftlab 20taga yetkaziladi, §4'ga qarang); M1 tillari verified; rasm sha256 mavjud; buzilgan yozuv `quarantined` — foydalanuvchiga chiqmaydi, taxminan to'ldirish YO'Q.

### 7.2. Profil, auth, obuna

```sql
profile(id uuid PK, phone text UNIQUE, name, region, district, birth_date NULL,
        locale_pref, theme_pref, role,            -- 'user'|'editor'|'admin'|'superadmin'
        referral_code text UNIQUE, referred_by uuid NULL,
        status,                                    -- 'active'|'banned'
        created_at)
otp_challenge(id uuid PK, phone, code_hash, channel,   -- 'telegram'|'sms'
        expires_at, attempts smallint, consumed bool, created_at)
telegram_account(profile_id FK PK, tg_user_id bigint UNIQUE, username, linked_at)
device(id uuid PK, profile_id FK, fingerprint, first_seen, last_seen)   -- anti-fraud
refresh_token(id uuid PK, profile_id FK, token_hash, expires_at, revoked_at NULL)

tariff(id uuid PK, code UNIQUE, days int, price_uzs int, old_price_uzs NULL,
       badge text NULL, sort_order, active bool)
tariff_translation(tariff_id FK, locale, name, description, PK(tariff_id, locale))

payment(id uuid PK, profile_id FK, tariff_id FK, amount_uzs, provider,  -- 'payme'|'click'|'sandbox'
        status,   -- 'created'|'pending'|'paid'|'failed'|'canceled'|'refunded'
        provider_txn_id, idempotency_key UNIQUE, promo_code_id NULL,
        created_at, paid_at NULL, meta jsonb)
entitlement(id uuid PK, profile_id FK, source,  -- 'purchase'|'promo'|'referral'|'admin'|'b2b'
        starts_at, ends_at, payment_id NULL, created_by NULL, note NULL)
        -- amaldagi VIP = max(ends_at) > now(); yangi pass mavjud passga qo'shiladi

promo_code(id uuid PK, code UNIQUE, kind,       -- 'percent'|'fixed'|'days'
        value int, max_uses, per_user_limit, valid_from, valid_to, active, created_by)
promo_redemption(id uuid PK, promo_code_id FK, profile_id FK, payment_id NULL, created_at)
referral_attribution(referee_id FK PK, referrer_id FK, created_at,
        reward_status,      -- 'pending'|'granted'|'rejected'  (birinchi to'lovdan keyin)
        fraud_flags jsonb)
limit_config(key text PK, free_value int, vip_value int, updated_at, updated_by)
        -- masalan: daily_practice_questions, exam_sim_per_day ...
```

### 7.3. O'quv holati

```sql
exam_session(id uuid PK, profile_id FK, mode,   -- 'variant'|'exam'|'practice'|'mistakes'|'grand_mock'
        variant_id NULL, category_id NULL, sign_id NULL, locale,
        time_limit_sec NULL, errors_allowed NULL, started_at, finished_at NULL,
        status,          -- 'in_progress'|'passed'|'failed'|'abandoned'
        score NULL, total int, stopped_reason NULL)
session_answer(session_id FK, question_id FK, answer_id, is_correct, answered_at,
        position smallint, PK(session_id, question_id))

variant_progress(profile_id FK, variant_id FK, best_correct int, attempts int,
        completed_at NULL, PK(profile_id, variant_id))
        -- ochilish qoidasi: N+1 ochiq ⇔ best_correct(N) ≥ unlock_threshold (default 10 — bilet
        -- 20 savolligicha qoladi, §4'ga qarang, shuning uchun bu qiym mos)
        -- GRAND MOCK ochiq ⇔ o'rtacha o'zlashtirish ≥ grand_mock_threshold (default 85%)

question_memory(profile_id FK, question_id FK, stability real, difficulty real,
        due_at, last_reviewed_at NULL, reps int, lapses int, state smallint,
        PK(profile_id, question_id))                 -- FSRS
category_mastery(profile_id FK, category_id FK, mastery real, seen int, correct int,
        updated_at, PK(profile_id, category_id))
saved_question(profile_id FK, question_id FK, created_at, PK(profile_id, question_id))
streak(profile_id PK, current int, best int, last_active_date date,
        daily_goal int, today_done int)
```

### 7.4. Tizim

```sql
audit_log(id uuid PK, actor_id FK, action, entity, entity_id, before jsonb, after jsonb,
          ip inet, created_at)                      -- barcha admin mutatsiyalari
event(id bigserial PK, profile_id NULL, anon_id NULL, name, props jsonb, ts)
          -- analitika: append-only, oy bo'yicha partitsiya; M7'da ClickHouse'ga ko'chishga tayyor
notification(id uuid PK, profile_id FK, kind, payload jsonb, channel, sent_at NULL, read_at NULL)
-- B2B (M5): org, org_member(org_id, profile_id, role), org_license(org_id, seats, starts_at, ends_at)
```

---

## 8. Auth oqimi (telefon + Telegram)

1. Foydalanuvchi telefon raqam kiritadi → server `otp_challenge` yaratadi va kodni **Telegram Gateway API** orqali yuboradi (SMS'dan ancha arzon). Telegram topilmasa → «SMS orqali yuborish» tugmasi (Eskiz/PlayMobile). Dev/sandbox rejimda kod konsolga/log'ga tushadi.
2. 6 raqamli kod tekshiriladi (TTL 5 daq, ≤5 urinish, qayta yuborish 60s cooldown, telefon+IP rate-limit) → profil topiladi/yaratiladi → JWT access (15 daq) + rotating refresh (30 kun, httpOnly cookie web'da).
3. Ixtiyoriy chuqur Telegram-bog'lash (bot `/link` kod) — bildirishnomalar va M4 quiz uchun.
4. Anti-fraud: `device` fingerprint yoziladi; referal mukofoti faqat referee birinchi to'lovidan keyin.

---

## 9. Billing

- **Pass modeli:** tarif = muddat (kun) + narx. To'lov muvaffaqiyatli → `entitlement` yaratiladi: `starts = max(now, joriy_end)`, `ends = starts + days`. Avto-yechib olish YO'Q (halollik + UZ bozorida recurring murakkab).
- **Provider adapterlari:** umumiy `PaymentProvider` interfeysi; Payme (Merchant JSON-RPC) va Click (Shop API) implementatsiyalari + **SandboxProvider** (dev/staging'da avto-to'laydi). Webhook: imzo tekshiruvi, **idempotentlik** (`provider_txn_id` + `idempotency_key`), holat mashinasi `created→pending→paid`, qayta urinishlarga chidamli.
- **Rekonsilyatsiya:** kunlik job provider hisobotini lokal to'lovlar bilan solishtiradi; farqlar admin-navbatga. «Pul keldi-yu obuna ochilmadi» = 0 bo'lishi majburiy.
- **Chek/fiskal:** to'lov yozuvi + kvitansiya UI; fiskal OFD integratsiya production-shartnoma bilan keladi (yuridik shaxs ochilgach).
- **Narx psixologiyasi (UI):** «kuniga ~X so'm», eski narx, tejash — onless kabi, lekin raqamlar admin-paneldan.

## 10. Promo va Referal

- **Promo-kod:** percent / fixed / bonus-kun; muddat, umumiy va har-foydalanuvchi limiti; checkout'da qo'llanadi; admin yaratadi/o'chiradi; har ishlatish `promo_redemption`da.
- **Referal:** har profilga kod + havola. Ro'yxatda kiritilsa `referral_attribution`. Mukofot (config): referee'ga chegirma %, referrer'ga bonus-kunlar — **faqat referee birinchi to'lovidan keyin**. Anti-fraud: bir qurilma/telefon takrorlari, tezlik limitlari, shubhalilar admin ko'rigiga.

---

## 11. O'quv yadro (M1 mexanika)

| Rejim | Savollar | Vaqt | Feedback | Qoida |
|---|---|---|---|---|
| Bilet | 20 (variantdan, tartibda — real manba 10-savollik "ticket"lardan juftlanadi) | yo'q | darhol + izoh | N+1 ochilishi: bilet N'da ≥10/20. Free: faqat 1-bilet |
| Imtihon | 20 tasodifiy | 25 daq | oxirida | ≤2 xato o'tdi; 3-xatoda stop; vaqt tugasa avto-submit |
| Mashq | kategoriya/belgi bo'yicha | yo'q | darhol + izoh | free-limit: kuniga N savol (config) |
| Xatolar banki | xato qilinganlar | yo'q | darhol + izoh | Leitner: ketma-ket to'g'ri → chiqadi |
| GRAND MOCK (M2) | 20 | 25 daq | oxirida | ochilish: o'rtacha o'zlashtirish ≥85% (config) |

- **Imtihon UI (real parite):** F1–F5 javob klavishalari (savoldagi javob soniga dinamik mos — real to'plamda 2–5 variant), 1–20 navigator (javob berilgan/berilmagan/belgilangan), test ichida til almashtirish, shrift +/−, fullscreen, rasmni kattalashtirish (zoom — savollarning ~58% rasmli, mobilda majburiy ehtiyoj), savolni saqlash, «Ekspert tahlili» tugmasi (mashq rejimlarida hamda imtihon yakunidagi natija-ko'rikda har savol bo'yicha).
- **Anti-cheat:** imtihonda savollar `is_correct`siz keladi; baholash faqat serverda; natija `finish`da.
- **Uzilishga chidamlilik:** javoblar lokal navbatga yoziladi va qayta ulanishda sync; sessiya `resume` qilinadi.
- **Kunlik maqsad + streak:** default 10 savol/kun (config), nazokatli premium ohang.

## 12. Learning Engine

FSRS xotira modeli (har profil×savol), `GET /learn/next` — due savollar; weak-area fokus (lapses/xato-darajasi bo'yicha); interleaving; kategoriya mastery-map; **«Imtihonga tayyorlik %»** prognozi (mastery + due + oxirgi natijalardan). Hammasi server-side, unit-testlar referens implementatsiyaga solishtiriladi.

## 13. Ekspert tahlili (izohlar)

- **Format:** tartiblangan bloklar (jsonb): kirish, MUHIM, ESLATMA, OGOHLANTIRISH, MASLAHAT, har-javob-tahlili (savoldagi har variant uchun — 2–5 ta, to'g'ri/noto'g'ri sababi), YHQ modda-havolalari, biriktirilgan belgilar (sign chiplar rasm bilan), misollar.
- **Workflow:** AI-draft (import paytida yoki admin buyrug'i) → ekspert tahriri → `verified`. Foydalanuvchi faqat verified ko'radi.
- **Draft-xomashyo (real to'plam):** importdagi mavjud `comment` maydoni (1219/1235 savolda, aksariyati YHQ modda-havolali) AI-draft uchun boshlang'ich manba — noldan generatsiya emas: mapper `comment`ni kirish-blok sifatida ko'chiradi, undagi YHQ havolalarini `legal_refs`ga ajratadi, AI faqat tuzilmalash/boyitish (bloklar, har-javob-tahlili) qiladi. Bu ekspert-verify hajmini ham keskin kamaytiradi.
- **Sifat sikli:** «Foydali bo'ldimi?» ovozi; past-reyting izohlar admin qayta-ishlash navbatiga (M3 UI, model M1'da).

---

## 14. API dizayni (asosiy endpointlar)

Baza `/api/v1`, JSON, JWT. Kontent endpointlari ETag/Cache-Control bilan.

- **Auth:** `POST /auth/otp/request` (phone, channel) · `POST /auth/otp/verify` → tokens · `POST /auth/refresh` · `POST /auth/logout`
- **Profil:** `GET/PATCH /me` · `GET /me/entitlement` · `GET /me/payments` · `POST /me/telegram/link`
- **Kontent:** `GET /categories` · `GET /variants` (+progress) · `GET /variants/{n}` · `GET /signs?group=` · `GET /signs/{code}` (+savollari) · `GET /questions/{id}` (+verified izoh)
- **Sessiya:** `POST /sessions` · `POST /sessions/{id}/answers` · `POST /sessions/{id}/finish` · `GET /sessions/{id}` (resume) · `GET /me/sessions` (tarix)
- **Learning:** `GET /learn/next` · `POST /learn/review` · `GET /me/mistakes` · `GET/POST/DELETE /me/saved` · `GET /me/stats` (mastery, streak, tayyorlik %)
- **Billing (M2):** `GET /tariffs` · `POST /checkout` (tariff, promo?) → provider URL/deep-link · `POST /webhooks/payme|click` · `POST /promo/validate`
- **Referal (M2):** `GET /me/referral` · ro'yxatda `referral_code` param
- **Events:** `POST /events` (batch)
- **Admin (M3):** `/admin/**` — kontent CRUD+verify, import/export, users, billing/refund, tariffs/promo/limits, izoh-navbat, analytics, broadcast, audit — RBAC + audit-log

---

## 15. Flutter arxitekturasi (feature-first, Clean)

```
lib/
  app/        # router, DI, theme (design system), l10n
  core/       # network(Dio+interceptors), error/Result, utils, event-logger
  features/
    auth/  home/  variants/  exam/  practice/  mistakes/  learn/
    results/  stats/  explanation/  signs/  saved/  billing/  referral/
    profile/  settings/
    (har biri: data/ domain/ presentation/)
  shared/widgets/   # QuestionCard, AnswerOption(+F1-F5, javob soniga dinamik), CountdownTimer,
                    # QuestionGrid, MasteryBar, ResultRing, SignChip,
                    # CalloutBlock, TariffCard, EmptyState
```

State: Riverpod. Modellar: freezed. Testlar: unit + widget + integration (to'liq imtihon oqimi).

## 16. UI/UX konsepsiyasi

Minimal-premium, Material 3 + «Apple-clean»; dark (default) + light; 60/120fps; responsive (mobil-web→desktop). Onless'dan sezilarli sayqalliroq: boy empty-state'lar, silliq mikro-animatsiyalar, aniq progress-vizualizatsiya (bilet grid + mastery ring), izoh-bloklarning chiroyli tipografiyasi. Accessibility: AA kontrast, katta tap-target, screen-reader, klaviatura navigatsiyasi (F1–F5 + strelkalar). Keyingi qadam: kodlashdan oldin interaktiv HTML mockup tasdiqlash uchun.

## 17. Ommaviy SSG sayt (M2)

Astro: landing, narxlar (API'dan build-time), jarimalar ma'lumotnomasi, belgilar katalogi (SEO-trafik), oferta/maxfiylik/qaytarish siyosati sahifalari. App bilan bitta dizayn-til. `app.` subdomen — Flutter ilova; asosiy domen — SSG.

---

## 18. Super Admin (M3 — qamrov)

Kontent-studio (savol/javob/rasm/bilet/belgi/izoh CRUD, verify-workflow, quarantine, import/export JSON/CSV/Excel, media-kutubxona, versiya-tarix); Foydalanuvchilar (qidiruv, profil, entitlement berish/olish, ban, GDPR-o'chirish); Billing (tranzaksiyalar, refund, rekonsilyatsiya, eksport); Narx-boshqaruv (tariflar, promo, referal-config, limitlar); Izoh-sifat navbati; Analitika: daromad, DAU/MAU, retention kohortalar, konversiya voronkasi (tashrif→ro'yxat→trial→to'lov), ARPU/LTV/churn, kontent-qiyinlik (savol bo'yicha xato % — sifat signali); Broadcast (in-app + Telegram); RBAC (superadmin/admin/editor/support), to'liq audit-log, feature-flags.

## 19. Analitika (investor-tayyor)

Client'dan batch eventlar (`view_question`, `answer`, `session_finish`, `paywall_view`, `checkout_start/paid`, `referral_used`...) → Postgres partitsiyalangan `event` → Metabase/Grafana dashboardlar. M7'da hajm oshsa ClickHouse. Barcha biznes-metrikalar (MRR, konversiya, retention D1/D7/D30, LTV/CAC) shu oqimdan hisoblanadi.

## 20. Xavfsizlik va compliance

JWT+rotating refresh (httpOnly), server-side baholash (anti-cheat), rate-limit (Redis), input-validation, parametrik SQL (sqlc), XSS/CSP/HSTS/CORS, secrets manager, signed URL (media, kerak bo'lsa), webhook imzo + idempotentlik, RBAC + audit. **Yuridik:** shaxsiy ma'lumotlar UZ-serverda; ommaviy oferta, maxfiylik, qaytarish siyosati (M2 SSG'da); kontent-litsenziya hujjatlashtiriladi. OTP xavfsizligi: hash-langan kod, TTL, urinish-limit.

## 21. Testlar

Go: unit (scoring, FSRS, unlock-qoidalar, promo/referal hisob-kitob, webhook idempotentlik), integration (testcontainers: API+Postgres+Redis), provider-mock to'lov oqimi E2E. Flutter: unit (usecase/VM), widget (savol karta, taymer, F1–F5, izoh-bloklar), integration (to'liq imtihon + to'lov-sandbox oqimi). Import: invariant testlari. Performance: Lighthouse budget, k6 load (M7 to'liq). TDD — har feature test-first.

## 22. CI/CD va muhitlar

GitHub Actions: lint (golangci-lint, dart analyze) → test → build (Docker image, Flutter web, Astro) → deploy. Muhitlar: dev (compose), staging (sandbox-to'lovlar), prod (UZ VPS, Docker; images CDN). Migratsiya avtomatik (golang-migrate). Monitoring: healthcheck + structured logs + uptime alert (M7'da kengayadi).

---

## 23. M1 qurish tartibi

1. Repo + git + compose (Postgres/Redis/MinIO) + Go skeleton + Flutter skeleton + CI
2. DB migratsiyalar (§7 to'liq — billing jadvallari ham, ishlatish M2'da) + sqlc
3. Import pipeline + real kontent seed (savollar/biletlar/belgilar/rasmlar, validation-hisobot)
4. Auth (OTP: sandbox-kanal + Telegram Gateway adapter) + profil
5. Content API + belgilar katalogi + testlar
6. Session/scoring (4 rejim + bilet-unlock + resume) + testlar
7. FSRS learning engine + stats/tayyorlik % + testlar
8. Flutter: theme/design-system, auth oqimi, home
9. Imtihon simulyator UI (F1–F5, navigator, taymer) + bilet/mashq/xatolar oqimlari
10. Izoh-render (bloklar) + AI-draft generator + saqlanganlar + streak
11. Free-limit enforcement + entitlement tekshiruv + event-logging
12. E2E + performance + staging deploy

## 24. Ochiq savollar

- Qoraqalpoqcha kontent manbasi (topilgach `kaa` yoqiladi)
- Real data eksport formati — **hal bo'ldi (2026-07-19):** avtoimtihon-strukturali `questions.<locale>.json` (id, question, image, comment, answers[2–5], correct_answer 1-based, ticket "1".."124") + `quiz-images/*.webp` (~715 fayl); mapper shu formatga moslanadi, importer invariantlari yangilandi (§7.1)
- Tarif raqamlari (admin-panelda kiritiladi; onless'dan arzon pozitsiya)
- Yuridik shaxs muddati → production to'lov kalitlari
- Battle real-time va Telegram-bot arxitekturasi (M4 spec'ida)
- B2B litsenziya modeli detallari (M5 spec'ida)
