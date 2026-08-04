# B2B sinfxona stansiyasi: kriptografik bog'lash va login-siz kiosk

**Sana:** 2026-08-04
**Holat:** dizayn tasdiqlangan, implementatsiya kutilmoqda
**O'rnini bosadi:** `0055_b2b_station` dagi `X-Device-Fingerprint` modeli

## 1. Muammo

Hozirgi B2B modeli ikkita jiddiy kamchilikka ega.

**1.1. Har PC uchun alohida akkaunt kerak.** VIP `billing.Service.Status`
ichida `profileID` ustiga qurilgan (`backend/internal/billing/entitlement.go:81`).
Stansiya fingerprintи faqat *qo'shimcha* yo'l sifatida qo'shilgan — so'rov baribir
autentifikatsiyalangan profildan kelishi kerak. `profile.phone` esa `UNIQUE`
(`0002_identity.up.sql:3`). Natijada 30 kompyuterli sinfxona uchun 30 ta telefon
raqami va 30 ta parol kerak bo'ladi; 100 kompyuterli maktab uchun model butunlay
ishlamaydi. Hozirgi hujjat (`docs/b2b/school-station-pricing.md`) buni "har
o'quvchi o'z akkaunti bilan kiradi" deb rasmiylashtirgan, lekin real avtomaktab
sinfxonasida o'quvchilarda akkaunt yo'q.

**1.2. Bog'lash kriptografik emas.** `fingerprint` — brauzer `localStorage`
ichidagi tasodifiy UUID (`frontend/src/lib/device-fingerprint.ts:4`), u
`X-Device-Fingerprint` headerida oddiy matn sifatida yuboriladi
(`backend/internal/devicefp/devicefp.go:14`). Uni brauzer konsolidan ko'chirib
olib, istalgan kompyuterdan yuborish mumkin — VIP tekinga ochiladi. Ya'ni
"faqat bog'langan PCda ishlaydi" kafolati amalda mavjud emas. Bundan tashqari
brauzer keshi tozalansa bog'lanish yo'qoladi va seat behuda band bo'lib qoladi.

## 2. Maqsad

- Sinfxona PCsida **hech qanday login/parol bo'lmasin** (kiosk rejimi).
- Bitta tashkilotga **bitta ro'yxatga olish kodi** — 30 PC ham, 100 PC ham.
- Litsenziya **aynan o'sha jismoniy kompyuterga** bog'lansin va nusxa
  ko'chirilmasin.
- Qisqa internet uzilishlariga (72 soatgacha) chidasin.

### Maqsad emas (YAGNI)

- O'quvchi shaxsi bo'yicha progress kuzatuvi — kiosk rejimi ataylab anonim.
- Uzoq muddatli (oylik) offline ish — kontent o'g'irlanishi xavfi yuqori.
- Self-serve B2B checkout — litsenziya avvalgidek admin tomonidan ochiladi.

## 3. Arxitektura

Uchta komponent.

### 3.1. `avtotest-station` agenti

Windows uchun bitta Go binary (~8 MB). Yangi til/stack kiritilmaydi.

Vazifalari:

1. **HWID yig'ish** — Windows `MachineGuid` (registry), motherboard UUID va
   tizim diski seriya raqami birlashtirilib SHA-256 qilinadi. Natija
   `hwid_hash`.
2. **Kalit yaratish** — Ed25519 juftligi PCning o'zida tug'iladi. Private key
   DPAPI (`CryptProtectData`, **machine scope**) bilan shifrlanib diskda
   saqlanadi. Boshqa kompyuterga ko'chirilsa ochilmaydi.
3. **Lokal proxy** — `127.0.0.1:17817` da tinglaydi, hamma so'rovni
   `https://avtotest.uz` ga uzatadi va yo'lda stansiya tokenini qo'shadi.
   Frontend JS kaliti ham, tokenni ham ko'rmaydi: hammasi agent ichida.
4. **Kiosk ishga tushirish** — `chrome.exe --kiosk --app=http://127.0.0.1:17817`.
   Chrome topilmasa Edge (`msedge.exe`) ga tushadi.

Lokal proxy tanlanishining sababi: brauzer uchun hamma narsa bitta origin
(`127.0.0.1:17817`) bo'lib qoladi, CORS muammosi yo'q, va Next.js frontend
kodiga deyarli tegilmaydi.

