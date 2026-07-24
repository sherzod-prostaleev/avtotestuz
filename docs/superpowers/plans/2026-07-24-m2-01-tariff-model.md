# M2-01 Tariff Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve the three paid duration-pass tariffs (Nexia 7d / Gentra 30d / Malibu 75d) from the DB via a localized public `GET /api/v1/tariffs`, with computed per-day price and discount, plus an idempotent seed.

**Architecture:** The billing schema already exists (`tariff`, `tariff_translation`). Add a seed migration, one locale-fallback sqlc query, pure pricing helpers, a `billing.Service.ListTariffs` method that maps rows→DTOs, and a public `billing.Handler` wired in `server.go` next to the content handler. No auth (pricing shows before login).

**Tech Stack:** Go, chi, pgx, sqlc, Postgres. Tests: Go `testing` with `testdb` helper, run `-p 1`.

## Global Constraints
- Go toolchain: prefix every command with `PATH="$HOME/.local/go/bin:$PATH"`; sqlc at `~/go/bin`.
- DB tests: `TEST_DATABASE_URL` set, run with `-p 1` (single test DB).
- Locale fallback: requested locale → `uz-Latn` (mirror `ListCategories`). Locales: `uz-Latn`, `uz-Cyrl`, `ru`, `kaa`.
- `badge` is a KEY (`popular`/`best_value`/null) — backend never translates it; frontend i18n maps the label.
- Free "Matiz" tier is NOT in the DB (schema `days > 0`); it is a frontend-only card, out of scope here.
- Response envelope: `httpx.Data(w, 200, out)` → `{"data": [...]}`.

## File Structure
- Create: `backend/internal/db/migrations/0011_seed_tariffs.up.sql` / `.down.sql` — initial tariff rows + translations (idempotent).
- Create: `backend/internal/db/queries/billing.sql` — `ListActiveTariffs` locale-fallback query.
- Create: `backend/internal/billing/tariff.go` — `TariffDTO`, pure `pricePerDay`/`discountPercent`, `Service.ListTariffs`.
- Create: `backend/internal/billing/tariff_test.go` — unit tests for pricing math.
- Create: `backend/internal/billing/tariff_db_test.go` — DB test for `ListTariffs`.
- Create: `backend/internal/billing/handlers.go` — `Handler{Svc}` + `GET /tariffs`.
- Create: `backend/internal/billing/handlers_test.go` — endpoint test.
- Modify: `backend/internal/server/server.go` — wire `billing.Handler` (public) after `ch.Routes(api)`.
- Regenerate: `backend/internal/db/sqlc/*` via `make generate`.

---

### Task 1: Seed migration (initial tariffs)

**Files:**
- Create: `backend/internal/db/migrations/0011_seed_tariffs.up.sql`
- Create: `backend/internal/db/migrations/0011_seed_tariffs.down.sql`

**Interfaces:**
- Produces: rows in `tariff` (codes `nexia`,`gentra`,`malibu`) + `tariff_translation` (3 locales each) that later tasks read.

- [ ] **Step 1: Write the up migration**

