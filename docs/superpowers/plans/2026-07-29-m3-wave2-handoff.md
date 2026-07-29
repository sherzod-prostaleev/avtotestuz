# M3 Admin — Wave 2 handoff (2026-07-29)

Bu fayl yangi sessiya uchun to'liq brief. Ichidagi hamma fakt 2026-07-29 da
kodning o'zidan tekshirilgan (grep/curl/ssh bilan), taxmin emas.

---

## 0. Loyiha va eng muhim chegaralar

**Driver Go** (`drivergo.uz`) — o'zbek haydovchilik nazariy imtihoniga
tayyorlaydigan pullik platforma. Go backend + Next.js 14 frontend, monorepo:
`/home/sher/Рабочий стол/avtotest`.

⚠️ **PLATFORMA JONLI ISHLAYAPTI. HAQIQIY PUL TO'LAGAN FOYDALANUVCHILAR BOR**
(Payme, Click, qo'lda o'tkazma, referral pul to'lovlari haqiqiy kartalarga).

**Buziladigan chegaralar (bularni buzish = jiddiy zarar):**

1. **Serverdagi bazaga zarar yetkazmaslik.** Premium sotib olganlar bor.
   Migratsiya yozsang — faqat qo'shimcha (additive), hech qachon destruktiv emas.
2. **GRAND MOCK (imtihon simulyatsiyasi) dizayni buzilmasligi kerak** —
   uni foydalanuvchining o'zi qilgan. Amalda bu degani:
   `frontend/src/components/shared/answer-option.tsx` va u ishlatadigan
   **umumiy** CSS tokenlar (`--success`, `--danger`, `--accent`, `--ring`)
   admin ishi ichida O'ZGARTIRILMAYDI. Admin uchun rang kerak bo'lsa —
   yangi `*-ink` tokeni yarat. Wave 1 da aynan shu xato bir marta qilingan
   va orqaga qaytarilgan (commit `2905d07` ni o'qi).
3. **Chrome extension taklif qilinmaydi** — foydalanuvchi rad etgan.
4. **sudo-bypass rad etilgan chegara.**
5. **`main` ga merge va prod deploy — faqat foydalanuvchi aniq ruxsat berganda.**

---

## 1. NIMA TUGAGAN (tekshirilgan)

### Wave 1 — admin UX/UI poydevori: TUGADI, deploy qilingan

PR #12 → merge commit `1c1fc19` → `main`. Prod'ga 2026-07-29 da deploy
qilingan va jonli saytda tekshirilgan. CI 4/4 yashil.

Qamrov:
- **`AdminDataTable`** (`frontend/src/components/admin/admin-data-table.tsx`) —
  bitta primitiv: desktopda TanStack Virtual bilan virtualizatsiya qilingan
  jadval, `md` dan pastda **o'sha `columns`** dan avtomatik yasalgan karta
  ro'yxati. Yangi direktoriya yozganda ikkinchi markup daraxti yozilmaydi.
  `meta` orqali boshqariladi: `hideOnCard`, `cardTitle`, `numeric`.
- **Virtualizatsiya xatosi tuzatilgan.** Qatorlar `position: absolute` edi.
  Absolyut element *blockify* bo'ladi → `display: table-row` `block` ga
  aylanadi → qator jadvalning ustun-kenglik algoritmidan chiqadi → sarlavha
  o'zi nomlagan ustun ustida turmaydi. Endi qatorlar normal oqimda, bo'sh
  joyni ikkita spacer `<tr>` ushlaydi. **Bu yechimni buzma.**
- Shell: yig'iladigan sidebar, breadcrumb, telefon pastki paneli, drawer.
  Sidebar ham, pastki panel ham ruxsatlar bo'yicha filtrlanadi.
  Yopiq drawer `invisible` (faqat transform bilan yashirish 40 ga yaqin
  havolani tab tartibida qoldirar edi).
- 44px teginish nishonlari, WCAG kontrast tuzatishlari, `*-ink` tokenlar.
- ⌘K command palette mavjud va shell'ga ulangan
  (`(shell)/layout.tsx:127`).
- **Playwright gate:** `frontend/e2e/admin-shell-responsive.spec.ts` —
  stack talab qilmaydi (admin API tarmoq chegarasida stub qilinadi),
  15 marshrut 390px da + 9 ustun tekislash 1280px da, CI'da ishlaydi.
  Fixture'lar: `frontend/e2e/fixtures/admin-rows.ts` (ataylab "yomon"
  qatorlar: uzun o'zbek ismlari, 40 belgili ID, 7×30 RBAC matritsasi).

### Backend'da ALLAQACHON BOR bo'lgan admin endpointlar

`backend/internal/admin/handlers.go`, hammasi `/admin/v1` ostida,
`RequirePermission(...)` bilan himoyalangan:

```
users:      GET /users, GET /users/{id}, POST {id}/block, POST {id}/unblock,
            POST {id}/grant, GET/POST {id}/sessions(+revoke, revoke-all),
            GET {id}/referral, GET {id}/referral/ledger,
            PUT {id}/referral/balance, PATCH {id}/referral/rate
content:    GET /content/questions, GET/PATCH /content/questions/{id},
            GET {id}/revisions, POST {id}/explanation/verify,
            GET /content/explanations, POST /content/explanations/bulk-verify,
            GET /content/signs, GET /content/tickets
payments:   GET /payments/transactions, GET/DELETE {id},
            GET /payments/recon, GET/PATCH /payments/providers[/{provider}],
            manual: cards CRUD, queue confirm/reject, events/{id}/ignore,
            telegram GET/PUT/test
referral:   GET /referral/payouts, POST {id}/paid, POST {id}/reject
cms:        GET/PUT /cms/home, /cms/legal, /cms/contacts
monitoring: GET /monitoring/health, /metrics, /jobs, /alerts, /feed, /stream
security:   GET /security/audit, GET /security/rbac,
            POST /security/totp/{enroll,confirm,disable}
settings:   GET /settings/flags, PATCH /settings/flags/{key},
            GET /settings/limits, PATCH /settings/limits/{key}
support:    GET /support/tickets, GET/PATCH {id}, GET/PUT /support/banner,
            POST /support/broadcasts/webpush
b2b:        GET/POST /b2b/orgs, GET {id}, {id}/stats, {id}/export.csv,
            POST {id}/invites, {id}/licenses, {id}/members,
            PATCH/DELETE {id}/members/{profileID}, POST .../grant
analytics:  GET /analytics/overview
investors:  GET /investors/overview
```

---

## 2. HOZIRGI ANIQ HOLAT — sahifa ↔ backend mosligi

Uch toifa bor. **Buni yodda tut: "sahifa bor" ≠ "ishlaydi".**

### A. To'liq ishlaydigan (UI + backend bor)

`/users`, `/users/[id]`, `/content/questions[/[id]]`, `/content/explanations`,
`/content/signs`, `/content/tickets`, `/payments/transactions[/[id]]`,
`/payments/manual`, `/payments/providers`, `/payments/recon`,
`/payments/referral-payouts`, `/support/inbox[/[id]]`, `/support/broadcasts`,
`/security/audit`, `/security/rbac` (faqat o'qish), `/security/totp`,
`/settings/flags`, `/settings/limits`, `/monitoring/health`,
`/monitoring/perf` (→ `/monitoring/metrics`), `/monitoring/logs`
(→ `/monitoring/feed`), `/monitoring/jobs`, `/monitoring/alerts`,
`/analytics/overview`, `/investors`, `/b2b/orgs[/[id]]`,
`/cms/home`, `/cms/legal`, `/cms/chrome` (→ `/cms/contacts`).

### B. Nav'da `stub: true` — sahifa bor, backend YO'Q

```
/analytics/exports     /analytics/funnels
/cms/brand             /cms/surfaces
/payments/catalog      /payments/webhooks
/security/ip           /settings/config
```

### C. Alohida holat — `/payments/refunds`

Bu **stub emas**. Sahifa ataylab tushuntiruvchi matn: refund pul yo'li
Payme'ning kiruvchi `CancelTransaction` chaqiruvi orqali ketadi (U-04),
admin'dan initiate qilinmaydi. Agar Wave 2 da admin-initiated refund
qo'shilsa, bu sahifa qayta yoziladi — lekin **avval backend kerak**.

---

## 3. WAVE 2 — QOLGAN ISHLAR

Spec: `docs/superpowers/specs/2026-07-26-m3-super-admin-control-center.md`
(§4 modul spetsifikatsiyalari, §9 fazalar, §10 acceptance, §11 anti-pattern).
Dizayn tizimi: `docs/superpowers/specs/2026-07-26-m3-0-admin-design-system.md`.

Har bir punkt uchun **backend ham, frontend ham** kerakligi ko'rsatilgan.

### M3-3 — To'lovlar (eng yuqori xavf, eng ehtiyot bo'lib qilinadigan)

| Ish | Backend | Frontend |
|---|---|---|
| Admin-initiated refund + entitlement revoke | YANGI: `POST /payments/transactions/{id}/refund`, refund'da VIP/entitlement'ni qaytarib olish (spec §10: "refund revokes VIP") | `/payments/refunds` qayta yoziladi |
| Webhook inbox + replay | YANGI: `GET /payments/webhooks`, `POST /payments/webhooks/{id}/replay` | `/payments/webhooks` (hozir stub) |
| Tarif/promo katalogi (yozish) | YANGI: catalog CRUD | `/payments/catalog` (hozir stub) |

⚠️ Refund va clawback mantig'i oldin bir necha marta xavfsizlik xatosi
bergan (tasklar #42, #43 — "clawback bypass = cash extraction loop",
"Payme cancel commits outside tx"). Bu joyga tegishdan oldin
`backend/internal/billing` dagi mavjud clawback kodini to'liq o'qi va
tranzaksiya chegaralarini buzma.

### M3-4 — CMS

| Ish | Backend | Frontend |
|---|---|---|
| Brand sozlamalari | YANGI | `/cms/brand` (stub) |
| Dinamik sahifa yuzalari (surfaces) | YANGI | `/cms/surfaces` (stub) |

### M3-5 — Monitoring (chuqurlashtirish)

| Ish | Backend | Frontend |
|---|---|---|
| Job pause/resume | YANGI: POST `/monitoring/jobs/{id}/{pause,resume}` | `/monitoring/jobs` (hozir faqat o'qish) |
| Alert ack/snooze | YANGI: POST `/monitoring/alerts/{id}/{ack,snooze}` | `/monitoring/alerts` (faqat o'qish) |
| Jonli log tail | `/monitoring/stream` (SSE) BOR — ulanmagan | `/monitoring/logs` real-time ga o'tkaziladi |

### M3-6 — Analitika va investorlar

| Ish | Backend | Frontend |
|---|---|---|
| Funnels | YANGI | `/analytics/funnels` (stub) |
| Async eksport jobs | YANGI: job yaratish + holat + yuklab olish | `/analytics/exports` (stub) |
| Investor entities/documents | Hozir faqat `GET /investors/overview` | `/investors` kengaytiriladi |

### M3-7 — Xavfsizlik va sozlamalar

| Ish | Backend | Frontend |
|---|---|---|
| RBAC yozish (rol/ruxsat tahrirlash) | Hozir faqat `GET /security/rbac` | `/security/rbac` yozish rejimi |
| IP allowlist | YANGI | `/security/ip` (stub) |
| Runtime config + maintenance mode + cache purge | YANGI | `/settings/config` (stub) |

### M3-2 qoldig'i — Kontent studiyasi

| Ish | Backend | Frontend |
|---|---|---|
| Signs CRUD | Hozir faqat `GET /content/signs` | `/content/signs` yozish |
| Tickets CRUD | Hozir faqat `GET /content/tickets` | `/content/tickets` yozish |
| Media kutubxona | YANGI (MinIO bor) | YANGI sahifa |
| Import/export + taksonomiya | YANGI | YANGI |
| Learning & sessions brauzeri | YANGI | YANGI |

### Boshqa ochiq ish

- **Test suite parallel ishga tushganda xavfsiz emas** (umumiy port 3000 +
  `.next/`). Ikkita Playwright chaqiruvi bir vaqtda **soxta qizil** beradi.
  Wave 1 to'qnashuv oynasini uch barobar kengaytirdi. Tuzatish: har bir
  ishga tushirish uchun alohida port + alohida `.next` papka.
- **`admin-responsive.spec.ts` (haqiqiy login, haqiqiy backend) hech qachon
  ishlamagan.** Ishga tushirish uchun: Go toolchain, `make up`, backend
  :8090 da, `make seed-admin`, keyin `npm run test:e2e:admin`.

---

## 4. QULFLANGAN QARORLAR — bularni "tuzatma"

- **Referral payout kartasi to'liq ko'rsatiladi** (`card_masked` ishlatilmaydi)
  va **foydalanuvchi telefoni maskalanmaydi**. Bu foydalanuvchining aniq
  mahsulot qarori: operator o'sha kartaga haqiqiy pul o'tkazadi, maskalangan
  raqamga o'tkazib bo'lmaydi. Kodda izoh bilan yozilgan (commit `4b0a120`).
  Yagona nazorat — `referral.read` ruxsati; **o'qishlar audit qilinmaydi**
  (audit faqat mutatsiyalarda yoziladi).
- **`--success`, `--danger`, `--accent`, `--ring`** — umumiy tokenlar,
  admin ishi ichida tegilmaydi (yuqoridagi GRAND MOCK chegarasi).
- **`.back-link`** umumiy klass, 12 ta mijoz sahifasida ishlatiladi —
  uning rangini o'zgartirma; admin havolalari `text-accent-ink` ni o'zi qo'shadi.

---

## 5. ISH USULI — majburiy

### Har bir o'zgarishdan keyin (istisnosiz)

```bash
cd frontend
npx tsc --noEmit          # CLEAN bo'lishi shart
npx next lint             # 0 warning
npx vitest run            # hozir 458/458
npm run build             # o'tishi shart
npx playwright test e2e/admin-shell-responsive.spec.ts   # 25/25
```

Backend tegilsa: `cd backend && go build ./... && go vet ./... && go test ./...`

### Test yozish qoidasi

**Har bir regression testni avval SINDIRIB ko'r.** Ya'ni tuzatishdan oldingi
kodga qarshi ishga tushirib, qizil bo'lishiga ishonch hosil qil. Wave 1 da
ikki marta "o'tayotgan" test aslida hech narsani tekshirmayotgani aniqlangan:
- Tailwind arbitrary klass (`w-[900px]`) inject qilingan, lekin dev JIT uni
  generatsiya qilmagan → test bo'shliqni o'lchagan.
- `waitFor` limitini 1ms ga tushirish testni sindirmagan, chunki `waitFor`
  callback'ni birinchi marta darhol chaqiradi va shart allaqachon rost edi.

**jsdom cheklovlari:** layout yo'q (kenglik/overflow o'lchab bo'lmaydi),
Tailwind CSS qo'llanilmaydi (`:has()`, `sr-only` kabi CSS yechimlar
ko'rinmaydi), blockification amalga oshirilmagan. Layout bilan bog'liq
narsa Playwright'da tekshiriladi, unit testda emas.

### Commit uslubi

Loyiha uslubi: `type(scope): imperativ sarlavha`, keyin bo'sh qator, keyin
**nima uchun** qilinganini tushuntiruvchi tan (nima qilinganini emas —
buni diff ko'rsatadi). Har bir commit oxirida:

```
Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
```

### Merge va deploy

- `main` ga to'g'ridan-to'g'ri push qilma — branch + PR (`gh pr create`).
- **Merge va deploy faqat foydalanuvchi aniq aytganda.**
- Deploy'da CI'ning avtomatik qadami YO'Q. Haqiqiy protsedura:

```bash
git checkout main            # sync git'dan emas, ISH DARAXTIDAN rsync qiladi
./deploy/sync-to-vps.sh
ssh root@89.117.59.137 'cd /opt/drivergo/deploy && \
  docker compose -f docker-compose.prod.yml --env-file app.env build web && \
  docker compose -f docker-compose.prod.yml --env-file app.env up -d --no-deps web'
./deploy/smoke.sh https://drivergo.uz https://drivergo.uz
```

⚠️ **`deploy/README.md` dagi buyruq serverda ISHLAMAYDI** (`docker-compose.app.yml`
bilan) — `unable to prepare context: path "/opt/backend" not found` beradi.
Serverdagi haqiqiy fayl `deploy/docker-compose.prod.yml`, working dir
`/opt/drivergo/deploy`.

⚠️ **SSH Claude Code sandbox'ida bloklanadi** (TCP 22 ochiq, lekin banner
almashinuvi tugamaydi) — `dangerouslyDisableSandbox: true` kerak.

⚠️ Deploy oldidan rollback nuqtasi:
`docker tag <hozirgi-image-id> drivergo-web:rollback-<sana>`.
`--no-deps` shart, aks holda `api` ham qayta quriladi.

---

## 6. Birinchi qadam

1. `docs/superpowers/specs/2026-07-26-m3-super-admin-control-center.md`
   ni to'liq o'qi (461 qator).
2. Yuqoridagi §2 inventarizatsiyani kodga qarshi qayta tekshir — bu fayl
   2026-07-29 dagi holat, kod o'zgargan bo'lishi mumkin.
3. Foydalanuvchidan Wave 2 ning qaysi bo'limidan boshlashni so'ra. Tavsiya
   tartibi: **M3-5 monitoring (eng past xavf, backend'i qisman bor)** →
   **M3-7 xavfsizlik/sozlamalar** → **M3-2 kontent CRUD** →
   **M3-4 CMS** → **M3-6 analitika** → **M3-3 to'lovlar (eng oxirida,
   eng yuqori xavf)**.
4. Bitta bo'limni oxirigacha tugat (backend + frontend + test + i18n uchta
   tilda: `uz-Latn`, `uz-Cyrl`, `ru`), keyin keyingisiga o't. Yarim
   tugallangan bo'limlarni qoldirma.