### 3.2. Backend: `station` auth turi

Yangi JWT `typ: "station"`. Mavjud `learner` tokeni bilan yonma-yon yashaydi.
`devicefp` paketi va unga ishonadigan hamma yo'l olib tashlanadi.

### 3.3. Shadow profil

Har stansiya uchun bitta `profile` qatori (`kind = 'station'`). `phone` ustuni
`UNIQUE NOT NULL` bo'lgani uchun sintetik qiymat yoziladi: `'st:' ||
gen_random_uuid()`. Profil enroll tranzaksiyasida `b2b_station` qatoridan
**oldin** yaratiladi va uning idsi `station_profile_id` ga yoziladi (teskarisi
mumkin emas — chicken-and-egg). Bu qaror ataylab: `exam_session`,
`session_answer`, `variant_progress` va butun statistika mashinasi
`profile_id` ustiga qurilgan, shuning uchun kiosk rejimi ularning birortasiga
tegmaydi. Almashtiruvchi variant (hamma joyda `profile_id` ni nullable qilish)
o'nlab so'rovni buzardi.

`kind = 'station'` profillar quyidagilardan chiqarib tashlanadi: leaderboard,
referral, admin foydalanuvchilar ro'yxati, push obunalari, sertifikatlar.

**Sitting (o'tirish):** kiosk UIda "Yangi o'quvchi" tugmasi bo'ladi — u faol
sessiyani yopadi va ekranni tozalaydi, shunda keyingi o'quvchi oldingisining
natijasini ko'rmaydi. Server tomonda hech narsa saqlanmaydi; statistika
stansiya bo'yicha jamlanadi.

## 4. Ro'yxatga olish (enrollment)

### 4.1. Org kodi

`b2b_station_activate_code` (bir martalik, per-PC) o'rniga org darajasidagi
`b2b_org_enroll_code` keladi: `max_uses`, `used_count`, `expires_at`,
`revoked_at`. O'qituvchi `/teacher` panelidan "Ro'yxatga olish oynasini ochish
(2 soat)" tugmasi bilan yaratadi.

Standart qiymatlar: TTL 2 soat, `max_uses` = qolgan bo'sh seatlar soni.

### 4.2. Oqim

```
1. IT xodimi:  avtotest-station.exe /S /CODE=AVTO-7X2K     (GPO orqali 100 PCga)
   yoki:       dastur ochiladi, kod qo'lda kiritiladi
2. Agent:      HWID hisoblaydi, Ed25519 juftlik yaratadi
3. Agent  →    POST /b2b/stations/enroll
               { code, public_key, hwid_hash, label, os, agent_version }
4. Server:     kod faol? org faol? litsenziya tirik? active_stations < seats?
               → b2b_station qatori + shadow profil yaratiladi
               ← { station_id, lease }
5. Agent:      station_id + lease ni saqlaydi, kalitni DPAPI bilan muhrlaydi
```

31-PC `seats_exhausted` oladi. Seat bo'shatish uchun o'qituvchi eskisini
"Revoke" qiladi.

### 4.3. Poyga xavfsizligi

Hozirgi `ActivateStation` (`backend/internal/b2b/station.go:271`) seat sanog'ini
tranzaksiya ichida o'qiydi, lekin org qatorini qulflamaydi — READ COMMITTED
ostida ikkita parallel enroll bir xil `usedStations` ni o'qib, ikkalasi ham
o'tib ketishi mumkin. GPO bilan 100 PC bir vaqtda ishga tushganda bu real
stsenariy. Yechim: enroll tranzaksiyasi boshida
`SELECT ... FROM b2b_org WHERE id = $1 FOR UPDATE` bilan org qatorini qulflash.

## 5. Autentifikatsiya (har ishga tushishda)

```
1. Agent  →  GET  /b2b/stations/challenge?station_id=...
             ← { nonce, expires_in: 60 }
2. Agent:    sig = Ed25519_sign(priv, station_id || nonce || unix_ts)
3. Agent  →  POST /b2b/stations/token  { station_id, nonce, ts, sig, hwid_hash }
4. Server:   nonce Redis'da mavjudmi (bir martalik, 60s TTL)?
             public_key bilan imzo to'g'rimi?
             hwid_hash bazadagi bilan mos keladimi?
             stansiya active? org active? litsenziya tirik?
             ← { access_token (typ=station, TTL 15 min), lease }
