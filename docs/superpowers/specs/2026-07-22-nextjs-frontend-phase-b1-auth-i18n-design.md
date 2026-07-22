# Next.js Frontend Phase B1 — Auth + BFF + i18n Foundation — Design Spec

**Sana:** 2026-07-22
**Status:** Foydalanuvchi tasdig'i kutilmoqda
**Miqyos:** Bosqich B'ning birinchi bo'lagi — auth (real login/OTP), BFF-proxy, i18n infratuzilmasi. Real content/session ulanishi (B2), to'liq test-yechish tajribasi (B3), qolgan sahifalar (B4), E2E (B5) — bularning hech biri bu spec doirasida emas.

## 0. Nima uchun bu spec kerak, va nega birinchi

Foydalanuvchi (2026-07-22 muhokamasi) auth-avval tartibini tanladi: "hech narsa auth'siz ishlamaydi" — content/session ulanishi, imtihon tajribasi va qolgan barcha sahifalar autentifikatsiyalangan foydalanuvchiga bog'liq. Bu spec Next.js frontend'ning eng ko'p Critical bug chiqargan zonasini (Flutter-davridagi single-flight refresh, token-rotation) TypeScript'da qaytadan quradi — endi httpOnly cookie + BFF pattern bilan, spec `2026-07-22-nextjs-frontend-foundation-design.md` §3'da qayd etilgan qaror asosida.

**i18n shu bosqichga qo'shilgan sababi:** `AVTOTEST-MASTER-PROMPT.txt` 8-saboq: "i18n BOSHIDAN: har matn darhol 3 tilda tarjima tizimiga kirsin." Bosqich A'ning 3 sahifasi qattiq-yozilgan o'zbekcha matn bilan qurilgan edi (ataylab — o'sha bosqichning yagona maqsadi vizual tasdiq edi). Endi haqiqiy sahifalar qurila boshlagach, bu qarzni darhol (3 sahifada arzon) yopish kerak — 12 sahifaga cho'zilsa qimmatlashadi.

## 1. Arxitektura

```
frontend/
  messages/
    uz-Latn.json  uz-Cyrl.json  ru.json
  src/
    middleware.ts                     # next-intl locale-routing + auth-guard (coarse) — MUST live under src/ (Next.js convention when a src/ dir is used; wrong location = silently never invoked)
    i18n/request.ts                   # next-intl server config
    app/
      [locale]/
        layout.tsx                    # NextIntlClientProvider + (Bosqich A'dagi) ThemeToggle joyi
        (public)/page.tsx             # Landing — Bosqich A'dan ko'chiriladi, matnlar i18n'ga chiqariladi
        (auth)/
          login/page.tsx              # telefon kiritish
          login/verify/page.tsx       # 6-xonali OTP
        (app)/
          dashboard/page.tsx          # Bosqich A'dan ko'chiriladi
          exam-mockup/page.tsx        # Bosqich A'dan ko'chiriladi (B3'gacha mock qoladi)
      api/
        auth/
          otp/request/route.ts
          otp/verify/route.ts
          refresh/route.ts
          logout/route.ts
        proxy/[...path]/route.ts      # generik autentifikatsiyalangan proxy
    lib/
      backend.ts                      # BACKEND_URL env + backendFetch() helper
      auth-cookies.ts                 # cookie nomlari + set/clear helper'lar
      refresh-lock.ts                 # single-flight refresh (module-level promise)
```

**Auth-holat butunlay server-tarafda** (middleware + Server Component'larda `next/headers`'dan cookie o'qish) — bu bosqichda Zustand ISHLATILMAYDI (Bosqich A spec'idagi rejaga mos: Zustand'ning birinchi ishlatilishi B3'da, imtihon-sessiya klient-holati uchun).

## 2. Cookie modeli

