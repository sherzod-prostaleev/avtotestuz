# B2B School Classroom — narx va shartlar

## Mahsulot

**School Classroom** = sinfxona **stansiya litsenziyasi**.

- Maktab `N` ta kompyuter (stansiya) uchun to‘laydi.
- VIP **faqat bind qilingan maktab PClarida** ishlaydi.
- Maktab VIP ni o‘quvchilarga **qayta sotmaydi** va shaxsiy login sifatida tarqatmaydi.
- Uyda mashq: o‘quvchi alohida **B2C VIP** sotib oladi (yoki free limit).
- Stansiya doim internetga ulangan bo‘lishi shart: har bir savol, rasm va javob serverdan real vaqtda keladi. Aloqa uzilsa, kiosk shu zahoti to‘xtaydi. **Offlayn rejim yo‘q** — maktabga bunday imkoniyat va’da qilinmasin.

## Sinfxonani ishga tushirish

1. Admin avtomaktab tashkilotini yaratadi va kerakli **sinfxona PC limiti** bilan litsenziya qo‘shadi.
2. Admin tashkilot sahifasida **installer key**ni ochadi va `avtotest-station-<slug>.exe` faylini yuklab oladi. Bir xil faylni istalgancha marta qayta yuklab olish mumkin — bu allaqachon o‘rnatilgan PClardagi nusxalarni bekor qilmaydi. Kalit oshkor bo‘lib qolsa, alohida **rotate** (almashtirish) tugmasi bor: u yangi yuklab olinadigan fayllarga ta’sir qiladi, lekin allaqachon ulangan PClarni buzmaydi.
3. Fayl har bir sinfxona kompyuterida **bir marta** ishga tushiriladi. Hech narsa qo‘lda kiritilmaydi — kalit faylning ichida keladi. Dastur o‘zini shu PCga o‘rnatadi, kompyuterni ishga tushirgan **aynan shu Windows hisobi** bilan birga avtomatik ishga tushishga yoziladi, maktab tashkilotiga ulanadi va kiosk oynasini ochadi. **Muhim:** PC aynan shu Windows hisobiga avtomatik kirishga (auto-login) sozlangan bo‘lishi shart — boshqa Windows hisobi bilan kirilsa kiosk o‘zi ochilmaydi.
4. Har bir keyingi safar shu Windows hisobi qayta kirganda (odatda — PC yoqilganda, agar auto-login sozlangan bo‘lsa) kiosk hech kim aralashmasdan o‘zi qayta ochiladi.
5. Kiosk **hech qanday login talab qilmaydi**. VIP o‘quvchi akkauntidan emas, maktabning litsenziyasidan keladi.

## Tavsiya etilgan paketlar (boshlang‘ich)

| Paket | Stansiyalar | Muddat | Izoh |
|-------|-------------|---------|------|
| School 20 | 20 | 6 / 12 oy | Kichik sinfxona |
| School 30 | 30 | 6 / 12 oy | Standart |
| School 50 | 50 | 6 / 12 oy | Katta maktab |

Narxlar admin tomonidan shartnomada belgilanadi (invoice / bank o‘tkazmasi). Self-serve checkout keyinchalik.

## Partner promo

Maktabga maxsus B2C promo-kod (`promo_code.partner_org_id`) — o‘quvchi o‘zi to‘laydi, maktab reseller emas.

## Qayta sotish taqiqi

Shartnoma / oferta: B2B litsenziya faqat maktab sinfxonasida foydalanish uchun. Installer faylini, installer key kodini yoki stansiya huquqini uchinchi shaxslarga sotish yoki topshirish taqiqlanadi.
