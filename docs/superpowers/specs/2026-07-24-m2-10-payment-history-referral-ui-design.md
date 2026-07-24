# M2-10: To'lov Tarixi va Referal UI Design Spec

## 1. Maqsad va Umumiy Ko'rinish
Ushbu bosqich M2-04 (To'lov tarixi read-side) hamda M2-06 (Referal dasturi backend) imkoniyatlarini foydalanuvchi interfeysiga (`/profile` va `/profile/referral`) olib chiqadi.

## 2. Asosiy Bo'limlar va UI Funksiyalari

### 2.1 Referal Dasturi Bo'limi (`ReferralSection`)
- **Referal Havola & Kod**: Foydalanuvchining shaxsiy referal kodi (`referral_code`) va taklif havolasi (`https://avtotest.uz/login?ref=CODE`).
- **Nusxalash Tugmasi**: "Nusxalash" tugmasini bosganda havola buferga nusxalanadi va "Nusxalandi!" bildirishnomasi (Toast/Badge) chiqadi.
- **Statistika Grid**:
  - `total_invited`: Jami taklif qilingan do'stlar.
  - `total_rewarded`: Mukofot berilgan do'stlar (xarid qilganlar).
  - `bonus_days_earned`: Ishlangan jami VIP kunlar.
- **Referal Kod Qo'llash (Apply Referral Code)**:
  - Boshqa foydalanuvchining referal kodini kiritish uchun matn maydoni va "Biriktirish" tugmasi (`POST /api/v1/referral/apply`).
  - Xatoliklar va muvaffaqiyat bildirishnomasi (masalan: o'z kodini kiritsa "O'zingizning kodingizni kirita olmaysiz" xabari).

### 2.2 To'lovlar Tarixi Bo'limi (`PaymentHistorySection`)
- `GET /api/v1/me/payments?limit=20` API'dan foydalanuvchining to'lovlarini oladi.
- **Jadval / Ro'yxat ko'rinishi**:
  - **Sana**: To'lov amalga oshirilgan sana (masalan: `24-iyul, 2026, 20:15`).
  - **Tarif**: Tarif nomi va VIP muddat (masalan: `Gentra (30 kun)`).
  - **Provayder**: `Payme` yoki `Click` logotipi/belgisi.
  - **Summa**: So'mda formatlangan narx (masalan: `59 900 so'm`).
  - **Status**: Yashil ("To'langan"), Sariq ("Kutilmoqda"), Qizil ("Bekor qilingan") nishonlari.

## 3. Tarjima (i18n)
`uz-Latn.json`, `uz-Cyrl.json`, `ru.json` fayllarida `Referral` va `PaymentHistory` tugunlari qo'shiladi.