- `at` (access token) — httpOnly, `Secure` (prod'da), `SameSite=Lax`, `maxAge` backend'ning 15-daqiqalik access-token muddatiga mos.
- `rt` (refresh token) — httpOnly, `Secure`, `SameSite=Lax`, `maxAge` 30 kun (backend'ning rotating-refresh muddati).
- Ikkalasi ham klient JavaScript'iga HECH QACHON ko'rinmaydi (httpOnly) — faqat Route Handler'lar va Server Component'lar `next/headers`'dan o'qiydi.

## 3. Route Handler'lar (BFF)

- `POST /api/auth/otp/request {phone, channel}` → backend `/api/v1/auth/otp/request`'ga proxy, javobni (shu jumladan dev-rejimdagi `debug_code`ni) o'zgarishsiz qaytaradi.
- `POST /api/auth/otp/verify {phone, code}` → backend'ga proxy; muvaffaqiyatda backend qaytargan `access_token`/`refresh_token`ni **cookie sifatida o'rnatadi**, klientga faqat `{ok: true}` qaytaradi (token hech qachon response body'da klientga chiqmaydi).
- `POST /api/auth/refresh` → `rt` cookie'dan tokenni o'qiydi, backend `/auth/refresh`ni chaqiradi, yangi cookie'larni o'rnatadi. **Single-flight**: `lib/refresh-lock.ts`dagi modul-darajasidagi promise orqali — bir vaqtda kelgan bir nechta so'rov FAQAT bitta haqiqiy backend-chaqiruvini ishga tushiradi (Flutter-davridagi Critical bug — ikkita parallel refresh backend'ning replay-detection'ini ishga tushirib, foydalanuvchining BARCHA sessiyalarini bekor qilgan edi — shu yerda oldini olinadi). Eslatma: bu yechim bitta uzoq-ishlaydigan Node protsessi (`next start`) doirasida to'g'ri ishlaydi; agar kelajakda serverless/edge-runtime'ga ko'chirilsa, bu holatni saqlash uchun Redis kabi tashqi do'kon kerak bo'ladi (hozircha qamrovdan tashqari, faqat eslatma sifatida yozib qo'yiladi).
- `POST /api/auth/logout` → `rt` cookie bilan backend `/auth/logout`ni chaqiradi; **backend javobidan qat'i nazar** ikkala cookie'ni tozalaydi (Flutter-davridagi tasdiqlangan naqsh: "logout() clears tokens on both thrown exception and Result.err").
- `ANY /api/proxy/[...path]` → `at` cookie'dan Bearer biriktirib backend `/api/v1/${path}`ga forward qiladi (method, body, query saqlanadi). Backend 401 qaytarsa: bitta marta ichki refresh chaqiriladi (yuqoridagi single-flight orqali), muvaffaqiyatli bo'lsa so'rov bir marta qayta uriniladi; refresh ham muvaffaqiyatsiz bo'lsa — cookie'lar tozalanadi va 401 klientga o'tkaziladi (klient tomon buni "chiqib ketish" sifatida ishlaydi — sahifa-darajasida redirect B2'da content-fetching bilan birga ulanadi, bu spec faqat proxy'ning o'zini qamrab oladi).

## 4. `middleware.ts` — ikki qatlamli guard

1. **next-intl locale-routing**: URL'da locale segmenti yo'q bo'lsa, `defaultLocale` (`uz-Latn`) bilan qayta yo'naltiradi.
2. **Coarse auth-guard**: `(app)/*` yo'llariga faqat `at` YOKI `rt` cookie **mavjudligi** tekshiriladi (JWT imzosi TEKSHIRILMAYDI — bu maxsus qaror: middleware'da JWT-secret'ni frontend'ga dublikat qilish xavfsizlik jihatdan noto'g'ri bo'lardi). Cookie yo'q bo'lsa `/login`ga redirect. Bu — tezkor, "aniq chiqib ketgan" foydalanuvchini himoyalangan kontentni ko'rishdan saqlaydigan birinchi qatlam; token haqiqatan yaroqli-ligini tekshirish (ikkinchi qatlam) B2'da real ma'lumot so'rovi orqali sodir bo'ladi (401 kelsa, o'sha sahifa `/login`ga redirect qiladi).
3. `/login`, `/login/verify`ga **allaqachon** cookie bilan kirilsa → `/dashboard`ga redirect (aksincha holat).

## 5. UI sahifalari

- `(auth)/login/page.tsx` — markazlashgan karta, `+998` prefiksli formatlangan telefon input, katta pill "Davom etish" (master-prompt §6.2). Muvaffaqiyatli `POST /api/auth/otp/request`dan keyin `/login/verify`ga navigatsiya (telefon raqami query-param yoki qisqa muddatli holat orqali uzatiladi).
- `(auth)/login/verify/page.tsx` — 6 ta alohida raqam-katakcha (avto-o'tish), 60s qayta-yuborish taymeri, "telefonni tahrirlash" havolasi, dev-rejimda `debug_code`ni ko'rsatish. Muvaffaqiyatli `POST /api/auth/otp/verify`dan keyin `/dashboard`ga navigatsiya.

## 6. i18n

`next-intl` — `locales: ["uz-Latn", "uz-Cyrl", "ru"]`, `defaultLocale: "uz-Latn"` (backend'ning mavjud locale-kod konvensiyasiga aynan mos — README'da hujjatlashtirilgan `?locale=uz-Latn` bilan bir xil). Bosqich A'ning 3 sahifasidagi barcha qattiq-yozilgan matnlar `messages/uz-Latn.json`ga chiqariladi, so'ng `uz-Cyrl.json`/`ru.json`ga tarjima qilinadi (mock-data.ts'dagi kontent-matnlar — demo savol va h.k. — bu safar TARJIMA QILINMAYDI, chunki ular real kontent emas, faqat vizual-namuna; UI-chrome matnlari — sarlavhalar, tugmalar, yorliqlar — tarjima qilinadi).

## 7. Xato holatlari

- OTP so'rov/tasdiqlash xatolari (`invalid_phone`, `rate_limited`, `invalid_code`, `expired_code`, `too_many_attempts`) — backend kodlari o'zgarishsiz klientga o'tkaziladi, UI har biriga mos xabar ko'rsatadi.
- Tarmoq xatosi (backend ishlamayotgan bo'lsa) — Route Handler 502 qaytaradi, aniq `network_error` kodi bilan (backend kodlari bilan aralashmasligi uchun).
- Refresh muvaffaqiyatsiz (`invalid_refresh`, `refresh_reused`) — cookie'lar tozalanadi, 401 klientga o'tadi.

## 8. Testlash

- Route Handler'lar: har biri eksport qilingan `POST`/`GET` funksiyasini to'g'ridan-to'g'ri chaqirib (Next.js Route Handler'lar oddiy funksiya — server ishga tushirilishi shart emas), global `fetch`ni `vi.stubGlobal`bilan mock qilib testlanadi.
- `refresh-lock.ts`: bir vaqtda 2 marta chaqirilganda backend-`fetch` FAQAT bir marta chaqirilishini tasdiqlovchi test (Flutter-davridagi single-flight testining TypeScript ekvivalenti).
- `middleware.ts`: mock `NextRequest` bilan uchala holat (cookie yo'q+himoyalangan yo'l→redirect; cookie bor+login yo'li→redirect; cookie yo'q+ommaviy yo'l→o'tkazish).
- UI sahifalari: Vitest+Testing Library, Bosqich A konvensiyasi bo'yicha.

## 9. Qamrovdan tashqari (B2+)

Real content/session API ulanishi (hozircha mock-data qoladi — login qilingandan keyin ham dashboard/exam-mockup hali statik ma'lumot ko'rsatadi, bu B2'ning ishi), TanStack Query hook'lari, real sessiya oqimi, qolgan sahifalar, Playwright E2E.
