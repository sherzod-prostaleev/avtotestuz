# M2-02 — Payme (Paycom) to'lov integratsiyasi (Dizayn/Spec)

Sana: 2026-07-24 · Milestone: M2 · Plan: M2-02 · Qatlam: backend (sandbox rejim)

## Maqsad
Payme Merchant API (JSON-RPC 2.0) webhook'ini va checkout-initiate oqimini qurish, toki foydalanuvchi tarif tanlab, Payme orqali to'lasa, VIP entitlement olsin. **Sandbox rejim** — kalitlar ENV placeholder; kod tayyor, live sandbox tester'ni (test.paycom.uz) foydalanuvchi kalit olgach o'tkazadi.

## Manba (rasmiy — developer.help.paycom.uz)
JSON-RPC 2.0, bitta POST endpoint, **doim HTTP 200**. Basic-auth: login `Paycom`, parol = cashbox KEY (test/prod alohida). Summalar **tiyin**da. State: 1=pending, 2=paid, -1=pending'dan bekor, -2=paid'dan bekor. Vaqtlar ms.

## Config (ENV)
`Config` ga qo'shiladi (`getenv`):
- `PAYME_MERCHANT_ID` (cashbox id) — checkout URL uchun.
- `PAYME_KEY` (prod key), `PAYME_TEST_KEY` (sandbox key) — Basic-auth paroli.
- `PAYME_ENV` = `test` (default) yoki `prod` — qaysi kalit va checkout hostini tanlaydi.
- Checkout host: test → `https://checkout.paycom.uz` (sandbox flow test.paycom.uz tester orqali webhook'ni haydaydi; checkout URL hosti bir xil).
- Dev'da hammasi bo'sh bo'lishi mumkin; webhook auth bo'sh-kalitni **rad etadi** (-32504).

## Sxema — yangi migratsiya `0012_payme_transaction`
`payment` (M2 sxemada bor) = bizning buyurtma. Payme tranzaksiyasi alohida (bir buyurtmaga ketma-ket urinishlar bo'lishi mumkin):
```sql
CREATE TABLE payme_transaction (
  payme_id     text PRIMARY KEY,                              -- Payme params.id
  payment_id   uuid NOT NULL REFERENCES payment(id),
  amount_tiyin bigint NOT NULL,
  state        int  NOT NULL,                                 -- 1 / 2 / -1 / -2
  reason       int,                                           -- cancel reason
  create_time  bigint NOT NULL,                               -- ms
  perform_time bigint NOT NULL DEFAULT 0,
  cancel_time  bigint NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payme_transaction_payment_idx ON payme_transaction(payment_id);
CREATE INDEX payme_transaction_time_idx ON payme_transaction(create_time);
```
`payment.provider_txn_id` = payme_id (ilk create'da). `payment.status`: created→pending(1)→paid(2) / canceled(-1/-2).

## Endpoint 1 — Checkout initiate (auth)
`POST /api/v1/me/checkout` (JWT talab qilinadi)
- Body: `{ "tariff_code": "gentra" }` (promo M2-05'da qo'shiladi).
- Amal: tarifni ol (active), `payment` yarat (profile_id=JWT, tariff_id, amount_uzs=tariff.price_uzs, provider='payme', status='created', idempotency_key=yangi uuid, meta={tariff_code, days}).
- Javob: `{ "payment_id": "<uuid>", "checkout_url": "https://checkout.paycom.uz/<base64>" }`.
- Checkout URL: `base64("m=<PAYME_MERCHANT_ID>;ac.order_id=<payment_id>;a=<amount_uzs*100>;l=<locale>;c=<return_url>")`.

## Endpoint 2 — Payme webhook (public, Basic-auth)
`POST /api/v1/billing/payme`
- **Auth:** `Authorization: Basic base64("Paycom:<KEY>")`; KEY = PAYME_TEST_KEY yoki PAYME_KEY (PAYME_ENV bo'yicha). Noto'g'ri/bo'sh → `-32504`. Bo'sh-config'da har doim rad.
- **Transport:** faqat POST (aks holda `-32300`), JSON parse xato → `-32700`, noma'lum metod → `-32601`. Har javob HTTP 200.
- **Dispatcher** (`params.method`):

| Metod | Amal |
|-------|------|
| `CheckPerformTransaction` | account.order_id bo'yicha payment top; mavjud + status='created'(payable) + amount(tiyin)==amount_uzs*100 → `{allow:true}`. Payment yo'q/noto'g'ri account → `-31050` (data:"order_id"). Summa mos emas → `-31001`. |
| `CreateTransaction` | Idempotent: payme_id mavjud bo'lsa → mavjud (create_time/transaction/state). Yangi: account+amount qayta tekshir; agar shu payment'da boshqa faol (state 1/2) txn bo'lsa → `-31008`; aks holda payme_transaction(state=1, create_time=now-ms) yarat, payment.status='pending', provider_txn_id=payme_id. Natija: `{create_time, transaction: payment_id, state:1}`. |
| `PerformTransaction` | payme_id bo'yicha txn top (yo'q → `-31003`). state==2 → idempotent qaytar. state!=1 → `-31008`. **12-soat timeout:** create_time+43_200_000ms < now → txn'ni -1(reason=4) qil, payment.status='canceled', keyin `-31008`. Aks holda: state=2, perform_time=now, payment.status='paid', paid_at=now, **`billing.Service.GrantDays(profile, tariff.days, "purchase", by=null)`** (VIP). Natija: `{transaction: payment_id, perform_time, state:2}`. |
| `CancelTransaction` | txn top (`-31003`). state==-1/-2 → idempotent. state==1 → -1; state==2 → -2 (payment.status='refunded'/'canceled'). cancel_time=now, reason=params.reason. **Eslatma:** performed(-2) bekorda entitlement qaytarish (revoke) — M2-04/refund doirasida; hozircha payment.status='refunded' belgilanadi, revoke keyin. |
| `CheckTransaction` | txn top (`-31003`). `{create_time, perform_time, cancel_time, transaction: payment_id, state, reason}` (0 = yo'q). |
| `GetStatement` | `from..to` (ms) oralig'idagi txn'lar ascending: `[{id: payme_id, time, amount, account:{order_id}, create_time, perform_time, cancel_time, transaction: payment_id, state, reason}]`. |

## Xato javob shakli
`{"error":{"code":-31050,"message":{"ru":"...","uz":"...","en":"..."},"data":"order_id"},"id":<req id>}` — account xatolarida `message`(3 til) + `data`(maydon) majburiy.

## Grant chegarasi
Perform'da `GrantDays` chaqiriladi → M2-02 to'liq revenue halqasi. **M2-04** endi = to'lov TARIXI endpoint + cheklar (read-side) + refund/revoke.

## Kod joylari
- `internal/config/config.go` — Payme ENV maydonlari + `PaymeKey()` helper (env bo'yicha to'g'ri kalit).
- `internal/db/migrations/0012_payme_transaction.up/down.sql`.
- `internal/db/queries/billing.sql` — payment + payme_transaction upsert/select so'rovlar.
- `internal/billing/checkout.go` — checkout-initiate logikasi + Payme URL builder (base64).
- `internal/billing/payme/` (yangi sub-paket) — JSON-RPC tiplari, dispatcher, 6 metod, auth, xatolar.
- `internal/billing/handlers.go` — `POST /me/checkout` (auth) + `POST /billing/payme` (public) route'lari.
- `internal/server/server.go` — checkout route auth bilan, payme webhook auth'siz (o'zi Basic-auth qiladi).

## Testlar
- URL builder (base64, tiyin konvertatsiya).
- Har JSON-RPC metod: happy-path + xato (account yo'q, summa mos emas, txn yo'q, holat mos emas), idempotentlik (Create/Perform/Cancel takror), 12-soat timeout, auth (noto'g'ri/bo'sh kalit → -32504), non-POST → -32300.
- Perform → GrantDays chaqirilishi (entitlement aktiv bo'ladi) — DB integration.
- GetStatement oralig'i.

## Scope tashqarisi
- Promo checkout'da (M2-05), to'lov tarixi UI + refund/revoke (M2-04), Click (M2-03), checkout FE (M2-09).
- SetFiscalData (ixtiyoriy — kerak bo'lsa keyin). ChangePassword (Payme so'ramaguncha yo'q).
