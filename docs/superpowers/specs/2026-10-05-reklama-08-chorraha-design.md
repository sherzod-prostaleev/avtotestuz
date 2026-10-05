# Reklama-08 «Chorraha» — dizayn

**Sana:** 2026-10-05 · **Papka:** `output/instagram/reklama-08/` (gitignored, faqat bu hujjat git'da)

## Maqsad

Foydalanuvchi: avvalgi 7 ta reklama bitta qolip bo'lib qoldi — butunlay boshqa g'oya, matn, dizayn; birinchi
3 soniyada ushlaydigan, lekin bachkana bo'lmagan hook; ovoz — Navoiy TTS erkak ovozi (tasdiqlangan).

## G'oya

«Tungi dron kamerasi»: svetofor ham, belgi ham yo'q chorrahaga uchta mashina fara yoqib kirib keladi, kadr
«PAUSE» bilan to'xtaydi — «Svetofor yo'q. Belgi yo'q. Kim birinchi o'tadi?». Javob rasmning o'zida yechiladi:
har mashina o'ng tomonidagi yo'lni «skaner» qiladi, yashilning o'ngga burilish yo'li sariqning yo'li bilan
kesishmasligi chiziladi, to'g'ri tartib qayta ijro (REPLAY) qilinadi. HUD tipografiyasi (Archivo condensed +
JetBrains Mono), cyan/amber/qizil urg'u; qog'oz ham, kartochka-so'rovnoma qolipi ham yo'q.

## Savol va faktlar

`avtoimtihon-274` (14-bilet 14-savol, rasm i28_4): to'g'ri javob 3 — «Ko'k yashil bilan, sariq». Prod birinchi
javoblar (BEGIN READ ONLY, 2026-10-05, n=130): 62 / 13 / 55 → 48% / 10% / 42%; 1-variant yashilni ikkinchi
navbatga qo'yadi — tuzoq. YHQ 105-band (o'ngdan yaqinlashayotganga yo'l berish) + atamalar: «yo'l berish —
… harakat yo'nalishi yoki tezligini o'zgartirishga majbur etishi mumkin bo'lgan hollarda …». `evidence.py`
iqtiboslarni lex.uz nusxasida, sonlarni va bilet raqamini bankda tekshiradi. Geometriya hisoblanadi: yashil
yo'li (NW burchak atrofida r=60 chorak aylana) sariq yo'liga (y=720) hech qayerda tegmaydi.

## Quvur

Navoiy erkak («E»: cross_lingual_prompt, instruct2 calm) — har satrga 3–5 seed, `voice.py` Whisper bilan eng
yaxshisini tanlaydi; `plan.py` `pause_after` (3 s o'ylash taymeri); yangi kinematografik A-minor partitura
(tape-stop, skaner, rewind effektlari); `render.py` — shahar fonini bir marta oldindan chizadi, kamera
kalitlari, ekran-maydon yorliqlari faqat xavfsiz zonada. Preview'lar har renderda tozalanadi (eski vaqt
jadvalidagi kadrlar verify'ni yolg'on yiqitgan edi).

## Sifat

1080×1920, 30 fps, 70.8 s, ≤ 9.5 MB; 2124 kadr xavfsiz zona/font/himoyalangan ekran tekshiruvi; verify PASS
(22 kadr PSNR ≥ 36.9 dB); Whisper mix 0.81–0.99.

## Nashr

Foydalanuvchi ko'rib ruxsat bergandan keyingina.
