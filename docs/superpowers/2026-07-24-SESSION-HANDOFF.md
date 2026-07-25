# SESSION HANDOFF — bu yerdan boshlang (yangilangan 2026-07-26 — U-50 after Sentry SDK + U-39 variant cache)

> Yangi sessiya (yoki boshqa AI) uchun: bu hujjat **aniq holat + keyingi aniq qadam**ni beradi. Avval buni o'qing, keyin ishlang. Bu hujjat repo'ga committed — Claude Code'ning session-memory tizimidan farqli, har qanday AI/vosita buni o'qiy oladi.
>
> **Unfinished SoT:** `docs/superpowers/specs/2026-07-26-full-project-unfinished-inventory.md` (U-xx). Agar handoff §4/J-table bilan inventory zid kelsa — **kod + inventory** yutadi.

---

## ⚡ HOZIRGI HOLAT (2026-07-26) — YANGI SESSIYA SHU YERDAN

### Git
- Branch: **`main`** (remote bilan sync — `git pull`).
- HEAD o‘qish: `git log --oneline -1` (bu fayl committed bo‘lgani uchun hash shu yerda muzlatilmaydi).

### Qulflangan mahsulot/dizayn qarorlari
| Qaror | Qiymat |
|--------|--------|
| Brand (UI) | **Driver Go** (AvtoTest UI da ko‘rinmaydi) |
| Yo‘nalish | **A — Asphalt & Signal** (amber CTA, indigo taqiqlangan) |
| Hero | Full-bleed asphalt + bitta phone mock |
| Sidebar | 6 primary + **Ko‘proq** |
| Scope tashqari | Rasmiy imtihon desktop view **locked**; no seed wipe; no purple |

### Dizayn hujjatlari
1. `docs/superpowers/specs/2026-07-25-driver-go-design-system.md` — **SOURCE OF TRUTH** (A–J)
2. Inventory: `docs/superpowers/specs/2026-07-26-full-project-unfinished-inventory.md`

### Dizayn / Growth — kodga mos holat (U-50)

| Phase | Nima | Holat |
|-------|------|-------|
| J0–J7, J9 | Tokens → chrome → session interior | ✅ |
| J8 | Figma SoT | ⬜ ixtiyoriy |
| J10 / M4-05 | Arena UI | ✅ |
| M4-01/02 | Leaderboard BE + UI | ✅ |
| M4-03/04 | Arena infra + rating/history | ✅ |
| M4-06 | Telegram bot poydevor + FE bog‘lash (U-09) | ✅ |
| M4-07 | TG daily quiz + notif | ⬜ **skipped** (U-10 — no tiny vertical) |
| M4-08 | Web push | **partial** (foundation + FSRS digest + admin broadcast stub; VAPID ops) |
| M3 | Super Admin | **partial** M3-0…M3-7 + audit + broadcast + limits + **inbox** + **homepage CMS** + **ops feed/alerts** + **investors stub** |
| M5 | B2B | **partial** U-40 admin grant + teacher read portal (`/teacher`) |
| M6 | PWA | **partial** (shell + meta/CMS + recently-opened **variant detail** cache; **U-39 partial** — full offline exam still open) |

### Shu to‘lqinda yopilgan (2026-07-26 #5 — Sentry + variant cache)
- **U-41** optional Sentry SDK (FE+BE) when `SENTRY_DSN` / `NEXT_PUBLIC_SENTRY_DSN` set; empty = no-op; **no pager**
- **U-39** recently-opened ticket `variants/{n}` SW cache + prefetch (not full offline exam)
- **M4-07 / U-10** re-checked — still **skipped** (design defers quiz/cron; no tiny vertical)
- Prior #4 still stands: k6 smoke, backup drill, Prometheus `/metrics`, site CMS cache

### Footer aloqa — CMS (U-17 / M3-4)
Public `GET /site/contacts` + Admin `/{locale}/admin/cms/chrome`. Homepage hero: `/{locale}/admin/cms/home` + `GET /site/home`. Bo‘sh maydonlar Landing i18n. Ops `/ops/*` deprecated bridges.

### Keyingi sessiya uchun aniq birinchi buyruq
```text
Inventory: pick smallest honest complete still open (legal CMS / content signs stub /
U-35 admin credential PDF / U-27 live recon when keys exist / FE Next majors).
Skip external: U-03 keys, U-02 host, U-12 LLM, inventing U-10 quiz, B2B school sales,
full offline exam sync, inventing Metabase/Grafana / pager.
Handoff: docs/superpowers/2026-07-24-SESSION-HANDOFF.md §⚡
Inventory: docs/superpowers/specs/2026-07-26-full-project-unfinished-inventory.md
```

