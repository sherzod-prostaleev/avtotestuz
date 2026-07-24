# M2-11 — Mehmon-Demo Landing Funnel (Dizayn / Spec)

Sana: 2026-07-24 · Milestone: M2 · Plan: M2-11 · Qatlam: frontend

## 1. Maqsad
Mehmon foydalanuvchilar (ro'yxatdan o'tmagan) uchun landing sahifasida interactive demo tajribasini kuchaytirish va conversion voronkasini (signup funnel) yaxshilash.

## 2. Frontend O'zgarishlari (`DemoQuestionBlock`)

### 2.1. Demo Savol Natijasi va Conversion Karta
Javob berilgach (`grade` olingach):
1. **Natija Paneli**:
   - To'g'ri bo'lsa: "Ajoyib! Imtihonda xuddi shunday savollar tushadi. Barcha 1231 ta savolni yechish uchun bepul ro'yxatdan o me o'ting!"
   - Noto'g'ri bo'lsa: "Qayg'urmang! DriveGo aqlli FSRS tizimi xatolaringizni saqlaydi va unutish arafasida qaytaradi."
2. **Call-To-Action (CTA)**:
   - "Ro'yxatdan o'tish (Bepul)" yashil/oltin urg'uli tugma → `/${locale}/login` sahifasiga yo'naltiradi.
   - "Yana bitta savol sinab ko'rish" tugmasi → yangi demo savolni yuklaydi (`GET /demo/question`).

### 2.2. i18n Matnlari
Landing va Demo bo'limlariga har 3 tilda (uz-Latn, uz-Cyrl, ru) yangi matnlar qo'shiladi.

## 3. Testlash Rejasi
- Demo javob berilganda CTA kartasi va 2 ta tugma ko'rinishi testlari.
- "Yana bitta savol" bosilganda yangi savol yuklanishi va status reset bo'lishi testlari.
