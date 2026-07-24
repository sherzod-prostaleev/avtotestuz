# M2-06 — Referal Dasturi (Backend Dizayn / Spec)

Sana: 2026-07-24 · Milestone: M2 · Plan: M2-06 · Qatlam: backend

## 1. Maqsad
Foydalanuvchilarga o'z do'stlarini taklif qilish uchun unikal referal kod/link taqdim etish va taklif qilingan do'st (referee) birinchi to'lovni amalga oshirgach, taklif qilgan foydalanuvchiga (referrer) va do'stiga bonus VIP kunlarini avtomatik taqdim etish. Anti-fraud (firibgarlikka qarshi) cheklovlarini ta'minlash.

## 2. Biznes Mantiq va Anti-Fraud Qoidalari
1. **Unikal Referal Kod**: Har bir foydalanuvchi uchun unikal 8 belgili referal kod (`REF-XXXXXX`) generatsiya qilinadi.
2. **O'z-o'zini taklif qilish taqiqlanadi**: `referrer_id != referee_id`.
3. **Bir martalik biriktirish**: Har bir foydalanuvchi faqat 1 ta referrer tomonidan taklif qilinishi mumkin (`referee_id` UNIQUE).
4. **To'lovdan keyin mukofotlash**:
   - Do'sti ro'yxatdan o'tganda va referal kodini biriktirganda holat `pending` bo'ladi.
   - Do'sti birinchi marta har qanday pullik tarifni sotib olganda (`paid` statusiga o'tganda):
     - Taklif qilgan kishi (`referrer`) **7 kunlik VIP mukofot** oladi (`GrantDays(7)`).
     - Holat `rewarded`ga o'tadi va `rewarded_at` vaqti saqlanadi.
     - Bir marta mukofotlangan referal qayta mukofot bermaydi.

## 3. Ma'lumotlar Bazasi Migratsiyasi (`0004_referral.up.sql`)
```sql
CREATE TABLE IF NOT EXISTS user_referral_code (
    user_id UUID PRIMARY KEY REFERENCES "user"(id) ON DELETE CASCADE,
    code VARCHAR(32) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS referral (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    referee_id UUID NOT NULL UNIQUE REFERENCES "user"(id) ON DELETE CASCADE,
    referral_code VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending', -- 'pending', 'rewarded'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rewarded_at TIMESTAMPTZ,
    CONSTRAINT chk_no_self_referral CHECK (referrer_id <> referee_id)
);

CREATE INDEX idx_referral_referrer ON referral(referrer_id);
CREATE INDEX idx_referral_code ON user_referral_code(code);
```

## 4. API Endpointlar
- `GET /api/v1/me/referral` (auth):
  ```json
  {
    "referral_code": "REF-A8B9C0",
    "invite_url": "https://avtotest.uz/r/REF-A8B9C0",
    "total_invited": 5,
    "total_rewarded": 2,
    "bonus_days_earned": 14
  }
  ```
- `POST /api/v1/referral/apply` (auth):
  - Request: `{ "code": "REF-A8B9C0" }`
  - Response: `{ "success": true, "referrer_name": "..." }`

## 5. Webhook Integratsiyasi (`ProcessPaymentGrant`)
`ProcessPaymentGrant` funksiyasiga referal mukofotini avtomatik tekshirish va berish mantiqi qo'shiladi:
- Agar to'lov qilgan `user_id` referee bo'lsa va uning referali `pending` holatda bo'lsa:
  - Referrer'ga `GrantDays(ctx, referrer_id, 7)` beriladi.
  - Referal statusi `rewarded`ga yangilanadi.

## 6. Testlash Rejasi
- Kod generatsiyasi va biriktirish unit testlari.
- O'z-o'zini taklif qilish va takroriy biriktirish xatolik testlari.
- To'lov webhook'ida mukofot avtomatik taqdim etilishi integration testlari.