5. Agent:    tokenni xotirada saqlaydi, 15 daqiqada yangilaydi
```

Nonce Redis'da saqlanadi — loyihada Redis limiter allaqachon mavjud
(`auth.Limiter`).

### 5.1. Hujum modeli

| Hujum | Nima bo'ladi |
|---|---|
| `station.key` faylini boshqa PCga ko'chirish | DPAPI machine scope → deshifrlanmaydi |
| Kalit baribir chiqarildi | `hwid_hash` mos kelmaydi → rad |
| Butun diskni VMga klonlash | motherboard UUID o'zgaradi → `hwid_hash` boshqa → rad |
| Tokenni ushlab olish | 15 daqiqada o'ladi; yangilash uchun kalit kerak |
| Imzoni replay qilish | nonce bir martalik, 60 soniya |
| Enroll kodini o'g'irlash | 2 soatlik oyna + seat cap; to'lgach kod o'lik |
| Bir kalitni ikki joyda ishlatish | bir vaqtda ikki xil IP → ogohlantirish (Faza 3) |

## 6. Offline lease

Server har token yangilanishida imzolangan ijara qaytaradi:

```json
{
  "station_id": "...",
  "org_id": "...",
  "license_ends_at": "2027-02-01T00:00:00Z",
  "lease_until": "2026-08-07T09:00:00Z",
  "issued_at": "2026-08-04T09:00:00Z",
  "agent_min_version": "1.0.0"
}
```

Imzo — **server** Ed25519 kaliti bilan (agent kalitidan alohida; public qismi
agent binarysiga kompilyatsiya vaqtida kiritiladi).

- Internet bor: har 30 daqiqada yangilanadi, `lease_until` doim oldinga suriladi.
- Internet yo'q: agent imzoni tekshiradi va `lease_until` o'tmagan bo'lsa
  ishlashda davom etadi (standart 72 soat).
- **Soatni orqaga surish:** agent ko'rgan eng oxirgi ishonchli vaqtni yozib
  boradi; tizim soati undan orqada bo'lsa — bloklaydi.
- `lease_until` o'tsa yoki `license_ends_at` tugasa — aniq xabar bilan to'xtaydi.

### 6.1. Kontent keshi

Agent proxy savol matnlari va rasmlarni ETag bilan diskka keshlaydi. Sinf bir
kun ishlagach kesh to'ladi va offline sessiya to'liq ishlaydi.

### 6.2. Natijalar navbati

Offline holatda javoblar lokal BoltDB'ga yoziladi va ulanish tiklanganda
yuboriladi. `session_answer` PK `(session_id, question_id)`
(`0004_learning.up.sql:30`) — ya'ni qayta yuborish idempotent, qo'shimcha
deduplikatsiya kerak emas.

## 7. Ma'lumotlar bazasi o'zgarishlari

Migratsiya `0056_b2b_station_key`:

```sql
ALTER TABLE profile
  ADD COLUMN kind text NOT NULL DEFAULT 'user'
  CHECK (kind IN ('user','station'));

