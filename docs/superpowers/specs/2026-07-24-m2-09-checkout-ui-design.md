# M2-09 — Checkout Oqimi va To'lov Sahifalari (Dizayn / Spec)

Sana: 2026-07-24 · Milestone: M2 · Plan: M2-09 · Qatlam: frontend

## 1. Maqsad
Foydalanuvchiga `/premium` sahifasida to'lov provayderini (`Payme` yoki `Click`) tanlash, promo-kod kiritish va chegirma prevyusini ko'rish, xaridni tasdiqlash va to'lov holati qaytish sahifalarini (`/checkout/success`, `/checkout/failure`, `/checkout/pending`) taqdim etish.

## 2. Foydalanuvchi Tajribasi va Ekranlar

### 2.1. Premium Sahifasidagi Checkout Modali / Formasi (`/premium`)
Har bir tarif kartasida:
- Provayder tanlov paneli: **Payme** (binafsha/ko'k brend badge) va **Click** (binafsha brend badge). Default: Payme.
- Promo-kod maydoni (Input + "Qo'llash" / "Apply" tugmasi):
  - Kiritilgan kodni `POST /api/v1/billing/promo/validate` orqali tekshiradi.
  - Muvaffaqiyatli bo'lsa: chegirma summasi (`-11 980 so'm`), yangi yakuniy summa (`47 920 so'm`) va bonus kunlar (bo'lsa) yashil/oltin indikatsiyada ko'rsatiladi.
  - Xatolik bo'lsa: "Promo-kod yaroqsiz" / "Promo-kod muddati o'tgan" / "Ishlatish limiti tugagan" xatosi qizil ko'rsatiladi.
- "Sotib olish" tugmasi:
  - `POST /api/v1/me/checkout` so'rovini `{ "tariff_code": "...", "provider": "payme"|"click", "promo_code": "..." }` bilan yuboradi.
  - Javobda `free: true` bo'lsa → foydalanuvchini `/checkout/success?free=true` sahifasiga yo'naltiradi va VIP statusni darhol faollashtiradi.
  - Javobda `checkout_url` bo'lsa → `window.location.href = result.checkout_url` orqali tegishli provayder checkout sahifasiga redirect qiladi.

### 2.2. Muvaffaqiyatli To'lov Sahifasi (`/checkout/success`)
- Konfetti animatsiyasi (Framer Motion).
- VIP Nishon (Oltin Crown/Shield icon) va muvaffaqiyatli xarid xabari ("Tabriklaymiz! VIP obuna faollashtirildi").
- "Mashqlarni boshlash" va "Bosh sahifaga qaytish" tugmalari.

### 2.3. Muvaffaqiyatsiz To me To'lov Sahifasi (`/checkout/failure`)
- Ogohlantirish kartasi ("To'lov amalga oshmadi yoki bekor qilindi").
- Sabab va "Qayta urinish" tugmasi (`/premium` sahifasiga qaytaradi).

### 2.4. Kutilayotgan To'lov Sahifasi (`/checkout/pending`)
- Yuklanish va holatni tekshirish animatsiyasi.
- Har 3 soniyada `GET /api/v1/me/entitlement` yoki `GET /api/v1/me/payments` orqali holatni polling qiladi.
- VIP faollashgach → avtomatik `/checkout/success` sahifasiga yo'naltiradi.

## 3. Komponentlar Strukturasi
- `frontend/src/app/[locale]/(app)/premium/page.tsx` — Yangilangan tariflar va checkout formasi.
- `frontend/src/components/checkout/provider-picker.tsx` — Payme / Click brend selektori.
- `frontend/src/components/checkout/promo-input.tsx` — Promo-kod inputi va preview visual kartasi.
- `frontend/src/app/[locale]/(app)/checkout/success/page.tsx` — Muvaffaqiyat sahifasi.
- `frontend/src/app/[locale]/(app)/checkout/failure/page.tsx` — Xatolik sahifasi.
- `frontend/src/app/[locale]/(app)/checkout/pending/page.tsx` — Polling status sahifasi.

## 4. i18n Matnlari (3 til)
`Premium`, `Checkout` va `Promo` bo'limlari uchun uz-Latn, uz-Cyrl va ru fayllariga mos matnlar qo'shiladi.

## 5. Testlash Rejasi
- Promo-kod validate va preview Vitest komponent testlari.
- Provider picker va checkout form testlari.
- Redirect va free checkout flow testlari.
