# SESSION HANDOFF — bu yerdan boshlang (2026-07-24)

> Yangi sessiya uchun: bu hujjat **aniq holat + keyingi aniq qadam**ni beradi. Avval buni o'qing, keyin ishlang.

## 0. Maqsad (kontekst)
AvtoTest — O'zbekiston YHQ imtihoniga tayyorlovchi **pullik onlayn maktab-startap** (onless.uz/osonprava.uz analogi, "10-15x kuchli"). Go backend + Next.js frontend. Manba-hujjat: repo ildizida `AVTOTEST-MASTER-PROMPT.txt`.

**Hozir qayerdamiz:** M1 (backend + kontent + frontend asosiy) TUGADI. Endi **M2 — Monetizatsiya** ustida ishlayapmiz. Ichida **M2-02 (Payme to'lov)** ni quryapmiz.

To'liq roadmap: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md` (M2→M7, admin oxirida; bajarilish tartibi + vaqt baholari 9-bo'limda).

## 1. Audit qilingan holat (2026-07-24, tekshirilgan)
- Git: `main`, origin bilan sinxron (push qilingandan keyin). Ish daraxti **toza**.
- Backend: `go build ./...` OK; `go test ./internal/billing/... -p 1` **o'tadi**.
- DB migratsiya: **version 12**, dirty emas.
- Dev API: `/tmp/avtotest-api-new` binardan 8090-portда ishlaydi (health 200).
- Kontent: 1231 savol (3 til), 62 bilet, 285 belgi (3 til, rasm). Foydalanuvchi ma'lumoti pre-launch tozalangan (0 profile).

## 2. Bu sessiyada bajarilgan ish (xulosa)
1. **Belgilar bazasi** to'liq: 285 belgi, 3 til (uz-Cyrl transliterator), haqiqiy rasmlar, importer MIME tuzatildi.
2. **Savol kontenti QA**: 1231 savol audit + dedup; 62 bilet (61×20 + 11); 5 matn tuzatildi; pre-launch user-wipe.
3. **Repo tozalandi** (~1.8GB o'lik Flutter worktree'lar).
4. **Roadmap** (M2→M7) to'liq dekompozitsiya + vaqt baholari.
5. **M2-01 (tarif modeli) TUGADI va LIVE**: `GET /api/v1/tariffs` — Nexia(7k)/Gentra(30k)/Malibu(75k), 3 til, hisoblangan per-day/discount. Spec+plan+4 TDD commit.
6. **M2-02 (Payme) — Task 1-2 TUGADI** (pastga qarang).

## 3. M2-02 Payme — ANIQ HOLAT va KEYINGI QADAM

**Spec:** `docs/superpowers/specs/2026-07-24-m2-02-payme-design.md` (Payme protokoli rasmiy `developer.help.paycom.uz` dan; account=order_id=payment.id; summa tiyin=so'm×100; Perform'da GrantDays VIP beradi; sandbox rejim, ENV placeholder kalitlar).

**Reja (7 TDD task):** `docs/superpowers/plans/2026-07-24-m2-02-payme.md`

| Task | Holat |
|------|-------|
| 1. Config + `payme_transaction` sxema + sqlc query | ✅ TUGADI (migratsiya 12) |
| 2. Checkout URL builder + `StartCheckout` | ✅ TUGADI (testlar o'tdi) |
| **3. JSON-RPC skelet (auth -32504, non-POST -32300, dispatcher, xato tiplari)** | ⏭️ **KEYINGI** |
| 4. CheckPerformTransaction + CreateTransaction | ⬜ |
| 5. PerformTransaction (**VIP beradi**) + CancelTransaction | ⬜ |
| 6. CheckTransaction + GetStatement | ⬜ |
| 7. Route ulash (`POST /me/checkout` auth, `POST /billing/payme` public) + to'liq oqim integration testi | ⬜ |

**➡️ KEYINGI ANIQ QADAM:** M2-02 reja faylining **Task 3** ini bajaring — `internal/billing/payme/` sub-paketida JSON-RPC skelet (`rpc.go`, `errors.go`, `handler.go` + test). Rejadagi kod-eskizlar va test-holatlariga amal qiling. Har task TDD + alohida commit.

**Tayyor primitivlar (Task 3+ ishlatadi):**
- Config: `cfg.PaymeKey()` (env bo'yicha kalit), `cfg.PaymeMerchantID`, `cfg.PaymeCheckoutHost()`.
- sqlc query'lar (billing.sql): `GetPaymentForPayme` (payment+tariff.days), `SetPaymentStatus`, `MarkPaymentPaid`, `CreatePaymeTransaction`, `GetPaymeTransaction`, `GetActivePaymeTxByPayment`, `PerformPaymeTransaction`, `CancelPaymeTransaction`, `ListPaymeTransactionsByTime`.
- `billing.Service.GrantDays(ctx, profileID, days, source, note, by)` — Perform'da VIP berish.
- `billing.Service.StartCheckout(...)` + `BuildPaymeURL(...)` (Task 2).
- Auth kontekst: `auth.FromContext(ctx)` → `claims.ProfileID` (checkout handler uchun, T7).

## 4. Operatsion faktlar (MUHIM — vaqt tejaydi)
- **Go PATH:** har `go`/`sqlc` buyrug'iga `export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"` prefiks (interaktiv bo'lmagan PATH'da yo'q).
- **sqlc generate:** `make generate` (repo ildizidan).
- **DB testlar:** `-p 1` (bitta test-DB `avtotest_test`); `testdb.New(t)` migratsiya qo'llaydi + `Truncate` qiladi. **`Truncate` `tariff`/`payment`/`payme_transaction`ni ham o'chiradi** → DB testlar o'z fixture'ini insert qiladi.
- **`pool.Exec` parametr bilan bir nechta SQL buyrug'ini QO'LLAMAYDI** (prepared statement) — parametrli insert'larni alohida `Exec`ga bo'ling.
- **Dev API restart:** `pkill -f "cmd/api"` KENG pattern shell'ni o'ldiradi (exit 144) — o'rniga `ss -ltnp | grep :8090` bilan aniq PID topib kill qiling; yangi binarni `run_in_background` bilan ishga tushiring (`nohup &` emas). Migratsiyalar API/importer start'da avtomatik qo'llanadi.
- **Infra:** `docker compose` (postgres:5432, redis:6379, minio:9000) ishlab turibdi; backend compose'da EMAS (`go run ./cmd/api` yoki binar).
- **Payme kalitlari:** hozircha ENV bo'sh (placeholder). Webhook auth bo'sh-kalitni rad etadi (-32504) — bu KUTILGAN. Foydalanuvchi Payme kabinetdan cashbox ochib TEST_KEY olgach, ENV'ga qo'yadi va test.paycom.uz sandbox tester'ni o'zi o'tkazadi.

## 5. Ish uslubi (skill'lar)
Har Plan: `brainstorming` (spec) → `writing-plans` (reja) → TDD implementatsiya → `requesting-code-review`. Mustaqil Plan'larni parallel subagentlarga berish mumkin (roadmap 0-bo'lim). Kelajakdagi Plan tartibi va vaqt baholari roadmap 9-bo'limda.

## 6. Keyingi bir necha Plan (M2 Wave 1, parallel)
M2-02 (Payme) tugagach: **M2-03 Click** · **M2-08 tarif UI** (FE) · **M2-05 promo** · **M2-04 to'lov tarixi** · **M2-11 demo**. Har biri roadmapda dekompozitsiya qilingan.
