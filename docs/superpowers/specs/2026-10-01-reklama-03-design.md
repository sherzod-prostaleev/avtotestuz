# Reklama-03: "O'quvchi yo'li" — diktorli mahsulot reklamasi (dizayn)

**Sana:** 2026-10-01 · **Holat:** dizayn tasdiqlangan, bajarish rejasi kutilmoqda

## Maqsad

Instagram Reels uchun ~50–53 soniyalik reklama. U Driver Go'ning **haqiqiy** funksiyalarini real UI ekranlarida, bitta o'quvchining tayyorgarlik yo'li sifatida ko'rsatadi. Maqsad 1122.mp4 (avtoshablonlar.uz) raqobatchi reklamasi darajasiga yetish yoki undan o'tish. O'sha reklamaning zaifliklari takrorlanmaydi:
- funksiyalar quruq ro'yxat qilib sanalmaydi;
- bir gap ekranda ikki marta yozilmaydi;
- subtitr Reels interfeysi ostida qolmaydi;
- aniq taklif bor.

**Muvaffaqiyat mezoni:**
- foydalanuvchi videoni "super" deb qabul qiladi;
- har bir da'vo manbali (`evidence.json`);
- talaffuzda inglizcha nomlar va raqamlar buzilmagan.

## Cheklovlar (qat'iy)

1. **Faqat mavjud funksiyalar.** Quyidagilar reklamada ko'rsatilmaydi ham, aytilmaydi ham: AI chat, video darslar, ovozli izoh, App Store / Google Play ilovasi, referal dasturi.
2. **Har bir raqam manbali.** Savollar soni, biletlar soni, belgilar soni (285), tillar soni (3), imtihon qoidasi (20/25/3) va sinov muddati (24 soat) `evidence.json` ga yoziladi. Manba prod DB (faqat `BEGIN READ ONLY`, umumiy sonlar) yoki kod qatori. `verify.py` ularni assert qiladi.
3. **Prod'ga tegilmaydi.** Ekranlar lokal stekdan va demo akkauntdan olinadi.
4. **Nashr qilinmaydi.** Instagram'ga chiqarish faqat foydalanuvchining alohida ruxsati bilan.

## Diktor matni va sahnalar

Vaqtlar taxminiy. Yakuniy vaqt diktor satrlarining haqiqiy uzunligidan `plan.py` da hisoblanadi.

Ovoz: **Muxlisa AI, Asomiddin** (`speaker: 1`). Ko'r sinovda tanlangan (`output/tts-sinov`, B: tabiiylik 5, aksent 4, reklama 5). Eng yaxshi variant ASCII tutuq belgisi + talaffuz lug'ati (`T3-talaffuz-ascii`) bo'ldi.

| # | ~Vaqt | Diktor (ekrandagi subtitr) | Tepadagi belgi | Haqiqiy UI |
|---|---|---|---|---|
| 1 | 0–4 | Haydovchilik imtihoni: yigirma savol, yigirma besh daqiqa. Uchinchi xato — va siz yiqildingiz. | IMTIHON QOIDASI | 20 katak va taymer 25:00; "uchinchi xato" so'zida 3 katak qizaradi, "yiqildingiz" da «TOPSHIRMADI» muhri |
| 2 | 4–7 | Drayver Go sizni shu imtihonga qadam-baqadam tayyorlaydi. | — | Logo nur bilan ochiladi, telefon ichida `/dashboard` |
| 3 | 7–12 | Avval bepul diagnostika: ro'yxatdan o'tmasdan darajangizni bilib olasiz. | BEPUL DIAGNOSTIKA | `/diagnostic`: savol → natija foizi |
| 4 | 12–18 | Xato qildingizmi? Izoh sababini yo'l harakati qoidalari asosida tushuntiradi. | YHQ IZOHI | Sessiyada xato javob qizaradi → izoh paneli, YHQ bandi yonadi |
| 5 | 18–23 | Xatolaringiz yo'qolmaydi: aqlli takrorlash ularni aynan kerakli paytda qaytaradi. | AQLLI TAKRORLASH | `/mistakes`: "N savol hozir takrorlashga tayyor" |
| 6 | 23–27 | Mavzular bo'yicha mashq, ikki yuz sakson beshta yo'l belgisi, uch tilda. | MASHQ · BELGILAR | `/practice` → `/signs` → til almashinuvi |
| 7 | 27–33 | Imtihon simulyatsiyasi xuddi haqiqiysidek, statistika esa o'tish ehtimolingizni ko'rsatadi. | IMTIHON · STATISTIKA | Imtihon runneri taymer bilan → `/stats` o'tish ehtimoli sanab o'sadi |
| 8 | 33–38 | Arenada boshqa o'quvchilar bilan bellashing va reytingda yuqoriga chiqing. | BATTLE ARENA | `/arena` 1v1 duel → `/leaderboard` |
| 9 | 38–44 | Savollar bazasi 2026-yil uchun yangilangan: imtihonga qo'shilgan yangi savollar ham shu yerda. | 2026 BAZA | `/tickets` to'ri, oxirgi biletlar yonadi |
| 10 | 44–50 | Diagnostika bepul. Ro'yxatdan o'tsangiz, yigirma to'rt soat to'liq Premium sovg'a! | BEPUL + 24 SOAT | "BEPUL" va "24 SOAT PREMIUM" kartasi, sovg'a animatsiyasi |
| 11 | 50–53 | Drayver Go. Imtihonga tayyor holda kiring! | — | Logo, DRIVERGO.UZ tugmasi, "Havola — profilda" |

