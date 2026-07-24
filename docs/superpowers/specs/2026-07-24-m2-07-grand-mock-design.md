# M2-07: GRAND MOCK (Bosh Imtihon Simulyatsiyasi) Design Spec

## 1. Umumiy Ko'rinish va Maqsad
GRAND MOCK — AvtoTest platformasidagi eng mas'uliyatli va nufuzli imtihon rejimi. U Davlat YHQ imtihon markazlari (DYXX) shartlarini 100% aks ettiradi.
Foydalanuvchini haqiqiy imtihonga to'liq tayyorligini tasdiqlash uchun ushbu rejimga faqat **o'rtacha bilim ko'rsatgichi 85% yoki undan yuqori** (`mastery >= 85%`) bo'lgan hamda **VIP obunasi faol** bo'lgan foydalanuvchilar kirish huquqiga ega bo'ladilar.

## 2. Biznes Qoidalari va Anti-Fraud
1. **Kirish sharti (Eligibility)**:
   - `mastery_percent >= 85` (FSRS & Kategoriya o'zlashtirish algoritmi bo'yicha).
   - `is_vip == true` (Faol premium obuna).
2. **Imtihon shartlari**:
   - 20 ta tasodifiy savol (barcha kategoriyalardan).
   - 25 daqiqa taymer.
   - Maksimum 2 ta xatoga ruxsat (3-xatoda imtihon to'xtatilib, "Yiqildi" deb baholanadi).
3. **Natija va Mukofot**:
   - Muvaffaqiyatli o'tgach (≥18/20 to'g'ri), foydalanuvchiga konfetti salyutlar bilan "Grand Mock Sertifikati" hamda g'alaba nishoni beriladi.

## 3. Backend API Kontrakti

### 3.1 Eligibility (Kirish huquqi) tekshirish
- **Endpoint**: `GET /api/v1/mock/eligibility`
- **Auth**: Required
- **Response `200 OK`**:
```json
{
  "data": {
    "eligible": true,
    "mastery_percent": 88.5,
    "min_required_percent": 85.0,
    "is_vip": true,
    "reason": null
  }
}
```
- Agar tayyor bo'lmasa (`eligible: false`):
```json
{
  "data": {
    "eligible": false,
    "mastery_percent": 72.0,
    "min_required_percent": 85.0,
    "is_vip": true,
    "reason": "mastery_too_low"
  }
}
```

### 3.2 Grand Mock imtihonini boshlash
- **Endpoint**: `POST /api/v1/mock/start`
- **Auth**: Required
- **Request Body**: `{ "locale": "uz-Latn" }`
- **Response `201 Created`**: Standard `SessionDTO` (`mode: "grand_mock"`, 20 savol).
- **Error `403 Forbidden`**: `{ "error": { "code": "mock_not_eligible", "message": "Grand Mock imtihoni uchun bilim darajangiz kamida 85% bo'lishi va VIP aktiv bo'lishi kerak." } }`

## 4. Frontend Dizayn & UI

1. **Dashboard / Practice sahifasi**:
   - `GrandMockCard`: Oltin rangli VIP gradient va nishonlar bilan bezatilgan maxsus imtihon kartasi.
   - **Qulflangan holat**: Mastery 85% dan past bo'lsa, joriy foiz bar (progressbar, masalan `72% / 85%`) va qulf belgisi ko'rsatiladi.
   - **Ochilgan holat**: Yarqiragan "Bosh Imtihonni Boshlash" tugmasi.
2. **Natija ekrani va Konfetti**:
   - Imtihon muvaffaqiyatli topshirilgach (`passed: true`), `canvas-confetti` kutubxonasi orqali konfetti salyut otiladi va foydalanuvchiga raqamli sertifikat dialogi taqdim etiladi.

## 5. Verifikatsiya va Testlar
- Backend unit & integration testlar (`mock_test.go` eligibility, start clamp, error cases).
- Frontend Vitest unit testlar (`grand-mock-card.test.tsx`).
