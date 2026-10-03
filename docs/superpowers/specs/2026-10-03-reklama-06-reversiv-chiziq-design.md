# Reklama-06 «Reversiv chiziq» — dizayn

**Sana:** 2026-10-03 · **Papka:** `output/instagram/reklama-06/` (gitignored, faqat bu hujjat git'da)

## Maqsad

reklama-05 formatida (diktorli, karaoke subtitrli, Muxlisa AI «Asomiddin») eng qiyin savollardan birini chiroyli
tushuntiradigan va oxirida Driver Go'ni reklama qiladigan Reel. Foydalanuvchi nomzodlardan (383, 1092, 1003, 328)
383 ni tanladi.

## Savol

`avtoimtihon-383` — «Reversiv svetoforlar bo'lmaganda yoki ular o'chirib qo'yilganda ikkitali uzuq uzuq chiziqni
kesib o'tishga ruxsat etiladimi?» (rasm `images/i39_3.webp`, 1.9 chizig'i).

| Variant | Matn | Birinchi javoblar (prod, n=110) |
|---|---|---|
| 1 | Agar u haydovchining chap tomonida bo'lsa, ruxsat etiladi | 6 (5%) |
| 2 ✓ | Agar u haydovchining o'ng tomonida bo'lsa, ruxsat etiladi | 36 (33%) |
| 3 | Istalgan tomondan ruxsat etiladi | 16 (15%) |
| 4 | Taqiqlanadi | 52 (47%) |

Xato ulushi 67.3% → «uch kishidan ikkitasi». So'rov reklama-05 bilan bir xil (har profilning birinchi javobi,
`BEGIN READ ONLY`, faqat agregat), 2026-10-03 da o'qildi.

## Faktlar (lex.uz/docs/-5953883, reklama-05 `assets/lex-5953883.html` nusxasi)

- «1.9 — reversiv harakat tashkil etilgan boʻlaklarning chegarasini belgilaydi; … (reversiv svetofori oʻchirilgan
  holatda) qarama-qarshi yoʻnalishdagi transport oqimini ajratadi»
- «Reversiv svetoforlar boʻlmaganda yoki ular oʻchirib qoʻyilganda, 1.9 chizigʻini faqat u haydovchining oʻng
  tomonida boʻlsa, bosib oʻtishga ruxsat etiladi.»
- «Reversiv svetofor oʻchirilganda, haydovchilar darhol 1.9 chizigʻidan oʻngga qayta tizilishlari kerak.»
- «Reversiv svetofor oʻchirilgan boʻlsa, qarama-qarshi yoʻnalishdagi transport oqimlarini ajratuvchi 1.9 chizigʻini
  bosib oʻtish taqiqlanadi.»
- «Ikki tomoni 1.9 yoʻl chizigʻi bilan belgilangan tasma ustiga oʻrnatilgan va ishoralari oʻchirilgan reversiv
  svetofor shu tasmaga kirishni taqiqlaydi.»
- Imtihon qoidalari (20 savol, 18/20) reklama-01 `evidence.json` dan.

Mnemonika «o'ngdagi chiziq — chiqish yo'li, chapdagisi — taqiq» qoida emas, eslab qolish vositasi sifatida beriladi.

## Diktor matni

1. Uch kishidan ikkitasi shu savolda xato qiladi.
2. Reversiv svetofor o'chiq. Ikkitali uzuq chiziqni kesib o'tish mumkinmi?
3. Javobingizni izohga yozing! · Uch! · Ikki! · Bir!
4. Eng ko'p tanlangan javob — «Taqiqlanadi». Bu xato!
5. To'g'ri javob — chiziq o'ng tomoningizda bo'lsa, mumkin.
6. Bu chiziq — reversiv tasmaning chegarasi. Svetofor bu tasmada yo'nalishni almashtirib turadi.
7. Svetofor o'chsa — bu tasmaga kirish taqiqlanadi.
8. Siz shu tasmada qolsangiz, chiziq o'ng tomoningizda. Darhol o'ngga o'ting — bu mumkin!
9. Chiziq chap tomoningizda bo'lsa — kesib o'tmang: u yoqqa kirish taqiqlangan.
10. Eslab qoling: o'ngdagi chiziq — chiqish yo'li, chapdagisi — taqiq.
11. Bitta so'z — bitta xato. Bunday uchta xato — va siz yiqildingiz.
12–13. reklama-05 CTA (xatoni imtihonda emas, Drayver Go'da qiling; tushuntirish; aqlli takrorlash; diagnostika
    bepul; Drayver Go).

11–13 satrlar va sanoq satrlari reklama-05 dagi bilan so'zma-so'z bir xil, ularning yozuvlari qayta ishlatiladi.
Birinchi yig'ishda video 79 s chiqdi; uzunlik va 10 MB yuklash limiti uchun reklama-05 dagi «Imtihon esa xuddi
haqiqiysidek» satri (va imtihon ekrani) olib tashlandi, sahnalar orasidagi pauza 0.5 → 0.4 s — natija 74.4 s.

## Arxitektura

reklama-05 quvuri nusxalanadi (`script.py`, `voice.py`, `align.py`, `plan.py`, `music.py`, `render.py`, `fx.py`,
`verify.py`, `asr_check.py`, `evidence.py`, testlar). O'zgaradigan qismlar:

- `script.py`/`evidence.py` — yangi matn va faktlar.
- `render.py` — reklama-05 ning sariq chiziq/soat sahnalari o'rniga: tepadan ko'rilgan yo'l, o'rtada 1.9 bilan
  ajratilgan reversiv tasma, ustida reversiv svetofor (yashil ↓ → qizil ✕ → o'chiq); mashina chiziq o'ngda bo'lganda
  o'ngga o'tadi (✓), chiziq chapda bo'lganda kesib o'tish ✕ va qarshidan oqim. Qolgan sahnalar (hook, savol + 4
  variant, sanoq, so'rovnoma, imtihon to'ri, brend, sayt ekranlari, CTA) qayta ishlatiladi.
- `capture/capture06.mjs` — shu savolning izoh ekrani lokal stekdan.

## Sifat mezonlari

1080×1920, 30 fps, ≤ 9.5 MB, matn xavfsiz zonada, font fallback yo'q, subtitr so'z vaqtiga mos, Whisper mix
tekshiruvi, `verify.py` PASS, testlar o'tadi.

## Nashr

Foydalanuvchi videoni ko'rib ruxsat bergandan keyingina Instagram'ga joylanadi.
