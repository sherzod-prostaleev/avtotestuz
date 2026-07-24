# M2-04 — To'lov tarixi endpointi (Dizayn/Spec)

Sana: 2026-07-24 · Milestone: M2 · Plan: M2-04 · Qatlam: backend

## Maqsad
Foydalanuvchi o'z to'lov tarixini ko'ra oladigan `GET /api/v1/me/payments` endpointi. Roadmap'dagi M2-04'ning "grant" qismi (T1: to'lov `paid` bo'lganda entitlement berish) allaqachon M2-02/M2-03'ning o'zida (`billing.Service.GrantDays`, Payme/Click webhook'larida) amalga oshirilgan — bu spec faqat qolgan **read-side tarix** qismini (roadmap T2) qamrab oladi.

## Qamrov qarori
Endpoint **barcha statusdagi** to'lovlarni qaytaradi (`created`, `pending`, `paid`, `failed`, `canceled`, `refunded`) — faqat `paid`larni emas. Sabab: bu "tarix" (nima bo'lganini ko'rsatish), faqat "chek" (muvaffaqiyatli to'lovlar ro'yxati) emas — foydalanuvchi muvaffaqiyatsiz/bekor bo'lgan urinishini ham ko'rishi kerak (masalan "nega mening to'lovim o'tmadi" degan savolga javob). Frontend keyinchalik statusga qarab har xil render qilishi mumkin (bu M2-10'ning ishi, bu spec faqat backend API).

## Endpoint
`GET /api/v1/me/payments?limit=N` (JWT talab qilinadi)

- `limit`: ixtiyoriy query-param, butun son. `<=0` yoki berilmasa → default `20` (session tarixi bilan bir xil konvensiya, `internal/session/handlers.go`ning `listMySessions`iga qarang). Butun son bo'lmasa → `400 invalid_request`.
- Natija: profilning to'lovlari, `created_at DESC` bo'yicha saralangan, `limit`gacha.
- Auth yo'q bo'lsa → `401 unauthorized` (mavjud `/me/*` konvensiyasi).

**Javob shakli** (massiv, `httpx.Data`):
```json
[
  {
    "id": "<uuid>",
    "tariff_code": "gentra",
    "tariff_name": "Gentra",
    "tariff_days": 30,
    "amount_uzs": 59900,
    "provider": "payme",
    "status": "paid",
    "created_at": "2026-07-24T10:00:00Z",
    "paid_at": "2026-07-24T10:02:15Z"
  }
]
```
`tariff_name` — so'rov qilingan `locale`ga qarab lokalizatsiya qilinadi (i18n.Parse, mavjud `/tariffs` endpointi bilan bir xil konvensiya), topilmasa `uz-Latn`ga, u ham bo'lmasa `tariff.code`ga tushadi (`ListActiveTariffs`ning fallback naqshi). `paid_at` — `null` bo'lishi mumkin (hali to'lanmagan/bekor qilingan holatlar uchun).

## Ma'lumotlar manbai
Yangi jadval/migratsiya KERAK EMAS — `payment`/`tariff`/`tariff_translation` allaqachon mavjud (M2-01/M2-02/M2-03). Bitta yangi sqlc so'rov:

```sql
-- name: ListMyPayments :many
SELECT p.id, p.amount_uzs, p.provider, p.status, p.created_at, p.paid_at,
       t.code AS tariff_code, t.days AS tariff_days,
       COALESCE(tr.name, ftr.name, t.code) AS tariff_name
FROM payment p
JOIN tariff t ON t.id = p.tariff_id
LEFT JOIN tariff_translation tr ON tr.tariff_id = t.id AND tr.locale = $2
LEFT JOIN tariff_translation ftr ON ftr.tariff_id = t.id AND ftr.locale = 'uz-Latn'
WHERE p.profile_id = $1
ORDER BY p.created_at DESC
LIMIT $3;
```

## Kod joylari
- `internal/db/queries/billing.sql` — `ListMyPayments` qo'shiladi.
- `internal/account/handlers.go` — mavjud `Handler`ga (allaqachon `/me/*` marshrutlarini boshqaradi) `r.Get("/me/payments", h.listMyPayments)` qo'shiladi (`Routes` metodiga, `AuthedRoutes` emas — `account.Handler`ning o'zi allaqachon `server.go`da `auth.Required` bilan o'raladi, `getMe`/`patchMe`/`getEntitlement` bilan bir xil).

## Testlar
- Locale-fallback (so'ralgan til → uz-Latn → code) — `ListActiveTariffs`ning test naqshi bilan bir xil.
- `limit` default (20) va noto'g'ri qiymat (400) — `listMySessions`ning test naqshi bilan bir xil.
- Faqat so'ragan profilning to'lovlari qaytishi (boshqa profil ko'rinmasligi).
- Har xil status (paid/failed/canceled) hammasi qaytishi, `created_at DESC` tartib.
- Auth yo'q → 401.

## Scope tashqarisi
Frontend UI (M2-10), promo-kod ma'lumoti chekda (promo hali M2-05'da yo'q), PDF/chop etiladigan chek generatsiyasi (agar kerak bo'lsa keyinroq).