`backend/internal/db/migrations/0011_seed_tariffs.up.sql`:
```sql
-- Initial paid duration-pass tariffs. Idempotent: ON CONFLICT keeps later
-- admin edits from being clobbered when migrations re-run on redeploy.
INSERT INTO tariff (code, days, price_uzs, old_price_uzs, badge, sort_order, active) VALUES
  ('nexia',  7,  24900,  34900,  NULL,         1, true),
  ('gentra', 30, 59900,  99900,  'popular',    2, true),
  ('malibu', 75, 109900, 199900, 'best_value', 3, true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO tariff_translation (tariff_id, locale, name, description)
SELECT t.id, v.locale, v.name, v.description
FROM tariff t
JOIN (VALUES
  ('nexia',  'uz-Latn'::locale_code, 'Nexia',  '1 haftalik to''liq kirish'),
  ('nexia',  'uz-Cyrl'::locale_code, 'Nexia',  '1 ҳафталик тўлиқ кириш'),
  ('nexia',  'ru'::locale_code,      'Nexia',  'Полный доступ на 1 неделю'),
  ('gentra', 'uz-Latn'::locale_code, 'Gentra', '1 oylik to''liq kirish'),
  ('gentra', 'uz-Cyrl'::locale_code, 'Gentra', '1 ойлик тўлиқ кириш'),
  ('gentra', 'ru'::locale_code,      'Gentra', 'Полный доступ на 1 месяц'),
  ('malibu', 'uz-Latn'::locale_code, 'Malibu', '2,5 oylik to''liq kirish'),
  ('malibu', 'uz-Cyrl'::locale_code, 'Malibu', '2,5 ойлик тўлиқ кириш'),
  ('malibu', 'ru'::locale_code,      'Malibu', 'Полный доступ на 2,5 месяца')
) AS v(code, locale, name, description) ON v.code = t.code
ON CONFLICT (tariff_id, locale) DO NOTHING;
```

- [ ] **Step 2: Write the down migration**

`backend/internal/db/migrations/0011_seed_tariffs.down.sql`:
```sql
DELETE FROM tariff WHERE code IN ('nexia', 'gentra', 'malibu');
```
(translations cascade via `tariff_translation.tariff_id ... ON DELETE CASCADE`.)

- [ ] **Step 3: Apply migrations and verify seed + idempotency**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest"
docker compose exec -T postgres psql -U avtotest -d avtotest -c "\
DO \$\$ BEGIN NULL; END \$\$;"  # ensure DB reachable
PATH="$HOME/.local/go/bin:$PATH" go run ./backend/cmd/api >/tmp/api.log 2>&1 &
sleep 3; kill %1 2>/dev/null   # api applies migrations on start
docker compose exec -T postgres psql -U avtotest -d avtotest -t -A -F'|' -c "
SELECT code, days, price_uzs, badge FROM tariff ORDER BY sort_order;
SELECT count(*) FROM tariff_translation;"
```
Expected: 3 tariff rows (nexia/gentra/malibu), `tariff_translation` count = 9. Re-running must not duplicate (ON CONFLICT).

> If migrations are applied by the importer/api on boot in this project, running the api once is enough. Otherwise apply via the project's migrate path.

- [ ] **Step 4: Commit**
```bash
git add backend/internal/db/migrations/0011_seed_tariffs.*.sql
git commit -m "feat(billing): seed initial duration-pass tariffs"
```

---

### Task 2: Pricing math (pure helpers)

**Files:**
- Create: `backend/internal/billing/tariff.go` (helpers only in this task)
- Create: `backend/internal/billing/tariff_test.go`

**Interfaces:**
- Produces: `func pricePerDay(priceUZS int64, days int32) int64`; `func discountPercent(priceUZS int64, oldPriceUZS *int64) int32`. Task 3 consumes both.

- [ ] **Step 1: Write the failing test**

`backend/internal/billing/tariff_test.go`:
```go
package billing

import "testing"

func TestPricePerDay(t *testing.T) {
	cases := []struct {
		price int64
		days  int32
		want  int64
	}{
		{24900, 7, 3557},
		{59900, 30, 1997},
		{109900, 75, 1465},
		{100, 0, 100}, // guard: no divide-by-zero
	}
	for _, c := range cases {
		if got := pricePerDay(c.price, c.days); got != c.want {
			t.Errorf("pricePerDay(%d,%d)=%d want %d", c.price, c.days, got, c.want)
		}
	}
}