ALTER TABLE b2b_station
  ADD COLUMN public_key         text,
  ADD COLUMN hwid_hash          text,
  ADD COLUMN agent_version      text NOT NULL DEFAULT '',
  ADD COLUMN last_ip            inet,
  ADD COLUMN station_profile_id uuid REFERENCES profile(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX b2b_station_active_hwid_uidx
  ON b2b_station (hwid_hash) WHERE status = 'active' AND hwid_hash IS NOT NULL;

CREATE TABLE b2b_org_enroll_code (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  code       text NOT NULL UNIQUE,
  max_uses   int  NOT NULL CHECK (max_uses > 0),
  used_count int  NOT NULL DEFAULT 0,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX b2b_org_enroll_code_org_idx
  ON b2b_org_enroll_code (org_id, expires_at DESC);
```

Prod'da faol stansiya yo'qligi sababli (8.1) `fingerprint` ustuni va uning
unique indexi shu migratsiyada darhol o'chiriladi; `public_key` va `hwid_hash`
esa `NOT NULL` bo'ladi:

```sql
TRUNCATE b2b_station, b2b_station_activate_code;
ALTER TABLE b2b_station
  DROP COLUMN fingerprint,
  ALTER COLUMN public_key SET NOT NULL,
  ALTER COLUMN hwid_hash  SET NOT NULL;
DROP TABLE b2b_station_activate_code;
```

## 8. Kod o'zgarishlari

| Fayl | O'zgarish |
|---|---|
| `backend/internal/b2b/enroll.go` | **yangi** — org kodi bilan enroll, org qatori qulflangan holda seat cap |
| `backend/internal/b2b/station_auth.go` | **yangi** — nonce, imzo tekshiruvi, station JWT, lease imzolash |
| `backend/internal/b2b/station.go` | `ActiveStationVIP` fingerprint o'rniga `station_id` bo'yicha |
| `backend/internal/b2b/handlers.go` | yangi public marshrutlar: `/b2b/stations/{enroll,challenge,token}` |
| `backend/internal/auth/jwt.go` | `typ: "station"` claim turi va uni tekshiruvchi middleware |
| `backend/internal/billing/entitlement.go` | `stationFingerprint` → claims'dan `station_id` |
| `backend/internal/devicefp/` | **o'chiriladi** (shartlari uchun 8.1 ga qarang) |
| `backend/internal/db/queries/leaderboard.sql` | `kind = 'user'` filtri |
| `frontend/src/lib/device-fingerprint.ts` | **o'chiriladi** |
| `frontend/src/app/[locale]/(app)/station/` | **yangi** — kiosk UI, "Yangi o'quvchi" tugmasi |
| `frontend/src/app/[locale]/(app)/teacher/page.tsx` | enroll oynasini ochish/yopish, PC ro'yxati |
| `station/` | **yangi papka** — Go agent |
| `docs/b2b/school-station-pricing.md` | login qoidalari qayta yoziladi |

### 8.1. Migratsiya xavfsizligi

**Hal qilindi (2026-08-04):** prod'da faol stansiya yo'q (0 ta), real maktab
hali ulanmagan. Demak deprecation flagi kerak emas — `devicefp` paketi, uning
middleware'i, frontend `device-fingerprint.ts` va `b2b_station.fingerprint`
ustuni **Faza 1da darhol** o'chiriladi. Orqaga moslik yuki yo'q; migratsiya
`b2b_station` va `b2b_station_activate_code` jadvallarini bo'shatib
(`TRUNCATE`) yangi sxemaga o'tkazadi.

## 9. Fazalar

**Faza 1 — MVP.** Migratsiya, enroll + seat cap, challenge/imzo/token, shadow
profil, kiosk UI, `/teacher` paneli, minimal agent (kesh va offline yo'q).
Shu fazaning o'zi "30 PC = 0 login" muammosini yopadi.

**Faza 2 — chidamlilik.** Offline lease, kontent keshi, natijalar navbati,
soat orqaga surilishidan himoya.

**Faza 3 — operatsiya.** MSI/GPO silent install, auto-update, anomaliya
aniqlash (bitta stansiya bir vaqtda ikki IPdan), o'qituvchi statistikasi,
`fingerprint` ustunini o'chirish.

## 10. Test rejasi

**Go unit:**
- 30 seatли orgda 31-enroll `ErrSeatsExhausted` qaytaradi
- muddati o'tgan kod, `used_count >= max_uses` kod, `revoked_at` qo'yilgan kod → rad
- noto'g'ri imzo, boshqa stansiyaning kaliti → rad
- bir nonce'ni ikki marta ishlatish → rad
- `hwid_hash` mos kelmasligi → rad
- suspended org / muddati tugagan litsenziya → token berilmaydi
- lease muddati o'tgan → agent bloklaydi
- tizim soati orqaga surilgan → agent bloklaydi

**Tranzaksiya poygasi:** 30 seatga 40 parallel enroll → aniq 30 tasi o'tadi.

**Integratsiya:** test ichidagi soxta agent (Go kutubxonasi sifatida) to'liq
oqimni o'ynaydi: enroll → challenge → token → VIP so'rov → revoke → rad.

**Qo'lda:** ikkita real PC, kalit faylini ko'chirish urinishi, internetni uzish.
