# M2-10: To'lov Tarixi va Referal UI Implementation Plan

Ushbu reja foydalanuvchi profiliga (`/profile`) To'lovlar Tarixi va Referal Dasturi bo'limlarini qo'shishni ko'zda tutadi.

## Proposed Changes

### Frontend (`/frontend`)

#### [NEW] [referral-card.tsx](file:///home/sher/Рабочий стол/avtotest/frontend/src/components/profile/referral-card.tsx)
- Referal kod va havola nusxalash tugmasi.
- Statistika (jami takliflar, mukofotlar, ishlangan kunlar).
- Taklif kodini kiritish/biriktirish maydoni.

#### [NEW] [payment-history-card.tsx](file:///home/sher/Рабочий стол/avtotest/frontend/src/components/profile/payment-history-card.tsx)
- Foydalanuvchining o'tgan to'lovlari ro'yxati (Payme/Click icon, status, summa, sana, tarif).

#### [MODIFY] [profile/page.tsx](file:///home/sher/Рабочий стол/avtotest/frontend/src/app/[locale]/(app)/profile/page.tsx)
- `ReferralCard` va `PaymentHistoryCard` komponentlarini profil sahifasiga integratsiya qilish.

#### [NEW] [referral-card.test.tsx](file:///home/sher/Рабочий стол/avtotest/frontend/src/components/profile/referral-card.test.tsx)
- Referal kartasining Vitest unit testlari.

#### [NEW] [payment-history-card.test.tsx](file:///home/sher/Рабочий стол/avtotest/frontend/src/components/profile/payment-history-card.test.tsx)
- To'lov tarixi kartasining Vitest unit testlari.

#### [MODIFY] [uz-Latn.json](file:///home/sher/Рабочий стол/avtotest/frontend/messages/uz-Latn.json), [uz-Cyrl.json](file:///home/sher/Рабочий стол/avtotest/frontend/messages/uz-Cyrl.json), [ru.json](file:///home/sher/Рабочий стол/avtotest/frontend/messages/ru.json)
- Tarjima fayllarida `Referral` va `PaymentHistory` tugunlarini qo'shish.

## Verification Plan

### Automated Tests
- Frontend Typecheck: `cd frontend && npm run typecheck`
- Frontend Unit Tests: `cd frontend && npm run test`
- Backend Regression Tests: `export PATH="$HOME/.local/go/bin:$PATH" && cd backend && go test ./... -p 1 -count=1`
