# M2-05 — Promo-kodlar (Dizayn / Spec)

Sana: 2026-07-24 · Milestone: M2 · Plan: M2-05 · Qatlam: backend

## 1. Maqsad
Foydalanuvchilarga tarif xaridida chegirma yoki qo'shimcha kunlar beruvchi promo-kodlar tizimi.
Backend promo-kodlarni tekshirish (`POST /api/v1/billing/promo/validate`), checkout so'rovida promo-kodni qo'llash, narxni hisoblash, va to'lov muvaffaqiyatli o'tgach anti-fraud limitlarini (max_uses, per_user_limit) oshiruvchi redemption yozuvini yaratishni ta'minlaydi.

## 2. Sxema va Ma'lumotlar Manbai
Sxema migratsiya 0003'da allaqachon yaratilgan (`promo_code`, `promo_redemption`, `payment.promo_code_id`).

### `promo_code` (mavjud):
- `id`: uuid (PK)
- `code`: text NOT NULL UNIQUE
- `kind`: text ('percent', 'fixed', 'days')
- `value`: int NOT NULL (> 0)
- `max_uses`: int (NULL = cheksiz)
- `per_user_limit`: int NOT NULL DEFAULT 1
- `valid_from`: timestamptz (NULL = cheklov yo'q)
- `valid_to`: timestamptz (NULL = cheklov yo'q)
- `active`: boolean NOT NULL DEFAULT true

### Promo turlari (`kind`):
1. `percent`: Tarif narxidan `value` foiz chegirma. `discount = floor(price * value / 100)`.
2. `fixed`: Tarif narxidan `value` so'm chegirma. `discount = min(price, value)`.
3. `days`: Tarifga `value` ta bonus kun qo'shadi (agar to'lovli bo'lsa) YOKI 100% chegirma beradi (0 so'm checkout).

## 3. Validatsiya va Anti-Fraud Qoidalari
Promo-kod quyidagi barcha shartlar bajarilgandagina yaroqli deb hisoblanadi:
1. `active == true` va kod mavjud (registrsiz/case-insensitive taqqoslash: `LOWER(code)`).
2. `valid_from` bo'lsa: `now() >= valid_from`.
3. `valid_to` bo'lsa: `now() <= valid_to`.
4. `max_uses` bo'lsa: `promo_redemption` jadvalidagi ushbu promo_code_id bo'yicha jami redemption'lar soni < `max_uses`.
5. `per_user_limit`: `promo_redemption` jadvalidagi (promo_code_id, profile_id) bo'yicha redemption'lar soni < `per_user_limit`.

Xatolik holatlari:
- Kod topilmasa yoki nofaol: `404 promo_not_found`
- Muddat mos kelmasa: `400 promo_expired` / `400 promo_not_started`
- Ishlatish limiti tugagan bo'lsa: `400 promo_limit_reached`
- Foydalanuvchi limiti tugagan bo'lsa: `400 promo_user_limit_reached`

## 4. Endpoints va DTOlar

### 4.1. Promo-kodni tekshirish
`POST /api/v1/billing/promo/validate` (JWT talab qilinadi)

**Request Body:**
```json
{
  "code": "AVTO2026",
  "tariff_code": "gentra"
}
```

**Response Body (200 OK):**
```json
{
  "valid": true,
  "code": "AVTO2026",
  "kind": "percent",
  "value": 20,
  "discount_uzs": 11980,
  "original_amount_uzs": 59900,
  "final_amount_uzs": 47920,
  "bonus_days": 0
}
```

### 4.2. Checkout endpointini kengaytirish
`POST /api/v1/me/checkout` (JWT talab qilinadi)

**Request Body:**
```json
{
  "tariff_code": "gentra",
  "provider": "payme",
  "promo_code": "AVTO2026"
}
```

Mantiq:
- `promo_code` berilgan bo'lsa: validate qilinadi.
- Payments jadvalida `amount_uzs` = `final_amount_uzs` va `promo_code_id` = `promo.id` saqlanadi.
- Agar `final_amount_uzs == 0` bo'lsa (100% chegirma yoki 0 so'm promo):
  - Payment status = `paid`, `paid_at` = `now()`.
  - Entitlement darhol uzaytiriladi (`GrantDays`).
  - `promo_redemption` yozuvi yaratiladi.
  - Response: `{"payment_id": "...", "checkout_url": "", "free": true}`.

### 4.3. GrantDays mantiqi (Webhook yoki Free Checkout)
Payment `paid` statusiga o'tganda (`GrantDays` chaqirilganda):
- Agar `payment.promo_code_id` mavjud bo'lsa:
  - `promo_redemption` jadvaliga `(promo_code_id, profile_id, payment_id)` yoziladi.
  - Agar promo `kind == 'days'` bo'lsa, `GrantDays` entitlement kunlariga `tariff.days + promo.value` qo'shadi.

## 5. Testlash Rejasi
1. Percent, Fixed, Days promo-kodlari uchun narx hisoblash unit testlari.
2. Expired, not active, max_uses va per_user_limit testlari.
3. `/billing/promo/validate` integration testlari (auth, valid, invalid cases).
4. `/me/checkout` promo bilan checkout yaratish va 0 so'mlik bepul checkout testi.
5. Webhook `paid` bo'lganda `promo_redemption` yaratilishi va anti-fraud limit qayta to'lmasligi testlari.