func TestDiscountPercent(t *testing.T) {
	p := func(v int64) *int64 { return &v }
	cases := []struct {
		price int64
		old   *int64
		want  int32
	}{
		{24900, p(34900), 29},
		{59900, p(99900), 40},
		{109900, p(199900), 45},
		{100, nil, 0},   // no old price
		{100, p(50), 0}, // old not higher
	}
	for _, c := range cases {
		if got := discountPercent(c.price, c.old); got != c.want {
			t.Errorf("discountPercent(%d,%v)=%d want %d", c.price, c.old, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && PATH="$HOME/.local/go/bin:$PATH" go test ./internal/billing/ -run 'TestPricePerDay|TestDiscountPercent'`
Expected: FAIL (undefined: pricePerDay / discountPercent).

- [ ] **Step 3: Write minimal implementation**

`backend/internal/billing/tariff.go`:
```go
package billing

import "math"

// pricePerDay is the rounded per-day cost used for the "~X so'm/kun" framing.
func pricePerDay(priceUZS int64, days int32) int64 {
	if days <= 0 {
		return priceUZS
	}
	return int64(math.Round(float64(priceUZS) / float64(days)))
}

// discountPercent is the rounded savings vs the old price, or 0 when there is
// no old price or it is not higher than the current price.
func discountPercent(priceUZS int64, oldPriceUZS *int64) int32 {
	if oldPriceUZS == nil || *oldPriceUZS <= priceUZS {
		return 0
	}
	return int32(math.Round(float64(*oldPriceUZS-priceUZS) / float64(*oldPriceUZS) * 100))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && PATH="$HOME/.local/go/bin:$PATH" go test ./internal/billing/ -run 'TestPricePerDay|TestDiscountPercent'`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add backend/internal/billing/tariff.go backend/internal/billing/tariff_test.go
git commit -m "feat(billing): tariff per-day + discount helpers"
```

---

### Task 3: Query + `ListTariffs` service method

**Files:**
- Create: `backend/internal/db/queries/billing.sql`
- Modify: `backend/internal/billing/tariff.go` (add `TariffDTO` + `Service.ListTariffs`)
- Create: `backend/internal/billing/tariff_db_test.go`
- Regenerate: `backend/internal/db/sqlc/*`

**Interfaces:**
- Consumes: `pricePerDay`, `discountPercent` (Task 2); generated `Queries.ListActiveTariffs(ctx, locale string) ([]sqlc.ListActiveTariffsRow, error)`.
- Produces: `type TariffDTO struct{...}`; `func (s Service) ListTariffs(ctx context.Context, locale string) ([]TariffDTO, error)`. Task 4 consumes both.

- [ ] **Step 1: Write the query**

`backend/internal/db/queries/billing.sql`:
```sql
-- name: ListActiveTariffs :many
-- Localized tariff list with uz-Latn fallback (mirrors ListCategories).
SELECT t.code, t.days, t.price_uzs, t.old_price_uzs, t.badge,
       COALESCE(tr.name, ftr.name, t.code) AS name,
       COALESCE(tr.description, ftr.description, '') AS description
FROM tariff t
LEFT JOIN tariff_translation tr ON tr.tariff_id = t.id AND tr.locale = $1
LEFT JOIN tariff_translation ftr ON ftr.tariff_id = t.id AND ftr.locale = 'uz-Latn'
WHERE t.active = true
ORDER BY t.sort_order, t.code;
```

- [ ] **Step 2: Generate sqlc and confirm it compiles**

Run: `make generate && cd backend && PATH="$HOME/.local/go/bin:$PATH" go build ./...`
Expected: builds; `sqlc.Queries` now has `ListActiveTariffs`. Confirm the generated row field names/types:
```bash
grep -n "ListActiveTariffsRow\|func (q \*Queries) ListActiveTariffs" backend/internal/db/sqlc/billing.sql.go
```
Expected fields: `Code string`, `Days int32`, `PriceUzs int64`, `OldPriceUzs pgtype.Int8`, `Badge pgtype.Text`, `Name string`, `Description string`. (If names differ, use the actual generated names in Step 3.)

- [ ] **Step 3: Write the failing DB test**

`backend/internal/billing/tariff_db_test.go`:
```go
package billing

import (
	"context"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestListTariffs(t *testing.T) {
	pool := testdb.New(t)
	svc := Service{Q: sqlc.New(pool)}

	got, err := svc.ListTariffs(context.Background(), "uz-Latn")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d tariffs, want 3", len(got))
	}
	// sorted by sort_order: nexia, gentra, malibu
	if got[0].Code != "nexia" || got[1].Code != "gentra" || got[2].Code != "malibu" {
		t.Fatalf("wrong order: %s %s %s", got[0].Code, got[1].Code, got[2].Code)
	}
	g := got[1] // gentra
	if g.Days != 30 || g.PriceUZS != 59900 || g.PricePerDayUZS != 1997 || g.DiscountPercent != 40 {
		t.Errorf("gentra computed fields wrong: %+v", g)
	}
	if g.Badge == nil || *g.Badge != "popular" {
		t.Errorf("gentra badge = %v, want popular", g.Badge)
	}
	if got[0].Badge != nil {
		t.Errorf("nexia badge = %v, want nil", got[0].Badge)
	}
	if got[0].Name != "Nexia" || got[0].Description == "" {
		t.Errorf("nexia translation missing: %+v", got[0])
	}

	// ru locale returns Russian description
	ru, err := svc.ListTariffs(context.Background(), "ru")
	if err != nil {
		t.Fatal(err)
	}
	if ru[0].Description != "Полный доступ на 1 неделю" {
		t.Errorf("ru description = %q", ru[0].Description)
	}

	// unknown locale falls back to uz-Latn
	kaa, err := svc.ListTariffs(context.Background(), "kaa")
	if err != nil {
		t.Fatal(err)
	}
	if kaa[0].Description != "1 haftalik to'liq kirish" {
		t.Errorf("kaa fallback description = %q", kaa[0].Description)
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `cd backend && PATH="$HOME/.local/go/bin:$PATH" go test ./internal/billing/ -run TestListTariffs -p 1`
Expected: FAIL (undefined: TariffDTO / ListTariffs).

- [ ] **Step 5: Add `TariffDTO` + `ListTariffs` to `tariff.go`**

Append to `backend/internal/billing/tariff.go`:
```go
import (
	"context"
	// keep existing "math"
)

type TariffDTO struct {
	Code            string  `json:"code"`
	Days            int32   `json:"days"`
	PriceUZS        int64   `json:"price_uzs"`
	OldPriceUZS     *int64  `json:"old_price_uzs"`
	PricePerDayUZS  int64   `json:"price_per_day_uzs"`
	DiscountPercent int32   `json:"discount_percent"`
	Badge           *string `json:"badge"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
}

// ListTariffs returns active tariffs for a locale (uz-Latn fallback) with the
// per-day price and discount computed for display.
func (s Service) ListTariffs(ctx context.Context, locale string) ([]TariffDTO, error) {
	rows, err := s.Q.ListActiveTariffs(ctx, locale)
	if err != nil {
		return nil, err
	}
	out := make([]TariffDTO, 0, len(rows))
	for _, r := range rows {
		var old *int64
		if r.OldPriceUzs.Valid {
			v := r.OldPriceUzs.Int64
			old = &v
		}
		var badge *string
		if r.Badge.Valid && r.Badge.String != "" {
			b := r.Badge.String
			badge = &b
		}
		out = append(out, TariffDTO{
			Code:            r.Code,
			Days:            r.Days,
			PriceUZS:        r.PriceUzs,
			OldPriceUZS:     old,
			PricePerDayUZS:  pricePerDay(r.PriceUzs, r.Days),
			DiscountPercent: discountPercent(r.PriceUzs, old),
			Badge:           badge,
			Name:            r.Name,
			Description:     r.Description,
		})
	}
	return out, nil
}
```
> Merge the two `import` blocks in `tariff.go` into one (`context`, `math`).

- [ ] **Step 6: Run the test to verify it passes**

Run: `cd backend && PATH="$HOME/.local/go/bin:$PATH" go test ./internal/billing/ -run TestListTariffs -p 1`
Expected: PASS. (If `r.OldPriceUzs`/`r.Badge`/`r.PriceUzs` names differ from Step 2's generated names, adjust.)

- [ ] **Step 7: Commit**
```bash
git add backend/internal/db/queries/billing.sql backend/internal/db/sqlc/ backend/internal/billing/tariff.go backend/internal/billing/tariff_db_test.go
git commit -m "feat(billing): ListTariffs service with localized rows + computed pricing"
```

---

### Task 4: Public `GET /tariffs` handler + wiring

**Files:**
- Create: `backend/internal/billing/handlers.go`
- Create: `backend/internal/billing/handlers_test.go`
- Modify: `backend/internal/server/server.go` (after `ch.Routes(api)`)

**Interfaces:**
- Consumes: `Service.ListTariffs` (Task 3); `i18n.Parse`, `httpx.Data`/`httpx.Error`.
- Produces: `type Handler struct{ Svc Service }`; `func (h *Handler) Routes(r chi.Router)` registering `GET /tariffs`.

- [ ] **Step 1: Write the failing handler test**

`backend/internal/billing/handlers_test.go`:
```go
package billing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestTariffsEndpoint(t *testing.T) {
	pool := testdb.New(t)
	h := &Handler{Svc: Service{Q: sqlc.New(pool)}}
	r := chi.NewRouter()
	h.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/tariffs?locale=uz-Latn", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data []TariffDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 3 || body.Data[0].Code != "nexia" {
		t.Fatalf("unexpected tariffs: %+v", body.Data)
	}

	// invalid/missing locale → 400
	req2 := httptest.NewRequest(http.MethodGet, "/tariffs?locale=xx", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("bad locale status = %d, want 400", w2.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && PATH="$HOME/.local/go/bin:$PATH" go test ./internal/billing/ -run TestTariffsEndpoint -p 1`
Expected: FAIL (undefined: Handler).

- [ ] **Step 3: Write the handler**

`backend/internal/billing/handlers.go`:
```go
package billing

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/i18n"
)

type Handler struct {
	Svc Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/tariffs", h.listTariffs)
}

func (h *Handler) listTariffs(w http.ResponseWriter, r *http.Request) {
	loc, ok := i18n.Parse(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
		return
	}
	out, err := h.Svc.ListTariffs(r.Context(), loc)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "tariffs query failed")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	httpx.Data(w, http.StatusOK, out)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && PATH="$HOME/.local/go/bin:$PATH" go test ./internal/billing/ -run TestTariffsEndpoint -p 1`
Expected: PASS.

- [ ] **Step 5: Wire the handler in `server.go`**

In `backend/internal/server/server.go`, immediately after `ch.Routes(api)` (public, needs only `deps.Queries`):
```go
		bh := &billing.Handler{Svc: billing.Service{Q: deps.Queries}}
		bh.Routes(api)
```
Ensure `"avtotest.uz/backend/internal/billing"` is imported (it already is — `billing.Service` is used below).

- [ ] **Step 6: Full build + billing tests + smoke**

Run:
```bash
cd backend && PATH="$HOME/.local/go/bin:$PATH" go build ./... && go test ./internal/billing/ -p 1
```
Expected: build OK, all billing tests pass. Then live smoke:
```bash
curl -s "http://localhost:8090/api/v1/tariffs?locale=ru" | head -c 400
```
Expected: JSON `{"data":[...3 tariffs, Russian descriptions...]}`.

- [ ] **Step 7: Commit**
```bash
git add backend/internal/billing/handlers.go backend/internal/billing/handlers_test.go backend/internal/server/server.go
git commit -m "feat(billing): public GET /tariffs endpoint"
```

---

## Self-Review notes
- Spec coverage: seed (T1), API + localization + fallback (T3+T4), computed per-day/discount (T2+T3), tests (all), public/no-auth (T4 wiring). ✓
- Free "Matiz" card is explicitly out of scope (frontend M2-08). ✓
- Generated-field-name caveat is flagged in T3 Step 2 (verify actual sqlc names before using them in Step 5).
