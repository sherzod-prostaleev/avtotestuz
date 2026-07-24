# M2-08 — Tarif UI (Dizayn/Spec)

Sana: 2026-07-24 · Milestone: M2 · Plan: M2-08 · Qatlam: frontend (Next.js)

## Maqsad
`/premium` sahifasini statik matndan haqiqiy tarif kartalariga qayta qurish: `GET /api/v1/tariffs`dan olingan ma'lumot bilan, mashina-brend freyming ("Matiz=bepul, Nexia/Gentra/Malibu=pullik") va narx psixologiyasi (kunlik narx, chizilgan eski narx, tejash %, "ommabop" badge) bilan. "Sotib olish" tugmasi haqiqiy `POST /me/checkout`ni chaqiradi va Payme checkout URL'ga redirect qiladi — M2-09 (to'liq checkout oqimi: promo, provider tanlash, callback/polling) buning ustiga keyin quriladi, lekin M2-08'ning o'zi ham ishlaydigan minimal xarid yo'lini beradi.

## Ma'lumot manbai
- `GET /api/v1/tariffs` (M2-01'dan live, auth talab qilmaydi): `[{code, days, price_uzs, old_price_uzs, price_per_day_uzs, discount_percent, badge, name, description}]`. `price_per_day_uzs`/`discount_percent` backend'da allaqachon hisoblangan — frontend qayta hisoblamaydi.
- Haqiqiy seed (M2-01): `nexia`(7 kun, 24900, eski 34900) / `gentra`(30 kun, 59900, eski 99900, badge=`popular`) / `malibu`(75 kun, 109900, eski 199900, badge=`best_value`).
- `POST /api/v1/me/checkout` (auth, M2-02/M2-03'dan live): body `{tariff_code, provider}` → `{payment_id, checkout_url}`.
- `GET /api/v1/me/entitlement` (auth, mavjud): `{active, until}` — VIP holatini bilish uchun (banner ko'rsatish, bloklash uchun emas).

## Sahifa: `/premium` (`src/app/[locale]/(app)/premium/page.tsx`)
To'liq qayta yoziladi (hozirgi versiya to'liq statik, hech qanday API chaqirmaydi). Naqsh: `src/app/[locale]/(app)/profile/page.tsx`ning `useState`+`useEffect`+`apiGet` uslubi (loyihada `useQuery` hech qayerda ishlatilmagan, shu konvensiyaga amal qilinadi).

**Yuklash ketma-ketligi:** sahifa mount bo'lganda ikkita so'rov parallel (`Promise.all`): `apiGet("tariffs")` va `apiGet("me/entitlement")`. Yuklanish/xato holatlari `profile/page.tsx`dagi kabi (`loading` state, xato bo'lsa "qayta urinish" tugmasi bilan xabar).

**VIP banner:** agar `entitlement.active === true`, sahifa tepasida kichik banner: "VIP faol — muddati: <until sana>" (mavjud `toVIPDTO`ning frontend analogidagi formatlash, `Intl.DateTimeFormat` yoki mavjud sana-formatlash helper'i bilan). Tugmalar HAR DOIM aktiv qoladi (GrantDays muddatni uzaytiradi — bloklash noto'g'ri xatti-harakat bo'lardi).

## Kartalar (4 ta, gorizontal grid, mobil'da vertikal stack)
Tartib: **Matiz** (birinchi, chapda) → Nexia → Gentra → Malibu (API tartibidagi `sort_order` bo'yicha).

**Matiz karta (frontend-only, statik, API'dan kelmaydi):**
- Sarlavha: "Matiz" + kichik "Bepul" yorlig'i.
- Tavsif: joriy bepul imkoniyatlar (mavjud i18n matnlaridan yoki yangi qisqa matn — "cheklangan mashqlar, 1-bilet demo" kabi).
- CTA: `Button variant="outline"` — "Hozirgi tarifingiz" (bosilmaydigan/disabled, chunki bu allaqachon faol holat, xarid emas).

**Pullik kartalar (Nexia/Gentra/Malibu, API'dan):**
- Sarlavha: `name` (mashina nomi).
- Agar `badge` bor bo'lsa (`popular`→"Ommabop", `best_value`→"Eng foydali" — i18n key orqali map qilinadi, backend'dan kelgan raw string emas): karta tepasida pill-badge (mavjud premium sahifadagi `badge` div uslubi bilan bir xil: `rounded-full border border-gold/40 bg-gold/10 px-3 py-1 text-[11px] font-extrabold text-gold`). `badge` qiymati mavjud har qanday tarif (hozircha faqat `gentra`/popular) `border-2 border-gold` bilan ajratib ko'rsatiladi (boshqa kartalar `border border-border/80`, `Card`ning default holati) — bitta qo'shimcha `className` shartli qo'llanadi, yangi komponent variant kerak emas.
- Narx bloki: katta shrift bilan "kuniga ~`price_per_day_uzs` so'm" (asosiy), kichikroq pastda to'liq narx (`price_uzs`) + `days` kun. Agar `old_price_uzs` bor bo'lsa: chizilgan (`line-through`) eski narx + `discount_percent`% tejash badge.
- Tavsif: `description` (API'dan, lokalizatsiya qilingan).
- Feature-list: mavjud `Premium` i18n namespace'idagi `feature1`-`feature4` (barcha pullik kartalar uchun umumiy, provayderga bog'liq emas).
- CTA: `Button variant="gold"` — "Sotib olish". Bosilganda: `apiPost("me/checkout", {tariff_code: code, provider: "payme"})` → muvaffaqiyatli bo'lsa `window.location.href = result.checkout_url`. Xato bo'lsa (masalan tarif topilmadi) — kichik inline xabar, sahifadan chiqarilmaydi. Tugma bosilgan vaqtda "Yuklanmoqda..." holatiga o'tadi (disabled, boshqa bosishlarni oldini olish uchun).

## i18n
Yangi kalitlar `messages/{uz-Latn,uz-Cyrl,ru}.json`ning `Premium` namespace'iga qo'shiladi (mavjud kalitlar — `title`, `feature1-4`, `backHome` va h.k. — saqlanadi, chunki ular hali ham foydalaniladi; `m1Notice` o'chiriladi, chunki endi to'lov ishlaydi). Yangi kalitlar: `badgePopular`, `badgeBestValue`, `perDay` (masalan "kuniga"), `daysLabel`, `matizTitle`, `matizFree`, `matizDescription`, `matizCurrentPlan`, `buyButton`, `buyLoading`, `buyError`, `vipActiveBanner` (`{date}` interpolyatsiya bilan), `loadError`, `retry`.

## Testlar
- Mavjud `premium/page.test.tsx` (hozirgi statik versiyaga yozilgan) qayta yoziladi: API mock qilinib, 4 karta render bo'lishi, narx/badge/discount to'g'ri ko'rsatilishi, "Sotib olish" bosilganda `apiPost` to'g'ri chaqirilishi va redirect bo'lishi, VIP banner shartli render bo'lishi, xato holatlarida "qayta urinish" ishlashi tekshiriladi.

## Scope tashqarisi
Provider tanlash UI (hozircha default Payme — M2-09'da tanlash qo'shiladi), promo-kod maydoni (M2-05/M2-09), checkout'dan qaytish/callback sahifalari (M2-09), to'lov tarixi ko'rsatish (M2-10, backend M2-04'da tayyor).
