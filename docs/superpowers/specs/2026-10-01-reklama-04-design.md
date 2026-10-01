# Reklama-04 «Imtihon kuni»: hikoya-trailer (dizayn)

**Sana:** 2026-10-01 · **Holat:** foydalanuvchi talabi bilan to'g'ridan-to'g'ri qurilmoqda (so'rov rejimi o'chiq; konsepsiya chatda e'lon qilindi)

## Maqsad

reklama-03 ga **o'xshamaydigan** ~40–45 soniyalik Reel. Funksiyalar ro'yxati emas, his-tuyg'u yoyi: qo'rquv → orqaga o'rash → tayyorgarlik → g'alaba. Montaj sifati "10/10" darajasida bo'lishi kerak:
- har pardaning o'z rang gradingi;
- VHS uslubidagi orqaga o'rash;
- yurak urishiga mos zoom;
- musiqa zarbasiga tushadigan montaj kesishlari;
- to'liq ekranli kinetik tipografiya.

## Ovoz

**UzbekVoice.ai, model `shoira`.** 5 modelning ko'r Whisper sinovida eng yuqori natija shu modelda chiqdi: 0.96 (lola 0.89, sevinch 0.84, kamola 0.81, jasur 0.74). Shoira 24 kHz, jasur va kamola 16 kHz. Kalit `output/instagram/reklama-04/.env` da (gitignored), u bir martalik sinov kaliti va foydalanuvchi uni keyin almashtiradi. API: `POST https://uzbekvoice.ai/api/v1/tts`, `{text, model, blocking}`, `Authorization: <key>`. Javobda imzolangan `.wav` URL keladi.

## Ssenariy (pardalar)

| Parda | Diktor | Ekran (haqiqiy lokal UI) |
|---|---|---|
| I. Qo'rquv (sovuq ko'k-qizil grade, yurak urishi) | Imtihon kuni. 20 savol. 25 daqiqa. Birinchi savol — va siz javobni bilmaysiz. | `exam-first`: taymer 25:00, xira, yurak urishiga mos zoom |
| II. Orqaga o'rash | To'xtang. Yetti kun orqaga qaytamiz. | VHS ⏪, skanlayn, «7 KUN OLDIN» |
| III. Tayyorgarlik (iliq grade, 1→7-KUN) | 1-kun: bepul diagnostika, darajangiz 30%. Xato qilsangiz, izoh sababini tushuntiradi. Xatolar takrorlashga qaytib keladi. Belgilar. Mavzular. Imtihon mashqi. Yettinchi kun: tayyorsiz. | `diag-result` (3/10 = 30%), `ex-explain`, `mistakes`, `signs` / `practice` / `exam` musiqa zarbasida, `stats` |
| IV. G'alaba (yashil grade) | Imtihon kuni. Oxirgi savol. 19/20 — topshirdingiz! | `exam-last` (19 katak, «Xato 1/2») → to'g'ri javobga tap → `exam-pass` «Tabriklaymiz! 19/20», «TOPSHIRDI» muhri, konfetti |
| V. CTA | Driver Go. Diagnostika bepul — bugundan boshlang! | Logo, DRIVERGO.UZ, «24 soat Premium sovg'a» |

## Haqqoniylik qoidalari (reklama-03 dan meros)

- **"Har bir savolga/xatoga izoh" deyilmaydi:** 1232 izoh 1277 savolga to'g'ri keladi.
- **Imtihon qoidasi:** 18 to'g'ri javob kerak, ya'ni uchinchi xato yiqitadi.
- **19/20 sahnalashtirilgan hikoya.** Ekrandagi karta haqiqiy simulyatsiya natijasi bo'lib, unda "rasmiy imtihonni kafolatlamaydi" degan yozuv bor. Kafolat haqida da'vo qilinmaydi.
- **Sonlar manbasi:** 30% diagnostika natijasi lokal ekrandan olingan. Qolgan sonlar (20, 25, 24 soat) `evidence.json` dan olinadi va assert qilinadi.

## Ishlab chiqarish

reklama-03 quvuri qayta ishlatiladi. Unga quyidagilar qo'shiladi:
- `uzbekvoice.py` TTS mijozi;
- pardaga mos rang gradingi va VHS effekti (`fx.py`);
- yurak urishi, tasma o'rash va g'alaba "drop"i bilan yangi musiqa;
- hikoya sahnalariga moslangan `render.py`.

Tekshiruvlar reklama-03 dagi bilan bir xil: xavfsiz zona, shrift almashuvi, `verify.py`, ASR. Nashr faqat foydalanuvchining alohida ruxsati bilan.
