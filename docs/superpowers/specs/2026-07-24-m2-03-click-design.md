# M2-03 — Click (Shop API) to'lov integratsiyasi (Dizayn/Spec)

Sana: 2026-07-24 · Milestone: M2 · Plan: M2-03 · Qatlam: backend (sandbox rejim)

## Maqsad
Click Merchant Shop API'ni (Prepare/Complete) va checkout-initiate oqimini qurish, toki foydalanuvchi tarif tanlab, Click orqali to'lasa, VIP entitlement olsin. Payme (M2-02) bilan bir xil `payment` state-machine va `billing.Service.GrantDays` ishlatiladi — faqat provayder-protokol qatlami yangi. **Sandbox rejim** — kalitlar ENV placeholder; kod tayyor, foydalanuvchi Click kabinetdan sandbox `secret_key`/`service_id`/`merchant_id` olgach real test o'tkaziladi.

## Manba
Rasmiy `click-llc/click-integration-php` (Click LLC'ning o'z GitHub tashkilotidagi kutubxonasi, `docs.click.uz`ga havola qiladi) — imzo formulasi, xato kodlari va so'rov/javob shakli shu yerdan tasdiqlangan. Ikkinchi mustaqil implementatsiya (`Muhammadali-Akbarov/click-pkg`, Django+FastAPI) bilan o'zaro tekshirildi — ikkalasi bir xil.

**Payme'dan asosiy farqlar:**
- JSON-RPC emas — flat request/response (form-urlencoded POST, JSON fallback).
- Alohida transport-auth qatlami yo'q — `sign_string` (MD5) o'zi autentifikatsiya.
- Faqat 2 metod: Prepare (`action=0`), Complete (`action=1`) — Payme'ning 6 metodidan farqli.
- Summalar **so'm** (butun/decimal), tiyin emas.
- ID'ni **biz** minttaymiz (`merchant_prepare_id` — bizning `click_transaction.id`), Click uni Prepare javobidan oladi va Complete'da bizga qaytarib yuboradi (Payme'da aksincha — Payme o'z `payme_id`sini beradi).

