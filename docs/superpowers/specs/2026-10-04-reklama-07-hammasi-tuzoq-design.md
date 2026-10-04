# Reklama-07 «Hammasi — tuzoq» — dizayn

**Sana:** 2026-10-04 · **Papka:** `output/instagram/reklama-07/` (gitignored, faqat bu hujjat git'da)

## Maqsad

Foydalanuvchi: avvalgi shablonga o'xshamagan, zamonaviy dizaynli, yangi savolni juda tushunarli o'rgatadigan va
oxirida Driver Go'ni reklama qiladigan Reel; xato qilish mumkin emas. reklama-01…06 bir xil qora shablonda edi.

## Yangi uslub (qog'oz / neo-brutalizm)

2026 motion trendlari (neo-brutalizm, kinetik tipografiya, ovozsiz ko'rishda ham tushunarli matn, 60 s dan uzun
Reels'da retention pasayishi) asosida:

- Och qog'oz fon `#F2EDE3` + siljuvchi nuqta to'r + don; qalin qora kontur (6 px), qattiq ofset soya, stiker ranglari
  (amber, ko'k, qizil, yashil, pushti); shriftlar Unbounded (sarlavha) + Space Grotesk (matn) — ikkalasida ham `ʻ`.
- Subtitr: konturli band o'rniga katta qora so'zlar, faol so'z ortida qiya marker bloki.
- Sahna almashuvi: kattalashib kirish / kichrayib chiqish + diagonal qora-amber chiziq; brendda amber ekran supurgisi.
- Musiqa: C-major pop (C–G–Am–F), marimba arpejio, shaker, qarsak; har vizual zarbaga ovoz effekti.

## Savol

`avtoimtihon-187` «Qayrilib olish taqiqlanadi:» (rasm yo'q), Driver Go 10-bilet 7-savol. Prod, har profilning
birinchi javobi, `BEGIN READ ONLY`, 2026-10-04, n=137: 1% / 7% / **31% (to'g'ri: piyodalar o'tish joylarida)** /
**62% «Hamma sanab o'tilgan hollarda»**. Xato 69.3% — ≥60 urinishli savollar orasida 6-o'rin. 1092 rad etildi:
4.1.1 belgisi qoidasi «to'g'riga va chapga» javobini aniq asoslamaydi.

## Fakt (lex.uz/docs/-5953883, 9-bob 62-band)

«Quyidagi joylarda qayrilib olish taqiqlanadi: piyodalarning oʻtish joylarida; tunnellarda; koʻpriklar, yoʻl
oʻtkazgichlar, estakadalar va ularning ostida (…belgilar bilan ruxsat berilgan qismlar bundan mustasno); temir yoʻl
kesishmalarida; yoʻlning koʻrinishi biror-bir yoʻnalishda 100 metrdan kam boʻlgan joylarda.» `evidence.py` aynan shu 5
bandni ajratib oladi va «15» hamda «bir tomonlama» 62-bandda yo'qligini assert qiladi. «Hammasi» varianti esa faqat
har biri to'g'ri bo'lsa to'g'ri — o'quv maslahati, qoida emas.

## Ssenariy (63.6 s)

hook (100 kishilik to'r, 62 tasi qizil, «TUZOQ!» muhri) → savol + 4 variant → «Siz-chi?» → 3-2-1 → so'rovnoma barlari,
«Hammasi»ga «TUZOQ!» → to'g'ri javob → zebra ustida qayrilish animatsiyasi → 62-bandning 5 joyi → F1/F2 chizib
tashlanadi («RO'YXATDA YO'Q!») → «Eslab qoling» varag'i: F1 ✕ F2 ✕ F3 ✓ ⇒ F4 «Hammasi» = XATO → brend → shu savolning
haqiqiy izoh ekrani (lokal stek, 62-band belgilangan) → xatolar banki → CTA (Diagnostika bepul, drivergo.uz).

8 satr reklama-06 yozuvlari (matn so'zma-so'z bir xil, test tekshiradi). Zaif satrlar uchun `takes.py` 3 take
sintez qilib, Whisper eng yaxshi eshitganini tanlaydi.

## Sifat mezonlari

1080×1920, 30 fps, ≤ 9.5 MB; har kadrda matn xavfsiz zonada, font fallback yo'q, dam olayotgan skrinshot piksel-piksel
manbaga teng (`check_frames.py` — 1908 kadr parallel); `verify.py` PASS (PSNR ≥ 35 dB 25 kadrning hammasida — avval
nom yaxlitlanishi sababli ko'p kadr solishtirilmay qolardi, tuzatildi); Whisper mix tekshiruvi; 30 test.

## Nashr

Foydalanuvchi videoni ko'rib ruxsat bergandan keyingina Instagram'ga joylanadi.
