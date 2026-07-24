# AvtoTest — To'liq Roadmap (M2 → M7, keyin M3 Admin oxirida)

> Maqsad: M1 dan keyingi barcha bosqichlarni **kichik, detalli, parallel bajarish mumkin** bo'lgan bo'laklarga ajratish, toki kelajakda har bir bo'lakni alohida AI-sessiyasi (yoki parallel bir nechta subagent) bajara olsin — AI-limitiga mos, tez va sifatli.
>
> Sana: 2026-07-24. Manba: `AVTOTEST-MASTER-PROMPT.txt` (yagona haqiqat manbai) + joriy kod holati.

---

## 0. Bu hujjatdan qanday foydalaniladi (AI-execution qo'llanmasi)

- **Ierarxiya:** `Milestone (M2…)` → `Plan (M2-01…)` → `Task (kichik, 1 sessiya-hajmli)`.
- **Har bir Plan = bitta to'liq sikl:** `brainstorming` (spec) → `writing-plans` (reja) → implementatsiya → `requesting-code-review`. Bu roadmap spec o'rnini bosmaydi — u **navigatsiya xaritasi**; har Plan bajarilishidan oldin o'z detalli spec'ini oladi.
- **Parallellik qoidasi:** har feature-Plan avval **API-kontraktni** belgilaydi (kichik umumiy task), keyin **backend va frontend parallel** ketadi. Mustaqil Plan'lar butunlay parallel.
- **Har task uchun javob berilishi kerak:** nima qiladi, qanday ishlatiladi, nimaga bog'liq. Task ~1 AI-sessiyaga sig'sin (juda katta bo'lsa — bo'lakla).
- **Standartlar (har task uchun majburiy):** testlar (backend Go test `-p 1`, frontend Vitest + Playwright), `make check` yashil, 3 til i18n (hardcode yo'q), dizayn-tizim (6.0-bo'lim: dark+indigo+Duolingo, yashil emas), nullable-safe, seam-testing.
- **Bajarish tartibi:** M2 → M4 → M5 → M6 → M7 → **M3 (Admin) ENG OXIRIDA** (foydalanuvchi qarori: hamma narsa qurilgach, panel unga moslab quriladi).

---

## 1. Joriy holat (2026-07-24) — nima tayyor

