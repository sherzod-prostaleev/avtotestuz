# Next.js Frontend Foundation (Phase A) — Design Spec

**Sana:** 2026-07-22
**Status:** Foydalanuvchi tasdig'i kutilmoqda
**Miqyos:** Faqat Bosqich A — Next.js skelet + dizayn-tizim + 3 ta vizual-tasdiq mockup sahifa. Bosqich B (real API ulanishi + qolgan ~12 sahifa + to'liq i18n + E2E) alohida spec/reja bo'lib, faqat shu bosqich vizual jihatdan tasdiqlangandan keyin yoziladi.

## 0. Nima uchun bu spec kerak

`AVTOTEST-MASTER-PROMPT.txt` mahsulot-talablarini (barcha sahifalar, biznes-qoidalar, API-kontrakt, 9 qattiq saboq) va dizayn-yo'nalishni (§6, ranglar/tipografiya) allaqachon batafsil belgilagan — bu spec ularni qaytadan ixtiro qilmaydi, ularga **havola qiladi**. Bu spec faqat quyidagilarni hal qiladi: (a) Next.js loyihasining texnik arxitekturasi (papka tuzilishi, state-boshqaruv, auth-BFF, i18n mexanizmi), (b) dizayn-tizimning kod-darajasidagi implementatsiyasi (tokenlar, shrift, komponent-andozalar), (c) "avval dizayn, keyin funksionallik" qoidasiga rioya qilib, birinchi ishlab chiqiladigan 3 ta mockup sahifaning aniq qamrovi.

Master-promptdagi tasdiqlangan qarorlar (bu spec ularga bo'ysunadi, qaytarmaydi):
- Stack: Next.js (App Router) + TypeScript + Tailwind + shadcn/ui, Go backend o'zgarmaydi (§0)
- Auth: httpOnly cookie + Next.js route-handler proxy (BFF), localStorage RAD ETILGAN (§0-c, 2026-07-21 qarori)
- Demo-savol uchun public backend-endpoint allaqachon qurilgan va tasdiqlangan (`GET/POST /api/v1/demo/question|answer`, commit `7a17371`)
- Kontent endi 13 kategoriyaga ega (commit `7fe900c`) — dashboard/stats mockup'lari shu haqiqiy taksonomiyani aks ettiradi
- Dizayn-tizim asoslari: §6.0 (dark-default navy `#0E1526`, aksent indigo `#6C63FF`, semantik ranglar alohida, pill-tugmalar 3D-bosim, kartalar 16-20px radius, Baloo2/Nunito+Manrope/Inter shrift juftligi, framer-motion)

## 1. Arxitektura

```
frontend/                          # repo ildizida, backend/ bilan simmetrik
  src/
    app/
      (public)/
        page.tsx                   # Landing (Bosqich A mockup #1)
      (app)/
        dashboard/page.tsx         # Dashboard (Bosqich A mockup #2)
        exam/[sessionId]/page.tsx  # Savol-yechish (Bosqich A mockup #3)
      globals.css                  # dizayn-tokenlar (CSS custom properties)
      layout.tsx
    components/
      ui/                          # shadcn/ui — Button/Card/Dialog/Input/... (bu bosqichda customize qilinadi: pill+3D-shadow variant)
      shared/                      # Bosqich A'da FAQAT 3 mockup sahifa uchun kerak bo'lganlari: QuestionCard, AnswerOption, CountdownTimer, MasteryBar, ResultRing
    lib/
      design-tokens.ts             # ranglar/shrift/spacing konstantalari (Tailwind config'dan foydalaniladigan yagona manba)
      mock-data.ts                 # Bosqich A mockup'lari uchun FAKE/STATIK ma'lumot (real API chaqirilmaydi), uz-Latn matn bilan hardcoded
  tests/
    unit/                          # Vitest + Testing Library — component-darajasida (masalan QuestionCard holatlari)
  package.json, tsconfig.json, tailwind.config.ts, next.config.ts
```

Bosqich B qo'shadigan narsalar (bu spec doirasidan tashqarida, shu yerda faqat kelajakdagi joylashuvni ko'rsatish uchun eslatiladi): `app/[locale]/` segmenti (next-intl, uz-Latn|uz-Cyrl|ru) barcha yo'llarni o'rab oladi, `(auth)/login|verify`, `(app)/variants|practice|mistakes|saved|stats|signs|profile|premium`, `app/api/proxy/[...path]/route.ts` + `app/api/auth/refresh/route.ts`, `features/`, `messages/*.json`, `e2e/`. Bosqich A ularning HECH BIRINI stub sifatida ham yaratmaydi.

**Nega bu bo'linish:** Bosqich A hech qanday tarmoq-chaqiruvi va i18n-routing qilmaydi (faqat `lib/mock-data.ts`dan statik, bitta tildagi — uz-Latn — ma'lumot) — maqsad FAQAT vizual/dizayn tasdig'i, funksional yoki ko'p-tillilik to'g'riligi emas. Komponentlar (QuestionCard, AnswerOption va h.k.) Bosqich B'da xuddi shu shaklda real ma'lumot bilan qayta ishlatiladi — ular allaqachon "well-defined interface" bilan (props orqali) yoziladi, shuning uchun Bosqich B faqat ularni real hook'larga va `next-intl`ga ulaydi, qayta yozmaydi.