**Diktor matni ≠ ekran matni.** Diktorga quyidagicha beriladi:
- ASCII `'`;
- "Drayver Go";
- raqamlar so'z bilan.

Ekranda esa to'g'ri `ʻ`, "Driver Go" va raqamlar ko'rinadi. Har bir satr uchun ikkala shakl ham `script.json` da saqlanadi.

**Yasashdan oldin tekshirilgan narsalar (2026-10-01):**
- **(a) 24 soatlik sinov Arena'ni ochadi.** `SignupTrialDuration` (24 soat, `backend/internal/auth/service.go:778`) oddiy `entitlement` qatorini yozadi. Arena esa `billing.Service.Status` → `ActiveEntitlementEnd` orqali tekshiradi va manbani ajratmaydi (`backend/internal/billing/entitlement.go:78`, `backend/internal/arena/service.go:257`). Shuning uchun 10-sahnadagi "to'liq Premium" iborasi to'g'ri.
- **(b) 2026-yilda yangi savollar qo'shilgan.** Seed tarixida `c1bf731` (2026-08-30), `f3e1e2b` (2026-09-03), `b611661` (2026-09-06) va `a9df26c` (2026-09-17) bor. Bazada 1277 savol va 64 bilet, eng katta ID — avtoimtihon-1281. 9-sahna da'vosi shu commitlar bilan asoslanadi.
- **(c) Imtihon qoidasi tuzatildi.** Rasmiy qoida: 20 savoldan kamida 18 tasi to'g'ri bo'lsa — TOPSHIRDI (yim.uz, `reklama-01/evidence.json` → `official_exam_rules`). Demak 2 ta xatoga ruxsat bor va uchinchisi yiqitadi. 1-sahna dastlab "faqat uchta xato" deb yozilgan edi, bu noto'g'ri. U "Uchinchi xato — va siz yiqildingiz" ga almashtirildi.
- **(d) "Har bir savolga izoh" deyilmaydi.** Seed'da 1277 savol va 1232 ta izoh bor (`backend/seed/avtoimtihon/data.json`). Shuning uchun 4-sahna umumiy da'vosiz yozildi: "Izoh sababini … tushuntiradi".

**Yakuniy matn (2026-10-01):** Muxlisa sekin gapirgani uchun video 63 s chiqdi. Foydalanuvchi qarori bilan 6 ta satr qisqartirildi va video 54.6 s bo'ldi. Amaldagi matn — `output/instagram/reklama-03/senariy.md` (`script.py`). Yuqoridagi jadval dastlabki loyiha sifatida qoldirildi.

## Vizual uslub

- **Kompozitsiya** (1080×1920):
  - tepada kalit so'z belgisi (2–3 so'z), y ≈ 180–260;
  - markazda telefon ramkasi ichida haqiqiy ekran, y 280–1040;
  - pastda karaoke subtitr bandi, y ≈ 1050–1225.

  Bir gap ikki marta yozilmaydi.