## Config (ENV)
`Config`ga qo'shiladi (`getenv`, Payme naqshi bilan bir xil uslub):
- `CLICK_SERVICE_ID`, `CLICK_MERCHANT_ID` — checkout URL uchun.
- `CLICK_SECRET_KEY` — imzo tekshirish uchun maxfiy kalit (sandbox/prod ENV orqali almashtiriladi, alohida `CLICK_ENV` shart emas — Payme'dan farqli, Click bitta kalit maydonini ENV orqali sandbox/prod uchun almashtirish bilan boshqaradi, xuddi shu tarzda `getenv("CLICK_SECRET_KEY","")`).
- Dev'da hammasi bo'sh bo'lishi mumkin; bo'sh `secret_key` bilan imzo hech qachon mos kelmaydi → har doim `-1` (SIGN CHECK FAILED) — bu KUTILGAN.
- Checkout host: `https://my.click.uz/services/pay` (sobit, ENV emas).

## Sxema — yangi migratsiya `0014_click_transaction`
```sql
CREATE TABLE click_transaction (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),  -- bizniki = merchant_prepare_id
  click_trans_id text NOT NULL,                                -- Click'niki (Prepare+Complete'da bir xil)
  click_paydoc_id text,
  payment_id     uuid NOT NULL REFERENCES payment(id),
  amount_uzs     bigint NOT NULL,                               -- so'm, tiyin emas
  state          int  NOT NULL DEFAULT 0,                       -- 0=waiting, 1=confirmed, -1=rejected
  reason         text,                                          -- Click yuborgan error_note (rejected bo'lsa)
  created_at     timestamptz NOT NULL DEFAULT now(),
  confirmed_at   timestamptz,
  rejected_at    timestamptz
);
CREATE INDEX click_transaction_payment_idx ON click_transaction(payment_id);
CREATE INDEX click_transaction_click_trans_idx ON click_transaction(click_trans_id);
-- M2-02 saboqi: concurrent-double-grant himoyasini BOSHIDANOQ qo'shamiz
-- (Payme'da buni keyin alohida tuzatishga to'g'ri kelgan edi).
CREATE UNIQUE INDEX click_transaction_one_active_per_payment
  ON click_transaction(payment_id) WHERE state IN (0, 1);
```
`payment.provider_txn_id` = `click_trans_id` (Prepare'da yoziladi). `payment.status`: `created` → (Prepare muvaffaqiyatli) `pending` → (Complete muvaffaqiyatli) `paid` / (Complete xato yoki Click bekor qilsa) `canceled`.

## Endpoint 1 — Checkout initiate (auth)
Mavjud `POST /api/v1/me/checkout` endpointi (M2-02'da qurilgan) **umumlashtiriladi**: body'ga `"provider": "payme" | "click"` qo'shiladi (default `"payme"`, orqaga moslik uchun). `billing.Service.StartCheckout` provayderga qarab `BuildPaymeURL` yoki `BuildClickURL` chaqiradi; `payment.provider` ustuni ham shunga mos yoziladi (hozir kod `'payme'`ni qattiq yozgan — bu safar parametrlashtiriladi).

`BuildClickURL(serviceID, merchantID, orderID string, amountUZS int64, returnURL string) string`:
`https://my.click.uz/services/pay?service_id=<serviceID>&merchant_id=<merchantID>&amount=<amountUZS>&transaction_param=<orderID>&return_url=<returnURL>` (query-parametrlar URL-encode qilinadi; `amount` butun so'm, tiyin konvertatsiyasi YO'Q).

## Endpoint 2 — Click webhook (public, imzo bilan o'z-o'zini autentifikatsiya qiladi)
`POST /api/v1/billing/click`
- **Transport:** `application/x-www-form-urlencoded` POST kutiladi (Click'ning haqiqiy xatti-harakati); himoya sifatida agar form bo'sh bo'lsa JSON body fallback qilinadi. Faqat POST (aks holda oddiy HTTP 405 — Click bu holatni JSON-RPC kabi maxsus xato kodi bilan emas, oddiy HTTP status bilan kutadi, chunki bu forma-based protokol).
- **So'rov maydonlari (har ikkala action uchun umumiy):** `click_trans_id, service_id, click_paydoc_id, merchant_trans_id, amount, action, error, error_note, sign_time, sign_string`; `action=1` (Complete) bo'lsa qo'shimcha `merchant_prepare_id` (bizning `click_transaction.id`, string sifatida qaytariladi/qabul qilinadi).
- **Imzo tekshirish (autentifikatsiya o'rnini bosadi):**
  `MD5(click_trans_id + service_id + secret_key + merchant_trans_id + (action=="1" ? merchant_prepare_id : "") + amount + action + sign_time)`.
  Mos kelmasa → `error=-1` ("SIGN CHECK FAILED!"). Bo'sh `secret_key` → doim mos kelmaydi → doim `-1`.
- **Tekshiruv tartibi** (rasmiy kutubxonadagi tartib bilan bir xil, qisqartirilgan):
  1. Majburiy maydonlar yo'q / `action=1`da `merchant_prepare_id` yo'q → `-8`.
  2. Imzo mos emas → `-1`.
  3. `action` `{0,1}`da emas → `-3`.
  4. `merchant_trans_id` (= `payment.id`) topilmasa → `-5`.
  5. (faqat Complete) `merchant_prepare_id` bo'yicha `click_transaction` topilmasa → `-6`.
  6. `payment.status == 'paid'` (allaqachon to'langan) → `-4`.
  7. Summasi mos emas (Click `amount`ni o'nlik satr sifatida yuboradi — `float64`ga parse qilib, `payment.amount_uzs`dan farqi `0.01`dan katta bo'lsa mos emas, rasmiy kutubxonadagi tolerantlik bilan bir xil) → `-2`.
  8. Click o'zi `error<0` yuborgan (Click tomonidan bekor qilingan) YOKI bizning yozuvimiz allaqachon `rejected` → `-9`.
  9. Aks holda → `0` (muvaffaqiyat).
- **Prepare (`action=0`, tekshiruv 0 dan o'tsa):** `click_transaction` yozuvi yarat (state=0, `click_trans_id`/`click_paydoc_id`/`amount_uzs` bilan) — agar shu `click_trans_id` uchun yozuv allaqachon bo'lsa, idempotent qaytar (qayta yaratma). `payment.status='pending'`, `provider_txn_id=click_trans_id`. Javob: `{click_trans_id, merchant_trans_id, merchant_prepare_id: <click_transaction.id>, error:0, error_note:"Success"}`.
- **Complete (`action=1`, tekshiruv 0 dan o'tsa):** `click_transaction.state=1`, `confirmed_at=now`, `payment.status='paid'`, `paid_at=now`, **`billing.Service.GrantDays(profile_id, tariff.days, "purchase", "", uuid.NullUUID{})`** (VIP) — M2-02'dagi kabi **bitta DB-tranzaksiyada** (`pool.Begin` + qator qulflash), pul bilan bog'liq xatolikni oldini olish uchun BOSHIDANOQ. Idempotent: `state==1` bo'lsa qayta GrantDays chaqirmasdan bir xil natijani qaytar. Click `error<0` yuborsa (yoki bizning tekshiruv `-9` chiqarsa): `click_transaction.state=-1`, `rejected_at=now`, `reason=error_note`, `payment.status='canceled'`. Javob (muvaffaqiyat): `{click_trans_id, merchant_trans_id, merchant_confirm_id: <click_transaction.id>, error:0, error_note:"Success"}`; (bekor): `{..., error:-9, error_note:"Transaction cancelled"}`.
- **Javob doim HTTP 200**, flat JSON (`{click_trans_id, merchant_trans_id, merchant_prepare_id | merchant_confirm_id, error, error_note}`) — Payme'dagi kabi `id`/`result`/`error` o'raladigan JSON-RPC konverti YO'Q.

## Xato kodlari (rasmiy, click-llc kutubxonasidan tasdiqlangan)
| Kod | Ma'no |
|-----|-------|
| 0 | Muvaffaqiyatli |
| -1 | Imzo (sign_string) mos emas |
| -2 | Summasi noto'g'ri |
| -3 | `action` noma'lum/qo'llab-quvvatlanmaydi |
| -4 | Allaqachon to'langan |
| -5 | `merchant_trans_id` (order) topilmadi |
| -6 | Tranzaksiya (`merchant_prepare_id`) topilmadi (faqat Complete) |
| -8 | So'rovda maydon yetishmayapti / noto'g'ri |
| -9 | Tranzaksiya bekor qilingan |

`error_note` — bitta til (inglizcha/rus, Click konvensiyasi bo'yicha), Payme'dagi kabi `ru/uz/en` map YO'Q (bu Click protokolining o'ziga xosligi, kamchilik emas).

## Grant chegarasi
Complete'da `GrantDays` chaqiriladi (Perform bilan bir xil VIP-berish mexanizmi). Refund/revoke oqimi M2-04'ga qoldiriladi (Payme bilan bir xil qaror).

## Kod joylari
- `internal/config/config.go` — `CLICK_SERVICE_ID/CLICK_MERCHANT_ID/CLICK_SECRET_KEY` ENV maydonlari.
- `internal/db/migrations/0014_click_transaction.up/down.sql`.
- `internal/db/queries/billing.sql` — `click_transaction` upsert/select so'rovlari + `CreatePayment`ni provider-parametrli qilish.
- `internal/billing/checkout.go` — `BuildClickURL` + `StartCheckout`ni provider-parametrli umumlashtirish.
- `internal/billing/click/` (yangi sub-paket) — `handler.go` (transport+imzo+dispatch), `errors.go` (xato kod jadvali), `methods.go` (prepare/complete).
- `internal/billing/handlers.go`/`internal/server/server.go` — `POST /billing/click` public marshrut ulanadi (Payme webhook bilan bir xil joyga, `deps.Pool` bilan — Complete'ning tranzaksiyasi uchun kerak).

## Testlar
- Imzo formulasi (MD5, aniq maydon tartibi) — birlik test.
- `BuildClickURL` — URL query-parametrlar to'g'riligi.
- Prepare: happy-path, xato holatlari (`-1/-2/-3/-5/-8`), idempotentlik (bir xil `click_trans_id` qayta).
- Complete: happy-path (`GrantDays` chaqirilishi — entitlement aktiv bo'lishi DB orqali tasdiqlanadi), idempotentlik, `-4/-6/-9`, Click `error<0` bilan bekor qilish.
- Concurrent-himoya: partial unique index'ning DB darajasida ishlashi (M2-02 Task 5/final-fix'dagi kabi to'g'ridan-to'g'ri test).
- Route: form-urlencoded va JSON fallback ikkalasi ham ishlashi.

## Scope tashqarisi
- Refund/revoke UI (M2-04), promo Click checkout'da (M2-05), checkout FE (M2-09), Click Card API (`/card/*` — token bilan to'lov, alohida oqim, kerak bo'lsa keyin), SetFiscalData/invoice-create (SMS orqali to'lov, ishlatilmaydi — bizda faqat checkout-URL orqali redirect oqimi).