## 2. State-boshqaruv va data-fetching (Bosqich B uchun arxitekturaviy qaror, Bosqich A'da ishlatilmaydi)

- **TanStack Query**: server-ma'lumot keshi (savollar, sessiya holati, profil) — avtomatik qayta-urinish, `staleTime`/`invalidate` orqali bilet-unlock kabi holatlarni yangilash. Flutter-davridagi "mid-session javob-xatosida qayta-urinish" talabi shu bilan tabiiy hal bo'ladi.
- **Zustand**: sof klient-holat — joriy savol indeksi, taymer qolgan vaqti, F1-F5 tanlovi, fullscreen/shrift-sozlamalari. Server-holat bilan aralashmaydi (ikkalasini bir joyga qo'yish — Flutter-davridagi Riverpod-hydration-race saboqiga o'xshash muammoni qaytarishi mumkin edi, shu sabab ataylab ajratilgan).
- Bosqich A bu ikkisini o'rnatadi (`package.json`ga qo'shiladi) lekin hali ishlatmaydi — mockup sahifalar `lib/mock-data.ts`dan to'g'ridan-to'g'ri o'qiydi.

## 3. Auth arxitekturasi (Bosqich B uchun, Bosqich A'da ishlatilmaydi — hujjatlashtirish uchun shu yerda)

Route Handler (`app/api/auth/*`, `app/api/proxy/[...path]`) — yagona joy Go API bilan Bearer token orqali gaplashadigan. OTP-verify muvaffaqiyatli bo'lsa, Route Handler access+refresh tokenlarni `httpOnly, Secure, SameSite=Lax` cookie sifatida o'rnatadi. Har bir keyingi klient-so'rov same-origin `/api/proxy/...`ga boradi; proxy cookie'dan tokenni o'qiydi, Go API'ga forward qiladi. 401 kelsa: single-flight refresh (bitta paylashilgan promise — Flutter'dagi bug aynan shu yerda: ikkita parallel 401 ikkita mustaqil refresh chaqirsa, backend buni "o'g'irlangan token" deb hisoblab hamma sessiyani bekor qiladi).

## 4. Dizayn-tizim implementatsiyasi

`globals.css`da CSS custom properties (`--bg`, `--surface`, `--accent`, `--success`, `--danger`, `--streak`, `--gold`, ...) dark (default) va light uchun; `tailwind.config.ts` shu o'zgaruvchilarga havola qiladi (rang-qiymatlarni ikki joyda takrorlamaslik uchun). Shriftlar: Baloo 2 (yoki Nunito, ikkalasi ham Google Fonts orqali `next/font`) sarlavha/raqamlar uchun, Manrope matn uchun — `next/font`bilan self-hosted (tashqi CDN so'rovisiz, tezlik uchun). shadcn/ui'ning `Button` komponenti pill (`rounded-full`) + pastki qattiq soya (`shadow-[0_4px_0_0_var(--accent-shadow)]`) + bosilganda pastga siljish (`active:translate-y-1 active:shadow-none`) variant bilan kengaytiriladi. `Card`: 16-20px radius, nozik border, `--surface` fon.