### Qoldiq (tashqi / katta — user/secrets yoki huge product)
- Payme/Click **prod** keys + yuridik shaxs (U-03)
- Staging remote host / registry (U-02 D18)
- Real LLM explanations (U-12); M4-07 quiz (U-10 skipped until scoped)
- Full offline exam sync (U-39 remainder) — **large**
- BI Metabase/Grafana (U-46 depth); tracing / **pager** (U-41 remainder after SDK)
- Off-site backup/WAL + soak load-test on real host (U-44/U-42 remainders; need U-02)
- B2B school sales / self-serve seats

---

## 0. Maqsad (kontekst)
AvtoTest — O'zbekiston YHQ imtihoniga tayyorlovchi **pullik onlayn maktab-startap** (onless.uz/osonprava.uz analogi, "10-15x kuchli"). Go backend + Next.js frontend. Manba-hujjat: repo ildizida `AVTOTEST-MASTER-PROMPT.txt`. To'liq roadmap: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md`. UI brand: **Driver Go**.

## 1. Audit qilingan holat (2026-07-26 snapshot)
- Git: `main`. Aniq HEAD: `git log --oneline -1`.
- Backend: `go build ./...`; `make test` / `make test-parallel` — paket izolyatsiyasi (§1.-2).
- Frontend: `npm run typecheck` / `lint` / `test` (vitest) + Playwright smoke CI.
- DB migratsiya: **0035** gacha (`alert_rule`); dirty emas. Support tickets `0034`.
- Kontent: ~1231–1235 savol (3 til), ~61–62 bilet, 15 biletsiz leftover (U-29), signs catalog.
- **`./run.sh`** — Docker infra + backend (:8090) + frontend (:3000).

### 1.-1 … 1.1 (tarixiy audit yozuvlari)
Quyidagi bo‘limlar **2026-07-25** sessiyalarining saqlangan audit jurnalidir. Ular o‘zgartirilmagan; yangi ish inventori U-xx jadvalida.

<details>
<summary>Tarixiy audit (2026-07-25) — ochish</summary>

### 1.-1 IKKINCHI AUDIT RAUNDI — 7 ta topilma
Critical #1 promo-limit sequential bypass → ProcessPaymentGrant re-check + proRatedDays.
Critical #2 parallel GrandDays → LockProfileForGrant.
#3 Grand Mock mastery exploit → `grand_mock_min_studied_pct` (0018).
#4 Grand Mock paywall → `402 vip_required` vs `403 mock_not_eligible`.
#5 referral `?ref=` + verify localStorage → `referral-storage.ts` + ReferralCapture.
#6 payment history i18n + refunded badge.
#7 payment tariff snapshots (0019).

### 1.-2 TEST INFRATUZILMASI
Paketga xos Postgres + Redis DB indekslari; `make test-parallel` yashil.

### 1.0 M4-01 Leaderboard
Redis sorted-set + RebuildPeriod daily-cap approximation (U-22 — documented low-risk).

### 1.1 Money-critical audit
Promo FOR UPDATE, referral claim-then-grant, payment history shape — tuzatildi.

</details>

## 2. Hozirgacha nima TUGADI (qisqa)

**M1** — TO‘LIQ. **M2 monetizatsiya (sandbox)** — TO‘LIQ (audit + Grand Mock + returnURL wired).

| Wave | Holat |
|------|-------|
| M4-01/02 Leaderboard | ✅ |
| M4-03…05 Arena | ✅ (U-48 RedisTransport / U-49 practice bot deferred) |
| M4-06 bot + U-09 FE link | ✅ |
| M4-08 push | partial (U-11 digest + admin broadcast stub) |
| U-04 refund revoke | ✅ (Payme path) |
| U-05 referral antifraud window | ✅ |
| U-01 demo migrate | ✅ |
| U-16 FSRS due practice UX | ✅ |
| U-21 promo pro-rate UX | ✅ |
| U-24 demo 5-question whitelist | ✅ |
| U-27 payrecon | partial dry-run skeleton |
| U-32 SEO shells | ✅ (jarimalar honest shell) |
| U-35 Grand Mock certificate | partial (share page; PDF/admin open) |
| U-38/39 PWA | foundation + meta/CMS + recently-opened variant detail; exam sync large |
| U-45 M3 | M3-0…M3-7 + audit + broadcast + flags + limits + inbox + home CMS + ops feed/alerts + investors stub |
| U-40 B2B | admin grant + teacher read portal |

> Merchant sandbox kalitlari qo‘yilsa to‘lov live; **prod** keys = U-03 tashqi blocker.

## 3. Qoldiq — past-xavfli / hujjatlashtirilgan
- Referal antifraud **attach window** ship bo‘ldi (U-05); yanada kuchli signal/fraud_flags — kelajak.
- `referral_attribution` (0003) — **dropped** (U-23 / mig `0028`); live jadval `referral` (0015).
- M4-01 rebuild cap approximation (U-22) — joriy VIP/cap; tarixiy fidelity yo‘q.
- Promo pro-rating: entitlement.note + FE notice (U-21); avtomatik refund yo‘q.
- 15 biletsiz leftover savollar (U-29) — practice/FSRS OK; UX copy tickets/practice da tushuntirilgan.
- Checkout `returnURL` — **tuzatildi** (endi `checkoutPendingReturnURL`); eski §3 “hardcoded empty” yozuvi eskirgan.

## 3.1 MUHIM TUZATISH — M4-01 ≠ streak
Streak/gamification M1’da. M4-01 = Leaderboard. Shubhali Plan nomida roadmap + inventory’ni tekshiring.

## 4. KEYINGI ANIQ QADAM (tavsiya) — kodga mos

| Plan | Nomi | Holat |
|------|------|-------|
| M4-01 | Leaderboard BE | **TUGADI** |
| M4-02 | Leaderboard UI | **TUGADI** |
| M4-03 | Arena realtime infra | **TUGADI** |
| M4-04 | Arena rating/medals/history | **TUGADI** |
| M4-05 | Arena UI | **TUGADI** |
| M4-06 | Telegram bot poydevor | **TUGADI** (+ FE link U-09) |
| M4-07 | TG quiz + notif | deferred (U-10) |
| M4-08 | Web push | **partial** (U-11 + broadcast stub) |

**Tavsiya (code-completable, tashqi blocker’siz):** legal CMS stub / content signs admin stub / U-35 admin credential polish. Skip: U-03, U-02 host, U-12, inventing U-10, B2B school sales, full offline exam.

To‘liq U-xx jadval: inventory §2.

## 5. Operatsion faktlar (MUHIM — vaqt tejaydi)
- **Go PATH:** `export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"`.
- **sqlc:** `make generate`.
- **DB testlar:** `make test` yoki `make test-parallel`. Migratsiya joyida o‘zgarsa `make test-db-reset`.
- **Redis testlar:** yangi paket → `internal/redisx/testhelper.go` `testDBByPackage`.
- **`pool.Exec`** parametrli multi-statement Qo‘llamaydi.
- **Dev API restart:** aniq PID (`ss -ltnp|grep :8090`), `pkill -f cmd/api` ISHLATMANG.
- **Infra:** `docker compose` (postgres:5432, redis:6379, minio:9000).
- **Payme/Click:** ENV bo‘sh → webhook rad.
- **ENV:** `backend/.env.example`. `ENV=staging|prod` → `JWT_SECRET`, `CLIENT_IP_ASSERTION_SECRET` (≥32), non-sandbox `OTP_CHANNEL` majburiy.
- **Health:** `/healthz` liveness, `/readyz` Postgres+Redis, `/metrics` Prometheus text (JSON via Accept) (U-41).
- **Money-critical naqsh:** `pool.Begin` + `SELECT...FOR UPDATE` / claim-`RETURNING` + tx-bound Service.

## 6. Ish uslubi
Har Plan: brainstorming → writing-plans → TDD → whole-branch review → **commit+push** green stage. Money/ops o‘zgarishlarida devops audit (`docs/superpowers/specs/*-devops-audit.md`). Build/test yashil ≠ “tugadi” — holistik review.

## 7. Keyingi Plan'lar
M2 tugadi. Growth asosiy yo‘llar ship. Qoldiq: M4-07, M4-08 polish, M6 offline exam depth, M7 ops, **M3 Admin last remnants**. Roadmap: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md` (M4 qatorlari U-50 da yangilandi).
