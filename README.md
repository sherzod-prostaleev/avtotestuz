# AvtoTest Platform

O'zbekiston YHQ nazariy imtihoniga tayyorlovchi onlayn maktab. Spec:
`docs/superpowers/specs/2026-07-17-avtotest-platform-master-design.md`.

## Dev boshlash

Talablar: Docker (compose bilan), Go 1.22+ (`~/.local/go`ga o'rnatilgan bo'lsa,
`export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`).

```bash
make up      # Postgres + Redis + MinIO (compose)
make seed    # [NAMUNA] sample kontentni import qiladi
make run     # API :8080 (PORT env bilan o'zgartiriladi)
make check   # lint + testlar
```

Sinash: `curl "localhost:8080/api/v1/variants/1?locale=uz-Latn"`

## Tuzilma

- `backend/` — Go API (chi + pgx/sqlc + golang-migrate)
- `docs/superpowers/specs|plans` — dizayn hujjatlari va rejalar
- Kontent importi: `backend/cmd/importer -data <dir> -verified`
  (canonical format: `data.json` + `images/`; barcha invariantlar tekshiriladi,
  buzilganlar quarantine bo'ladi — hech narsa taxmin qilinmaydi)

## API (M1 Plan 01 holati)

- `GET /healthz`
- `GET /api/v1/categories?locale=`
- `GET /api/v1/variants` · `GET /api/v1/variants/{n}?locale=`
- `GET /api/v1/signs?group=&q=&locale=` · `GET /api/v1/signs/{code}`
- `GET /api/v1/questions/{id}?locale=`

Javob konverti: `{"data":..., "meta":{...}}` yoki `{"error":{"code","message"}}`.
Kontent javoblarida to'g'ri javob maydonlari hech qachon qaytmaydi (anti-cheat).