- **Harakat:** bosish to'lqini, kerakli elementga zoom, raqam sanog'i (o'tish ehtimoli 0 → yakuniy qiymat), sahnalar orasida 6–10 kadrli tez o'tish. Eski sarlavha yangisidan oldin yo'qoladi.
- **Brend:** reklama-02 dagi Driver Go palitrasi (emerald / amber), Baloo 2 ExtraBold subtitr.

## Ishlab chiqarish

reklama-02 retsepti (`output/instagram/reklama-02/`) yangi papkada qayta ishlatiladi: `output/instagram/reklama-03/`.

1. **`script.json`**: sahnalar. Har sahnada: `id`, `speech` (diktorga), `display` (ekranga), `badge`, `capture` (qaysi ekran yoki holat).
2. **`talaffuz.py`**: talaffuz lug'ati, `display` → `speech` qoidalari (iPhone→ayfon, Driver Go→Drayver Go, `ʻ`→`'`, raqam→so'z). Unit testlari bor.
3. **`capture/`**: lokal stek (`reklama-01/capture/stack.sh`) va demo akkaunt. Playwright 390×844, `deviceScaleFactor: 3`. Har holat alohida PNG. Taymer va foizlar kabi dinamik qiymatlar kompozitorda animatsiya qilinadi.
4. **`voice.py`**: Muxlisa API (`output/tts-sinov/muxlisa_gen.py` dagi `synthesize` qayta ishlatiladi; kalit `.env` dan olinadi, faylga yozilmaydi). Har satr alohida `.wav`, 48 kHz ga resample, 80 Hz HPF, yengil kompressiya.
5. **`align.py`**: karaoke so'z vaqtlari. Muxlisa vaqt bermaydi. faster-whisper `word_timestamps=True` natijasi `fold()` qilingan ssenariy so'zlari bilan DP tekislanadi. Tekislanmagan so'zlarga vaqt ichki oraliqda harf soniga proporsional taqsimlanadi. Har satrda qamrov foizi chiqariladi.
6. **`plan.py`, `music.py`, `render.py`**: reklama-02 dan. Sahnalar satrlarga bog'lanadi, ovoz ostida musiqa pasayadi.
7. **Chiqish:**
   - master: 1080×1920, 30 fps, −14 LUFS, TP ≤ −1.5 dBTP;
   - yuklash nusxasi: ≤10 MB, 2-pass.

## Tekshiruv (`verify.py`)

- Video to'liq, xatosiz dekodlanadi; davomiyligi 45–55 s.
- Har kadrda matn xavfsiz zonada; Pango fallback yo'q (shrift oilasi assert qilinadi).
- Subtitr so'z vaqti tekislangan so'z bilan kadr aniqligida mos; har satrda tekislash qamrovi ≥ 70%, aks holda xato.
- ASR: Whisper large-v3 har satrni qayta taniydi. "ayfon/Drayver" kabi lug'at so'zlari to'g'ri eshitiladi. O'xshashlik ko'rsatkichi faqat buzilishni topish uchun, hal qiluvchi baho foydalanuvchining qulog'i.
- `evidence.json` dagi har bir raqam render qilingan matnda aynan shu qiymat bilan uchraydi.
- `talaffuz.py` unit testlari o'tadi.

## Xavflar

- **Karaoke tekislash.** Whisper'ning o'zbekchasi zaif. Qamrov past bo'lsa, o'sha satr uchun proporsional fallback ishlaydi va hisobotda belgilanadi.
- **Muxlisa ovozi ba'zi so'zlarda qoqiladi.** Satr qayta generatsiya qilinadi yoki so'z lug'atda boshqacha yoziladi. Taxminiy sarf ~2000 belgi.
- **Demo akkauntda ma'lumot yo'q** (xatolar banki, statistika, reyting). Holat lokal DB'da demo profil uchun yaratiladi, prod'ga tegilmaydi.

## Ko'lamdan tashqari

Instagram'ga nashr, pullik reklama (boost), ruscha yoki kirillcha versiya, Remotion'ga ko'chish.
