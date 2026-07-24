# M2-01 — Tarif modeli + narx logikasi (Dizayn/Spec)

Sana: 2026-07-24 · Milestone: M2 (Monetizatsiya) · Plan: M2-01 · Qatlam: backend (read-side + seed)

## Maqsad
Foydalanuvchiga ko'rsatiladigan **muddatli VIP-pass tariflarini** DB'dan lokalizatsiya bilan qaytaruvchi API va boshlang'ich seed. Bu M2 ning poydevori — checkout (M2-09) va to'lov (M2-02/03) shu tariflarga tayanadi.

**Model:** hamma tarif bir xil premium/VIP'ni ochadi, faqat **muddat + narx** farq qiladi (feature-gating YO'Q; joriy binary free/VIP entitlement modeliga mos).

## Sxema (ALLAQACHON MAVJUD — 0003_billing)
- `tariff(id, code UNIQUE, days>0, price_uzs>=0, old_price_uzs?, badge?, sort_order, active)`
- `tariff_translation(tariff_id, locale, name, description)`

Yangi migratsiya faqat **seed** qiladi, sxema o'zgarmaydi.

## Ma'lumot (seed — 3 pullik tarif)

| code | days | price_uzs | old_price_uzs | badge | sort_order |
|------|------|-----------|---------------|-------|-----------|
| `nexia` | 7 | 24900 | 34900 | `NULL` | 1 |
| `gentra` | 30 | 59900 | 99900 | `popular` | 2 |
| `malibu` | 75 | 109900 | 199900 | `best_value` | 3 |

**Tarjimalar** (name = mashina brendi, barcha tilda bir xil atoqli ot; description lokalizatsiya):

| code | name | uz-Latn desc | uz-Cyrl desc | ru desc |
|------|------|--------------|--------------|---------|
| nexia | Nexia | 1 haftalik to'liq kirish | 1 ҳафталик тўлиқ кириш | Полный доступ на 1 неделю |
| gentra | Gentra | 1 oylik to'liq kirish | 1 ойлик тўлиқ кириш | Полный доступ на 1 месяц |
| malibu | Malibu | 2,5 oylik to'liq kirish | 2,5 ойлик тўлиқ кириш | Полный доступ на 2,5 месяца |

**Matiz (bepul)** — DB'da EMAS (`days>0` cheklovi). Frontend "hozirgi bepul rejim" marketing-kartasini o'zi qo'shadi (M2-08 doirasida). Backend faqat pullik tariflarni biladi.

**`badge`** = kalit (`popular`/`best_value`/NULL); ko'rsatiladigan matn (Eng ommabop/Eng foydali) — frontend i18n. Backend badge kalitini qaytaradi, tarjima qilmaydi.

## API kontrakt
`GET /api/v1/tariffs?locale=<uz-Latn|uz-Cyrl|ru>`

- Faqat `active=true`, `sort_order` ASC.
- Lokalizatsiya: so'ralgan locale, topilmasa **uz-Latn** ga fallback (content API bilan bir xil pattern).
- Auth talab qilinmaydi (public — checkout'dan oldin ham ko'rinadi).

Javob (`httpx.Data` bilan `{"data": [...]}`):
```json
{ "data": [
  { "code": "nexia", "days": 7,  "price_uzs": 24900,  "old_price_uzs": 34900,
    "price_per_day_uzs": 3557, "discount_percent": 29, "badge": null,
    "name": "Nexia", "description": "1 haftalik to'liq kirish" },
  { "code": "gentra", "days": 30, "price_uzs": 59900, "old_price_uzs": 99900,
    "price_per_day_uzs": 1997, "discount_percent": 40, "badge": "popular",
    "name": "Gentra", "description": "1 oylik to'liq kirish" },
  { "code": "malibu", "days": 75, "price_uzs": 109900, "old_price_uzs": 199900,
    "price_per_day_uzs": 1465, "discount_percent": 45, "badge": "best_value",
    "name": "Malibu", "description": "2,5 oylik to'liq kirish" }
]}
```

**Hisoblangan maydonlar (backend):**
- `price_per_day_uzs = round(price_uzs / days)` (butun son).
- `discount_percent = round((old_price_uzs - price_uzs) / old_price_uzs * 100)` agar `old_price_uzs` bor va `> price_uzs`, aks holda `0` (yoki `null`). `old_price_uzs` null bo'lsa → `discount_percent = 0`, javobda `old_price_uzs: null`.

## Implementatsiya joylari
- `internal/db/queries/billing.sql` — `ListActiveTariffs` (tariff ⨝ tariff_translation, locale + fallback, active, sort). sqlc generate.
- `internal/billing/` — mavjud paket; tariff DTO + list logikasi (per-day/discount hisobi shu yerda).
- `internal/billing/handlers.go` (yangi yoki mavjudga) — `GET /tariffs`, `server.go` da `ch`/billing route qatoriga ulash (public, auth'siz).
- `internal/db/migrations/0011_seed_tariffs.up.sql` / `.down.sql` — idempotent seed (`INSERT ... ON CONFLICT (code) DO NOTHING` tariff; translation ham). Down: shu 3 kodni o'chiradi.

## Testlar
- `ListActiveTariffs`: 3 tarif, sort tartibi, faqat active.
- Locale-fallback: mavjud bo'lmagan locale → uz-Latn; ru → ru.
- Hisoblar: per-day (3557/1997/1465), discount (29/40/45), `old_price_uzs=NULL` holati (discount=0).
- API: 200, JSON shakli, locale query.
- Seed migratsiya idempotent (ikki marta ishga tushса dublikat yo'q).

## Scope tashqarisi (keyingi Plan'lar)
- To'lov (M2-02 Payme, M2-03 Click), grant→entitlement (M2-04).
- Tarif UI / Matiz-karta (M2-08), checkout (M2-09).
- Narx/tarifni admindan tahrirlash (M3).
- Free-tier'ni tariff sifatida ifodalash (kerak emas — free = xarid yo'q).
