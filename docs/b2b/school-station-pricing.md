# B2B School Classroom — narx va shartlar

## Mahsulot

**School Classroom** = sinfxona **stansiya litsenziyasi**.

- Maktab `N` ta kompyuter (stansiya) uchun to‘laydi.
- VIP **faqat bind qilingan maktab PClarida** ishlaydi.
- Maktab VIP ni o‘quvchilarga **qayta sotmaydi** va shaxsiy login sifatida tarqatmaydi.
- Uyda mashq: o‘quvchi alohida **B2C VIP** sotib oladi (yoki free limit).

## Sinfxonani ishga tushirish va login qoidasi

1. Admin avtomaktabni yaratadi va kerakli **sinfxona PC limiti** bilan litsenziya qo‘shadi.
2. Admin yoki maktab rahbari/o‘qituvchisi har bir kompyuter uchun bir martalik **PC ulash kodi** yaratadi.
3. Kod aynan ulanadigan kompyuterda `/teacher` sahifasiga kiritiladi. Brauzerning barqaror device fingerprinti avtomaktabga bog‘lanadi.
4. Har bir o‘quvchi ulangan kompyuterda **o‘z akkaunti** bilan kiradi. VIP o‘quvchi loginidan emas, so‘rovdagi ulangan PC fingerprintidan olinadi.

PC ulash kodi login yoki parol o‘rnini bosmaydi. `/teacher` ham “hamma kompyuterda bitta parol” sahifasi emas; u kod yaratish, PC ulash va maktab a’zolarini boshqarish uchun.

### Parallel sessiyalar

Hozirgi auth modeli bir profil uchun bir nechta parallel refresh sessiyaga ruxsat beradi: yangi login yangi `refresh_token` yaratadi, avvalgi faol tokenlarni bekor qilmaydi. Demak ikkinchi qurilmadan oddiy login birinchi qurilmani avtomatik chiqarmaydi.

Xavfsizlik istisnosi bor: allaqachon aylantirilgan eski refresh-token 45 soniyalik parallel-so‘rov grace oynasidan keyin qayta ishlatilsa, tizim token o‘g‘irlangan bo‘lishi mumkin deb barcha refresh sessiyalarni bekor qiladi. Shu sabab umumiy “demo login” texnik jihatdan ishlashi mumkin bo‘lsa ham tavsiya etilmaydi: brauzer cookie nusxalash yoki eski tokenni qayta qo‘llash hammaga qayta login talab qilishi mumkin.

Operator maktabga quyidagini aytadi: **har bir o‘quvchiga alohida akkaunt bering, sinfxona PClarini esa bir martalik kod bilan ulang**. Maktab VIPi PCga bog‘lanadi; uyda foydalanish uchun alohida B2C VIP yoki quyidagi Home seat kerak.

## Tavsiya etilgan paketlar (boshlang‘ich)

| Paket | Stansiyalar | Muddat | Izoh |
|-------|-------------|---------|------|
| School 20 | 20 | 6 / 12 oy | Kichik sinfxona |
| School 30 | 30 | 6 / 12 oy | Standart |
| School 50 | 50 | 6 / 12 oy | Katta maktab |

Narxlar admin tomonidan shartnomada belgilanadi (invoice / bank o‘tkazmasi). Self-serve checkout keyinchalik.

**Home seat (ixtiyoriy SKU):** litsenziyadagi `home_seats` — named o‘quvchi VIP (uyda ham ishlaydi). Stansiyadan alohida hisoblanadi; faqat admin grant.

## Partner promo

Maktabga maxsus B2C promo-kod (`promo_code.partner_org_id`) — o‘quvchi o‘zi to‘laydi, maktab reseller emas.

## Qayta sotish taqiqi

Shartnoma / oferta: B2B litsenziya faqat maktab sinfxonasida foydalanish uchun. Login/parol yoki stansiya huquqini uchinchi shaxslarga sotish taqiqlanadi.
