# Reklama-05 «Sariq chiziq» — dizayn

**Sana:** 2026-10-01 · **Papka:** `output/instagram/reklama-05/` (gitignored, faqat bu hujjat git'da)

## Maqsad

Foydalanuvchi reklama-02 (diktorli, karaoke subtitrli, Dd7d_idS2w3) ni «zo'r» deb baholadi va xuddi shunday,
bitta qiyin savolni tushuntiradigan yana bir Reel so'radi. Ovoz — Muxlisa AI «Asomiddin» (speaker 1),
reklama-03/04 da tanlangan diktor.

## Savol

`avtoimtihon-143` — «Yotiq chiziqning uzluksiz sariq chizig'i nimani bildiradi?» (rasm `images/i15_3.webp`,
640×480). Foydalanuvchi uchta nomzoddan (143, 383, 1092) tanladi.

| Variant | Matn | Birinchi javoblar (prod, n=142) |
|---|---|---|
| 1 ✓ | Transport vositalarining to'xtashi taqiqlangan joyni | 44 (31%) |
| 2 | Belgilangan yo'nalishli transport vositalari uchun va taksi to'xtashiga ruxsat berilgan joyni | 66 (46%) |
| 3 | Transport vositalarining to'xtab turishi taqiqlangan joyni | 32 (23%) |

Xato ulushi 69.0% → «o'n kishidan yettitasi» (6.9 → 7, ekranda «69%» ham ko'rsatiladi). So'rov: har profilning
birinchi javobi (`DISTINCT ON (profile_id, question_id) ORDER BY answered_at`), `BEGIN READ ONLY`, faqat agregat.
Bu ≥60 birinchi urinishli savollar orasida 7-o'rin.

## Faktlar (lex.uz/docs/-5953883, 2026-10-01 da o'qildi)

- «to'xtash — transport vositasi harakatini 10 daqiqagacha bo'lgan muddatga to'xtatish»
- «to'xtab turish — … yo'lovchilarni chiqarish yoki tushirish, yuk ortish yoki tushirish bilan bog'liq bo'lmagan
  hollarda harakatni 10 daqiqadan ko'proq vaqtga atayin to'xtatish»
- 2-ilova: «1.4 — to'xtash taqiqlangan joyni bildiradi … 3.27 bilan»; «1.10 — to'xtab turish taqiqlangan joyni
  bildiradi … 3.28 bilan»; «Yotiq chiziqlar oq rangda bo'ladi (1.4, 1.10 va 1.17 chiziqlar sariq rangda bo'ladi)».
- 1.10 ning uzuq-uzuqligi: bank savoli `avtoimtihon-583` («Sariq uzuq-uzuq yotiq chiziq» → «To'xtab turish
  taqiqlangan» bilan).
- Imtihon qoidalari va katalog sonlari reklama-01 `evidence.json` dan (20 savol, 25 daqiqa, 18/20).

Soddalashtirish: «to'xtab turish — undan ko'p» deyiladi; to'liq ta'rif ekranda kichik matn bilan beriladi.
Mnemonika «uzluksiz — umuman to'xtama / uzuq — uzoq turma» qoida emas, eslab qolish vositasi sifatida beriladi.

## Diktor matni

1. Har o'n kishidan yettitasi shu savolda xato qiladi.
2. Uzluksiz sariq chiziq nimani bildiradi?
3. Javobingizni izohga yozing! · Uch! · Ikki! · Bir!
4. Eng ko'p tanlangan javob — «taksi to'xtashi mumkin». Bu xato!
5. «To'xtab turish taqiqlangan» — bu ham xato!
6. To'g'risi — to'xtash taqiqlangan.
7. To'xtash — o'n daqiqagacha. To'xtab turish — undan ko'p.
8. Uzluksiz chiziq — bir daqiqaga ham to'xtama.
9. Uzuq chiziq — uzoq turma.
10. Bitta so'z — bitta xato. Bunday uchta xato — va siz yiqildingiz.
11. Shuning uchun xatoni imtihonda emas, Drayver Go'da qiling!
12. Xato qilsangiz — sababini tushuntiradi. · Xatolaringiz aqlli takrorlashda qaytadi.
13. Diagnostika — bepul, ro'yxatdan o'tmasdan. · Drayver Go. Imtihonga tayyor holda kiring!

## Arxitektura

reklama-02 quvuri asos, reklama-04 ning ovoz qatlami ulanadi:

- `script.py` — satrlar (`[ekran](aytilishi)` sintaksisi, `talaffuz.py` reklama-04 dan).
- `voice.py` — Muxlisa speaker 1, har satr alohida; so'z vaqtlari `align.py` (Whisper large-v3 + DP), qamrov ≥ 0.7.
- `plan.py` — 100 BPM 1/8 to'rga snap, `assets/timeline.json` (reklama-02 sxemasi).
- `music.py` — reklama-02 musiqasi, yangi cue'lar, diktor ostida ducking, −14 LUFS.
- `render.py` — reklama-02 render'i: odamlar hook, savol + 3 variant, sanoq, so'rovnoma foizlari; yangi sahnalar:
  soat (0–10 / 10+ daqiqa), yo'l cheti chizig'i animatsiyasi (uzluksiz/uzuq, mashina, 3.27/3.28 belgilari),
  imtihon to'ri + TOPSHIRMADI muhri, brend, haqiqiy sayt ekranlari (reklama-01 `capture/shots`), CTA.
- `verify.py` — to'liq dekod, LUFS, PSNR (kodlangan vs render), A/V offset; `asr_check.py --mix`.
- `evidence.py` — `evidence.json` (yuqoridagi faktlar) va skript da'volarini assert qiladi.

## Sifat mezonlari

1080×1920, 30 fps, ≤ 9.5 MB, matn har kadrda xavfsiz zonada, font fallback yo'q, dam olayotgan rasmlar manba
bilan piksel-aniq, subtitr so'z vaqtiga mos, Whisper mix tekshiruvi.

## Nashr

Foydalanuvchi videoni ko'rib ruxsat bergandan keyingina Instagram'ga joylanadi (alohida qadam).