**Backend (M1 to'liq):** auth (telefon+OTP, JWT+refresh), sessiya/scoring (20/25daq/≤2xato), FSRS o'quv dvigateli, entitlement/VIP gating, kontent API (savol/bilet/kategoriya/belgi), izohlar (AI-draft stub), saqlangan/streak/events, demo-endpoint.

**Kontent (M1 to'liq):** 1231 savol (3 til, hammasi valid), 62 bilet (61×20 + 62-bilet 11), 285 belgi (3 til, rasm), 13 kategoriya.

**Frontend (Next.js, M1 asosan tayyor):** landing, login/verify, dashboard, tickets, session (full-screen test engine), practice, signs, mistakes, saved, stats, profile, premium (statik), exam-mockup.

**Sxema TAYYOR, mantiq YO'Q (M2 uchun muhim):** `tariff`, `tariff_translation`, `promo_code`, `promo_redemption`, `payment`, `entitlement`, `referral_attribution`, `notification`, `audit_log`, `limit_config` jadvallari migratsiyalarda mavjud, bo'sh. Ya'ni M2 backend = **mavjud sxemaga mantiq yozish**, sxema qurish emas.

---

## 2. M2 — MONETIZATSIYA  🎯 (keyingi, eng katta biznes-qiymat)

**Maqsad:** to'lov, tarif, promo, referal, GRAND MOCK, mehmon-demo. Yuridik shaxs yo'q → **sandbox** rejimda quriladi.

**Parallel-guruhlar:** `M2-01` avval (poydevor). Keyin `A={M2-02, M2-03, M2-05, M2-07, M2-11, M2-08}` parallel. Keyin `B={M2-04}` (02/03 dan keyin). Keyin `C={M2-06, M2-09, M2-10}`.

| Plan | Nomi | Bog'liqlik | Backend/Frontend |
|------|------|-----------|------------------|
| M2-01 | Tarif modeli + narx logikasi | — | BE |
| M2-02 | Payme integratsiyasi (sandbox) | 01 | BE |
| M2-03 | Click integratsiyasi (sandbox) | 01 | BE |
| M2-04 | To'lov→entitlement grant + tarix | 02,03 | BE |
| M2-05 | Promo-kodlar | 01 | BE |
| M2-06 | Referal dasturi (anti-fraud) | 04 | BE |
| M2-07 | GRAND MOCK | (FSRS/stats) | BE+FE |
| M2-08 | Tarif UI (mashina-brend kartalar) | 01 | FE |
| M2-09 | Checkout oqimi (tanlash→promo→to'lov→qaytish) | 04,08 | FE |
| M2-10 | To'lov tarixi + cheklar + referal sahifasi | 04,06 | FE |
| M2-11 | Mehmon-demo (landing funnel) | (demo-endpoint) | FE |

### M2-01 Tarif modeli + narx logikasi (BE)
- T1: `tariff`/`tariff_translation` uchun sqlc so'rovlar + `GET /tariffs` (locale, active, sort). Narx psixologiyasi: `price_uzs`, `old_price_uzs`, `badge`, kunlik-narx hisobi (`price/days`).
- T2: Seed — mashina-nomli tariflar (Matiz=bepul, Nexia, Gentra, Malibu = premium darajalar), muddatlar 7/15/30/45/75 kun (master prompt). `cmd/gentariffs` yoki seed JSON.
- T3: Testlar: locale-fallback, active-filter, narx hisoblari.

### M2-02 Payme integratsiyasi (sandbox) (BE)
- T1: Payme Merchant API (JSON-RPC) endpoint `POST /billing/payme`: `CheckPerformTransaction`, `CreateTransaction`, `PerformTransaction`, `CancelTransaction`, `CheckTransaction`, `GetStatement`.
- T2: `payment` state-machine (created→pending→paid/cancelled), idempotency (`idempotency_key`, `provider_txn_id`), auth (Payme Basic-Auth), summa validatsiyasi.
- T3: Xatolik kodlari (Payme spec), testlar (har RPC metodi + qayta-urinish + noto'g'ri summa).

### M2-03 Click integratsiyasi (sandbox) (BE)
- T1: Click Shop API `POST /billing/click`: `Prepare`, `Complete`, imzo tekshirish (MD5 sign), `merchant_trans_id`.
- T2: `payment` state-machine bilan integratsiya (02 bilan bir xil model), idempotency.
- T3: Testlar (imzo, prepare/complete, error kodlari).

### M2-04 To'lov→entitlement grant + tarix (BE)
- T1: To'lov `paid` bo'lganda → `entitlement` yaratish/uzaytirish (source='purchase', tariff.days), VIP-stacking (mavjud logika).
- T2: `GET /me/payments` (tarix), chek ma'lumoti (summa, tarif, sana, provider).
- T3: Testlar: grant, stacking, idempotent qayta-webhook.

### M2-05 Promo-kodlar (BE)
- T1: Validatsiya (`kind` percent/fixed/days, `valid_from/to`, `max_uses`, `per_user_limit`), checkout'da narxga qo'llash.
- T2: `promo_redemption` yozuvi (payment bilan bog'liq), anti-fraud (per-user limit).
- T3: `POST /billing/promo/validate` (checkout preview), testlar.

### M2-06 Referal dasturi (BE)
- T1: Referal havola/kod (profile.referral_code mavjud), `referral_attribution` (referee→referrer).
- T2: **Mukofot faqat referee birinchi TO'LOVIDAN keyin** (`reward_status` pending→granted), anti-fraud (self-referral, bir qurilma).
- T3: `GET /me/referrals` (statistika), testlar.

### M2-07 GRAND MOCK (BE+FE)
- T1 (BE): Ochilish sharti — o'rtacha o'zlashtirish ≥85% (`/me/stats` mastery). To'liq imtihon-mock sessiya turi.
- T2 (FE): Locked/unlocked holat UI, mock oqimi (natija ekrani), konfetti.
- T3: Testlar (eligibility, mock scoring).

### M2-08 Tarif UI (FE)
- T1: Tarif-kartalar — mashina-brend, "kuniga ~X so'm" freyming, eski narx (chizilgan) + tejash %, "eng ommabop" badge, dizayn-tizim (pill, 3D).
- T2: Premium sahifasini qayta qurish (statik → API'dan tariflar). i18n (3 til).

### M2-09 Checkout oqimi (FE)
- T1: Tarif tanlash → promo qo'llash (validate) → provider tanlash (Payme/Click) → redirect.
- T2: Qaytish/callback sahifalari (success/failure/pending), holat polling.
- T3: E2E test (sandbox to'lov oqimi).

### M2-10 To'lov tarixi + referal sahifasi (FE)
- T1: To'lov tarixi ro'yxati + chek ko'rinishi.
- T2: Referal havola + ulashish + statistika (nechта taklif, nechta to'lov, mukofot).

### M2-11 Mehmon-demo (FE)
- T1: Landing'da ro'yxatsiz 1-bilet demo (mavjud `POST /demo/answer` rate-limitli endpoint), funnel → ro'yxatdan o'tishga chorlov.

---

## 3. M4 — GROWTH (o'sish, retention)

**Maqsad:** Battle Arena PvP, leaderboard, Telegram bot, push.

**Parallel-guruhlar:** `{M4-01, M4-06}` parallel (mustaqil). `M4-03` (Arena infra) katta — o'zi bir necha task. `M4-06`→`M4-07`.

| Plan | Nomi | Bog'liqlik | BE/FE |
|------|------|-----------|-------|
| M4-01 | Leaderboard (Redis sorted-set, kunlik/haftalik/oylik/all-time) | — | BE |
| M4-02 | Leaderboard UI | 01 | FE |
| M4-03 | Battle Arena — realtime infra (WebSocket, matchmaking) | 01? | BE |
| M4-04 | Battle Arena — match logic + medallar (Bronza→Brilliant, rating) | 03 | BE |
| M4-05 | Battle Arena UI (matchmaking, jonli 1v1, natija, do'stni chaqirish) | 03,04 | FE |
| M4-06 | Telegram bot — poydevor (auth-link, komandalar) | — | BE |
| M4-07 | Telegram bot — kunlik quiz + bildirishnomalar | 06 | BE |
| M4-08 | Push-kampaniyalar / notifications (web push) | 06? | BE+FE |

### M4-03 Battle Arena infra (katta — bo'laklar)
- T1: WebSocket server (gorilla/ws yoki nhooyr), ulanish auth (JWT), connection registry.
- T2: Matchmaking navbati (Redis), juftlash (rating-bo'yicha), match-yaratish.
- T3: Match state-machine (savol sinxron yuborish, javob qabul, timer, scoring), disconnect/reconnect.
- T4: Testlar (matchmaking, match oqimi, disconnect).

### M4-04 Match logic + medallar
- T1: Rating (ELO-uslub), medal darajalari (Bronza/Kumush/Oltin/Platina/Brilliant) progressiyasi.
- T2: Match tarixi, g'olib/mag'lub, seriya (streak). Testlar (reference qiymatlar bilan).

*(M4-05..08 — spec'lar bajarilishdan oldin detallashadi.)*

---

## 4. M5 — B2B (avtomaktablar)

**Maqsad:** tashkilotlar, guruh litsenziyalari, o'qituvchi dashboardi.

| Plan | Nomi | BE/FE |
|------|------|-------|
| M5-01 | Organization modeli (school, seat, group-license); `entitlement.source='b2b'` | BE |
| M5-02 | Guruh a'zolari + litsenziya boshqaruvi (invite/enroll, seat count) | BE+FE |
| M5-03 | O'qituvchi dashboardi (guruh ro'yxati, taraqqiyot) | FE |
| M5-04 | Guruh statistikasi/hisobotlar (o'zlashtirish, faollik, eksport) | BE+FE |

*(Talablar B2B mijoz topilganda aniqlashadi — hozircha yuqori darajali.)*

---

## 5. M6 — MOBIL (PWA)

**Qaror:** PWA tavsiya etiladi (master prompt: PWA M6'gacha yetarli). React Native — faqat zarurat bo'lsa.

| Plan | Nomi |
|------|------|
| M6-01 | PWA poydevor (manifest, service-worker, o'rnatiladigan, offline shell) |
| M6-02 | Offline kontent (savol/belgi cache — offline mashq), sinxronlash |
| M6-03 | Web-push bildirishnomalar (agar M4-08 da qilinmagan bo'lsa) |

---

## 6. M7 — MIQYOS (production-tayyorlik)

| Plan | Nomi |
|------|------|
| M7-01 | Observability (metrics/Prometheus, structured logs, tracing, alerting) |
| M7-02 | Load-test (k6) + performance (DB index audit, N+1, cache) |
| M7-03 | Xavfsizlik auditi (authz review, rate-limit audit, dependency scan, secrets, OWASP checklist) |
| M7-04 | Backup + DR (DB + blob backup, restore-drill, retention) |

---

## 7. M3 — SUPER ADMIN  🔒 (ENG OXIRIDA — foydalanuvchi qarori)

Hamma narsa qurilgach quriladi, toki panel butun mahsulotni boshqarishga moslashsin. 8 quyi-tizim (poydevor `RBAC` + `audit-log` birinchi):
kontent-studio (savol/izoh/belgi CRUD + verify + import/export — **eslatma:** importer session-referenced savollarni delete/reimport qilolmaydi, admin-studio smarter-upsert yo'li kerak), foydalanuvchilar boshqaruvi, billing/refund/rekonsilyatsiya, narx/promo/limit sozlash, izoh-sifat navbati, INVESTOR DASHBOARD (MRR/DAU/MAU/retention/voronka/LTV/churn), RBAC, audit-log.

---

## 8. Umumiy eslatmalar (cross-cutting)

- **Dizayn-tizim** (6.0): dark (#0E1526) + indigo aksent (#6C63FF), semantik ranglar (yashil=to'g'ri, qizil=xato, olov=streak, oltin=reyting), pill+3D tugmalar, Baloo2/Manrope, framer-motion. Yashil brend RANGI EMAS.
- **i18n:** har matn uz-Latn/uz-Cyrl/ru (next-intl), hardcode yo'q.
- **Testlar:** har Plan review'dan o'tadi (`requesting-code-review`); TDD tavsiya.
- **Sandbox→prod:** to'lovlar yuridik shaxs ochilgach production'ga o'tadi (kod tayyor turadi).
- **Parallel bajarish:** mustaqil Plan'larni bir vaqtda subagentlarga bering; feature ichida API-kontrakt belgilangач BE+FE parallel.

---

## 9. Bajarilish tartibi + vaqt baholari (BOSHLASH REJASI)

**Birlik:** `1 sessiya` = bitta fokuslangan spec→qurish→test→review sikli (~yarim kun AI + review). `S`≤1, `M`=1–2, `L`=3+ sessiya. **Wall-clock** = parallel treklar hisobga olinganda taxminiy davomiylik (sizning tempingizga bog'liq). Baholar **taxminiy** — tashqi bog'liqliklar (Payme/Click sandbox kaliti, yuridik shaxs, WebSocket-hosting) ta'sir qiladi.

### Umumiy tartib (nega shunday)
Revenue eng muhim → **M2 birinchi**. Kritik yo'l "foydalanuvchi to'lay oladi va VIP oladi" ni eng tez yopadi; qolgani atrofda parallel. Keyin retention (**M4**), so'ng **M5/M6/M7**, **admin oxirida**.

### M2 — Monetizatsiya (~13 sessiya; parallel bilan wall-clock ~1–1.5 hafta)

| Wave | Plan (parallel treklar) | Effort | Nima chiqadi |
|------|------------------------|--------|--------------|
| **0 — Poydevor** | M2-01 tarif modeli (solo) | S (1) | Tariflar API + seed; barcha M2 ochiladi |
| **1 — Kritik yo'l** (parallel) | A: M2-02 Payme · B: M2-08 tarif UI · C: M2-11 demo + M2-05 promo | M/M/S+S (~4) | To'lov provayder + narx kartalar + promo |
| **2 — Halqani yopish** | M2-04 grant→entitlement · M2-09 checkout (E2E) | S+M (~2.5) | **Payme bilan revenue to'liq ishlaydi — SHIPPABLE** ✅ |
| **3 — To'ldirish** (parallel) | M2-03 Click · M2-06 referal · M2-07 GRAND MOCK · M2-10 tarix/referal UI | M+S+M+S (~5) | Ikkinchi provayder, referal, mock, tarix — **M2 tugadi** |

> **Eng tez revenue nuqtasi:** Wave 0→1A→2. Ya'ni ~4–5 sessiyada Payme orqali pul qabul qilinadi. Qolgan hammasi shundan keyin parallel.
> **Tashqi bloklar (oldindan tayyorlang):** Payme & Click sandbox merchant kalitlari; prod uchun yuridik shaxs (sandbox hozir yetarli).

### M4 — Growth (~12.5 sessiya; wall-clock ~1.5–2 hafta)

| Guruh | Plan | Effort |
|-------|------|--------|
| Leaderboard (parallel Arena bilan) | M4-01 BE (S) → M4-02 UI (S) | ~2 |
| **Battle Arena** (eng og'ir) | M4-03 realtime infra (L, 3) → M4-04 match/medal (M) → M4-05 UI (M, 2) | ~6.5 |
| Telegram bot (mustaqil, parallel) | M4-06 poydevor (S) → M4-07 quiz/notif (M) | ~2.5 |
| Push | M4-08 (M) | ~1.5 |

> Arena eng katta risk — uni **erta prototiplang** (WebSocket-hosting qarori). TG bot va leaderboard Arena bilan to'liq parallel.

### M5 — B2B (~7 sessiya) · M6 — PWA (~3.5) · M7 — Miqyos (~5)
Bu bosqichlar talab-aniqlashtirilganda detallashadi; hozircha Plan-darajali. M5 real B2B mijoz topilganda; M7 prod-launchdan oldin.

### M3 — Super Admin (~13 sessiya, ENG OXIRIDA)
8 quyi-tizim. RBAC + audit-log poydevor (M, ~2), keyin har vertikal (~1.5 dan).

### Jami (adminGACHA): ~41 sessiya
Parallel 2–3 trek bilan wall-clock **~4–6 hafta faol ish** (sizning review tempingiz + tashqi bloklarga bog'liq). Admin +~13 sessiya (~1.5 hafta).

### Tavsiya: NIMADAN BOSHLAYMIZ
**M2-01 (tarif modeli)** — solo, hech narsaga bog'liq emas, 1 sessiya, va butun M2 ni ochadi. Spec (brainstorming) → reja (writing-plans) → qurish. Undan keyin darhol Wave 1 ni parallel subagentlarga taqsimlaymiz.