## 5. Uchta mockup sahifa (Bosqich A — statik ma'lumot, real API yo'q)

Har biri master-promptning tegishli bo'limiga aynan mos (raqamlar o'sha yerdan):

1. **Landing** (§6.1): hero (qalin sarlavha + aksent so'z + pill CTA), jonli demo-savol bloki (bu mockup'da **statik** savol — real `/demo/question` ulanishi Bosqich B), proof-stats qatori ("1235 savol · 61 bilet · 13 mavzu · 3 til"), "Nega biz?" 4-6 karta, "Qanday ishlaydi" 3 qadam, FAQ akkordeon, footer.
2. **Dashboard** (§6.3): salomlashuv+avatar+VIP-badge, streak-karta (olov+kun+progress-bar), tayyorlik-% halqa, 4 navigatsiya-karta (Biletlar/Imtihon/Mashq/Xatolar), til/tema-toggle header'da. Statik ma'lumotlar 13 haqiqiy kategoriya nomlaridan birini namuna sifatida ishlatadi (haqiqiy taqsimotni aks ettirish uchun, ixtiro qilingan kategoriya emas).
3. **Savol-yechish ekrani** (§6.5, "eng muhim ekran"): 4 vizual holat bir sahifada namoyish qilinadi (yoki interaktiv almashtirgich bilan) — javob berilmagan, to'g'ri-javob-berilgan (yashil+izoh-bloklar), xato-javob-berilgan (qizil+to'g'risi yashil), imtihon-rejimi (rang-feedback'siz). F1-F5 klaviatura-yorliqlari, 1-20 navigator-chiplari, taymer (oxirgi daqiqa qizil pulsatsiya) shu ekranda.

## 6. Xato holatlari (Bosqich A doirasida)

Bosqich A tarmoq-chaqiruv qilmagani uchun runtime xato-holatlari yo'q. Yagona e'tibor talab qiladigan narsa: `next/font` yuklanish-xatosi (fallback shrift zanjiri `tailwind.config.ts`da belgilanadi) va mockup-sahifalarning responsive layout'da (mobil-web) buzilmasligi (breakpoint-testlari qo'lda brauzerda tekshiriladi).

## 7. Testlash

- **Vitest + Testing Library**: `QuestionCard`, `AnswerOption`, `CountdownTimer`, `MasteryBar`, `ResultRing` — har biri holatlar bo'yicha (masalan CountdownTimer: normal/oxirgi-daqiqa-qizil holatlari; AnswerOption: neytral/to'g'ri/xato/imtihon-rejimi holatlari).
- Playwright E2E — Bosqich A'da YO'Q (real backend/auth kerak, Bosqich B doirasida).
- Qabul mezoni (Bosqich A): `npm run lint`, `npm run typecheck`, `npm test` toza; `npm run build` muvaffaqiyatli; 3 mockup sahifa brauzerda (dark+light, mobil+desktop kenglik) qo'lda ko'zdan kechiriladi va foydalanuvchiga skrinshot/lokal-server orqali ko'rsatiladi.

## 8. Qamrovdan tashqari (ataylab, Bosqich B'ga qoldirilgan)

Auth oqimi, real API-ulanish, i18n'ning ru/uz-Cyrl tarjimalari, qolgan ~12 sahifa, Playwright E2E, `next-intl` runtime-konfiguratsiyasi, CI'ga frontend job qo'shish. Bularning hech biri Bosqich A kod-bazasida stub sifatida ham yozilmaydi (YAGNI — bo'sh-lekin-mavjud sahifa yaratish master-promptning 6-saboqini ("halol placeholder") buzadi, agar navigatsiyadan ochilib qolsa; Bosqich A'da bu sahifalarga hali hech qanday navigatsiya yo'q, chunki header/nav Bosqich B'da quriladi).
