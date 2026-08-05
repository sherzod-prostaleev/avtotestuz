# B2B: admin-only enrollment and a per-school installer

**Sana:** 2026-08-06
**Holat:** dizayn tasdiqlangan, implementatsiya kutilmoqda
**Oldingi spec:** `2026-08-04-b2b-station-kiosk-design.md` (Faza 1 — bajarilgan, prod'da)

## 1. Muammo

Faza 1 sinfxona PClarini kalit bilan bog'ladi va login'ni yo'q qildi. Lekin PCni
ulash yo'li amalda ishlamaydi.

**1.1. Kod faqat `/teacher` orqali olinadi.** Buning uchun maktabda odam bo'lishi,
u ro'yxatdan o'tishi, admin uni a'zo qilishi va rolini `teacher` ga o'zgartirishi
kerak. Admin UI a'zoni doim `student` qilib qo'shadi (`role: "student"` kodda qattiq
yozilgan), ya'ni rolni alohida almashtirish shart. Operator uzoqdan yordam
berayotganda bu zanjir uzilib qoladi — birinchi maktabni ulashda aynan shunday bo'ldi.

**1.2. Rollar hech qanday foyda bermayapti.** Prod'da 1 ta maktab, 1 ta a'zo (test
uchun qo'shilgan), 0 ta stansiya. `owner/teacher/student`, taklif tizimi va a'zolik
jadvallari — hozirgi masshtabda faqat ortiqcha mashina.

**1.3. Har PCda kod qo'lda kiritiladi.** 30 PC uchun 30 marta. Kod 2 soat yashaydi
(maksimum 24), ya'ni o'rnatish bir kunga cho'zilsa kod o'ladi.

## 2. Maqsad

- PCni ulash **faqat admin panel orqali** boshqarilsin; maktabga odam biriktirish
  tushunchasi butunlay yo'qolsin.
- Admin har maktab uchun **tayyor .exe** yuklab olsin: fayl ichida o'sha maktabning
  o'rnatish kaliti bo'lsin, PCda hech narsa kiritilmasin.
- EXE ishga tushirilgach PC har kuni yoqilganda kiosk **o'zi ochilsin**.

### Maqsad emas

- **Offline ish** — internet uzilishiga chidamlilik. Bu keyingi spec (imzolangan
  lease, kontent keshi, natijalar navbati, soat orqaga surilishidan himoya). Bu
  spec tugagach darhol shunga o'tiladi. **Bu ish tugaganda ham internet ishonchsiz
  maktabga "offline ishlaydi" deb va'da berib bo'lmaydi.**
- MSI paketi, auto-update, Authenticode imzosi.

## 3. Model qanday soddalashadi

**Qoladi:** `b2b_org` + `b2b_org_license` + `b2b_station` + `b2b_org_enroll_code`.

**Yo'qoladi:** `b2b_org_member`, `b2b_invite`, `owner/teacher/student` rollari,
taklif tizimi, `/teacher` sahifasi va uning 12 ta endpointi, hamda **home seat**
(litsenziyadagi `home_seats` ustuni, `ActiveHomeSeats`, `grantB2BMember`).

Natijada model: **maktab = litsenziya + N ta stansiya.** Odam yo'q.

Home seat (maktab tanlagan o'quvchiga uyda ham ishlaydigan VIP) narx hujjatida
alohida SKU sifatida yozilgan, lekin hech qachon ishlatilmagan. U bilan birga
`docs/b2b/school-station-pricing.md` dagi tegishli bo'lim ham yangilanadi.

## 4. Admin paneldagi yuza

Maktab sahifasida (`/admin/.../b2b/orgs/{id}`) stansiyalar bo'limi tepasida:

```
O'rnatish kaliti:  AVTO-K7M2-P9XQ        30 seatdan 4 tasi ishlatilgan
                   2027-08-05 gacha amal qiladi

Til: [uz-Latn ▾]   [ O'rnatish faylini yuklab olish ]   [ Kalitni almashtirish ]
```

### 4.1. Endpointlar

| Metod | Yo'l | Vazifa |
|---|---|---|
| `GET` | `/admin/v1/b2b/orgs/{id}/installer` | Joriy kalit holati: kod, ishlatilgan/jami seat, muddat. Kalit yo'q bo'lsa `null`. |
| `POST` | `/admin/v1/b2b/orgs/{id}/installer` | Kalit yo'q bo'lsa yaratadi, bor bo'lsa **o'shanisini qaytaradi**. Rotatsiya emas. |
| `POST` | `/admin/v1/b2b/orgs/{id}/installer/rotate` | Eski kalitni bekor qiladi va yangisini yaratadi. |
| `GET` | `/admin/v1/b2b/orgs/{id}/installer.exe?locale=uz-Latn` | Tayyor EXE'ni oqim sifatida qaytaradi. Kalit yo'q bo'lsa 409. |

Ikki marta yuklab olish **bir xil EXE** beradi — eski fayl o'lmaydi. Bu ataylab:
admin 30 PCni bir necha kunda o'rnatishi mumkin.

Kalit almashtirilganda eski EXE ishlamay qoladi, lekin **allaqachon ulangan PClar
ishlashda davom etadi** — ular kalitga emas, o'z Ed25519 kalitiga tayanadi.

### 4.2. Kalit muddati

`b2b_org_enroll_code.expires_at` — litsenziyaning eng kech `ends_at` iga tenglashadi.

Faza 1 dagi `OpenEnrollWindow` TTLni `maxEnrollTTL = 24h` bilan cheklaydi. Bu chegara
**olib tashlanadi**, chunki uni ishlatadigan yagona yo'l (`/teacher`) ham o'chirilyapti:
yangi imzo TTL emas, `expiresAt time.Time` oladi va uni to'g'ridan-to'g'ri yozadi.
`defaultEnrollTTL` va `maxEnrollTTL` konstantalari o'chiriladi.

Haqiqiy chegara TTL emas, **seat cap**. Kalit sizib chiqsa eng yomon holatda o'sha
maktabning o'z 30 seati to'ladi — va rotatsiya bir bosishda.

`max_uses` avvalgidek bo'sh seatlar soniga tenglashadi.

## 5. EXE qanday yasaladi

### 5.1. Asosiy binar

`station/` moduli **Docker image qurilishi paytida** cross-compile qilinadi va
api image ichiga qo'yiladi (`/opt/station/avtotest-station.exe`). Repo'ga binar
fayl qo'yilmaydi va serverda Go toolchain kerak emas.

Bu qaror ataylab: repo'ga qo'yilgan yoki serverga qo'lda ko'chirilgan binar manba
kod bilan ajralib ketardi — 2026-08-05 da nginx konfiguratsiyasida aynan shunday
bo'lgan (`nginx-repo-prod-divergensiyasi`).

### 5.2. Konfigni EXE oxiriga yozish

Windows PE formati fayl oxiridagi ortiqcha baytlarga chidaydi. Yuklab olishda
server tayyor binarni o'qiydi va oxiriga qo'shadi:

```
[EXE baytlari][konfig JSON][uzunlik uint32 BE (4 bayt)][magic (16 bayt)]
```

Magic: `AVTOSTATIONCFG01` (aynan 16 bayt).

Agent o'qishda oxirgi 20 baytni oladi, magicni tekshiradi, uzunlikni o'qiydi,
shuncha bayt orqaga qaytadi va JSONni parse qiladi. Skanerlash yo'q — ya'ni EXE
ichida tasodifan uchragan bir xil baytlar chalg'itmaydi.

Konfig JSON:

```json
{
  "code":     "AVTO-K7M2-P9XQ",
  "api":      "https://drivergo.uz",
  "frontend": "https://drivergo.uz",
  "org":      "avto",
  "locale":   "uz-Latn"
}
```

`api` va `frontend` server tomonidan `PUBLIC_BASE_URL` dan olinadi. Yo'l-yo'lakay
bitta bug tuzatiladi: agentning hozirgi standart qiymatlari `avtotest.uz` ga ishora
qiladi, prod esa `drivergo.uz`.

**Ogohlantirish:** kelajakda EXE Authenticode bilan imzolansa, imzo fayl oxirini
ham qamrab oladi va bu usul buziladi. Unda har maktab uchun `-ldflags -X` bilan
qayta kompilyatsiya qilishga o'tish kerak.

### 5.3. Fayl nomi

`avtotest-station-<slug>.exe`, bunda slug — maktab nomidan olingan ASCII-xavfsiz
qism. Nom kirill yoki boshqa alifboda bo'lib, ASCII qolmasa — org id ning
dastlabki 8 belgisi ishlatiladi.

## 6. Agent PCda nima qiladi

### 6.1. Birinchi ishga tushirish

1. `os.Executable()` orqali o'z faylini ochadi va oxiridan konfigni o'qiydi.
   Konfig yo'q bo'lsa — hozirgidek `-code` bayrog'iga qaytadi (qo'lda o'rnatish
   yo'li saqlanadi).
2. **O'zini ko'chiradi:** `%ProgramData%\AvtoTest\station\avtotest-station.exe`.
   U yerga yozib bo'lmasa `%LOCALAPPDATA%\AvtoTest\station\` ga tushadi va bu
   holat logga yoziladi.
3. **Avtostartga qo'shadi:** `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
   ga `AvtoTestStation` nomi bilan ko'chirilgan nusxaning to'liq yo'li yoziladi.
   HKCU tanlangan, chunki **administrator huquqi talab qilmaydi** — sinfxona PCsida
   ko'pincha oddiy foydalanuvchi hisobi ishlaydi.
4. Ichidagi kod bilan ro'yxatdan o'tadi, Ed25519 kalitini yaratadi va DPAPI bilan
   muhrlaydi (Faza 1 dagi mantiq o'zgarmaydi).
5. Kioskni ochadi.

Ko'chirilgan nusxa konfig footerini saqlaydi — ya'ni u ham o'sha maktabga bog'liq
bo'lib qoladi.

### 6.2. Keyingi ishga tushirishlar

Avtostart ishlaydi. Agent allaqachon ro'yxatdan o'tganini ko'radi (holat fayli va
kalit joyida), **qayta o'rnatmaydi va qayta ro'yxatdan o'tmaydi** — shunchaki
tokenni yangilaydi va kioskni ochadi.

Yuklab olingan asl faylni USB'dan ishlatib, keyin o'chirib tashlash mumkin.

### 6.3. O'chirish

`avtotest-station.exe -uninstall` — avtostart yozuvini, ko'chirilgan nusxani va
kalit/holat fayllarini o'chiradi. Serverdagi stansiya yozuvi tegilmaydi; uni admin
panelidan revoke qilish kerak (seat shunda bo'shaydi).

## 7. Ma'lumotlar bazasi

Migratsiya `0058_b2b_drop_membership`:

```sql
DROP TABLE b2b_invite;
DROP TABLE b2b_org_member;
ALTER TABLE b2b_org_license DROP COLUMN home_seats;
```

`b2b_org_enroll_code` sxemasi o'zgarmaydi — faqat unga yoziladigan `expires_at`
qiymati boshqacha hisoblanadi.

Down migratsiya jadvallarni qayta yaratadi, lekin **ma'lumotni tiklay olmaydi** —
buni fayl boshida aniq yozish kerak.

## 8. Kod o'zgarishlari

| Fayl | O'zgarish |
|---|---|
| `backend/internal/b2b/handlers.go` | `AuthedRoutes` dagi barcha `/me/teacher/*` va `/me/invites*` marshrutlari va handlerlari o'chiriladi |
| `backend/internal/b2b/store.go` | `teacherRole`, `requireOwner`, a'zo/taklif funksiyalari o'chiriladi |
| `backend/internal/b2b/enroll_code.go` | `AsTeacher` o'ramlar o'chiriladi; `OpenEnrollWindow` litsenziya muddatini TTL sifatida oladi |
| `backend/internal/b2b/station.go` | `ActiveHomeSeats`, `ListStationsAsTeacher`, `RevokeStationAsTeacher`, `RenameStationAsTeacher` o'chiriladi |
| `backend/internal/b2b/installer.go` | **yangi** — konfig footerini yozish, slug hisoblash |
| `backend/internal/admin/b2b_handlers.go` | a'zo/taklif/grant endpointlari o'chiriladi; installer endpointlari qo'shiladi |
| `backend/internal/admin/b2b.go` | a'zo so'rovlari o'chiriladi; a'zo sanog'iga tayangan statistika tozalanadi; litsenziya yaratish so'rovi va uning `home_seats` maydoni tozalanadi (`createB2BLicense` body, org detali, CSV eksporti — a'zo ustunlari bilan birga) |
| `backend/internal/db/migrations/0058_*` | **yangi** |
| `station/internal/embedcfg/` | **yangi** — footerni o'qish (server bilan bir xil format) |
| `station/internal/selfinstall/` | **yangi** — o'zini ko'chirish, avtostart, o'chirish (Windows/dev bo'linishi `keystore` kabi) |
| `station/cmd/avtotest-station/main.go` | konfig footeridan o'qish, `-uninstall`, standart URL'lar tuzatiladi |
| `frontend/src/app/[locale]/(app)/teacher/` | **butunlay o'chiriladi** |
| `frontend/src/app/[locale]/admin/(shell)/b2b/orgs/[id]/page.tsx` | a'zolar/taklif/grant bo'limlari o'chiriladi; installer paneli qo'shiladi |
| `frontend/messages/*.json` | `Teacher` namespace butunlay; `AdminB2B` dagi a'zo kalitlari; yangi installer kalitlari (3 til) |
| `docs/b2b/school-station-pricing.md` | home seat bo'limi olib tashlanadi, o'rnatish yo'li qayta yoziladi |

## 9. Xavfsizlik

- **Kalit — bearer sekret.** EXE ichida ochiq yotadi. Chegara TTL emas, seat cap:
  kalitni topgan odam eng ko'pi bilan o'sha maktabning bo'sh seatlarini to'ldiradi,
  va rotatsiya bir bosishda. Ulangan PClar rotatsiyadan ta'sirlanmaydi.
- **EXE maktabga bog'liq.** Boshqa maktabda ishlatilsa kod o'sha org'ga tegishli
  bo'lgani uchun stansiya o'sha maktabning seatidan yeydi — ya'ni "boshqa maktab
  uchun ishlamaydi" degani: u faqat o'z maktabini ulaydi.
- **Installer endpointlari admin autentifikatsiyasi ortida** va mavjud
  `b2b.orgs.*` ruxsatlariga bo'ysunadi.
- **Avtostart HKCU'da** — ya'ni agent oddiy foydalanuvchi huquqi bilan ishlaydi va
  tizimga chuqur o'rnashmaydi.
- Footer yozishda `code` dan boshqa maxfiy narsa yozilmaydi; server kaliti,
  DATA_ENCRYPTION_KEY va shunga o'xshash narsalar EXE'ga **hech qachon** tushmaydi.

## 10. Test rejasi

**Go unit (backend):**
- footer yozish/o'qish round-trip; magic noto'g'ri, uzunlik fayldan katta, footer
  umuman yo'q — uchalasi ham toza xato
- `POST /installer` ikki marta chaqirilsa **bir xil kod** qaytaradi
- `rotate` eski kodni bekor qiladi va yangisini beradi; eski kod bilan ro'yxatdan
  o'tish rad etiladi
- kalit muddati litsenziya oxiriga tenglashadi
- litsenziyasi yo'q org'da installer 409

**Go unit (station):**
- footerdan o'qilgan konfig `-code` bayrog'idan ustun turadi; footer yo'q bo'lsa
  bayroqqa qaytadi
- ikkinchi ishga tushirishda qayta o'rnatilmaydi

**O'chirishdan keyin:** qolgan b2b va admin testlari sinmasligi; `grep` bilan
`teacherRole|b2b_org_member|b2b_invite|home_seats` qolmaganini tasdiqlash.

**Windows'da qo'lda** (`-selftest` ga qo'shiladi): o'zini ko'chirish, avtostart
yozuvi, qayta yuklashdan keyin kiosk o'zi ochilishi, `-uninstall` hammasini
tozalashi.

## 11. Bajarilish tartibi

Bitta reja, ketma-ket:

1. Migratsiya + a'zolik/rol/home-seat mashinasini o'chirish (backend va frontend)
2. Installer kaliti: litsenziya muddatiga bog'langan TTL, admin store funksiyalari
3. Footer formati (backend yozadi, station o'qiydi — bitta formatning ikki tomoni)
4. Admin endpointlari + EXE oqimi
5. Docker image'ga station binarini qo'shish
6. Agent: footerdan o'qish, o'zini ko'chirish, avtostart, `-uninstall`
7. Admin UI paneli + uch tilda matnlar
8. Hujjatlarni yangilash
