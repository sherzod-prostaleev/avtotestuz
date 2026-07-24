# M2-04 Payment History Endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

**Goal:** `GET /api/v1/me/payments` — a profile's payment history (all statuses), tariff name localized, newest first.

**Architecture:** One new sqlc query joining `payment`/`tariff`/`tariff_translation` (locale-fallback, same pattern as `ListActiveTariffs`); one new handler method on the already-existing `account.Handler` (same package that serves `/me`, `/me/entitlement`).

**Tech Stack:** Go, chi, pgx, sqlc. Tests: `testdb`, `-p 1`.

## Global Constraints
- `PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"` for go/sqlc; `make generate` for sqlc (repo root).
- Return ALL payment statuses (`created,pending,paid,failed,canceled,refunded`) — this is history, not a receipts-only view. Do not filter by status.
- `limit` query param: `<=0` or absent → default `20` (exact convention from `internal/session/handlers.go`'s `listMySessions`); non-integer → `400 invalid_request`.
- `tariff_name` locale fallback: requested locale → `uz-Latn` → `tariff.code` (exact `COALESCE` pattern from `ListActiveTariffs` in `internal/db/queries/billing.sql`).
- Auth: same `/me/*` convention — `auth.FromContext(r.Context())`, 401 `unauthorized` if missing.
- `paid_at` is nullable — serialize as JSON `null` when unset (use `*time.Time` in the DTO, same style as `account.handlers.go`'s existing `toVIPDTO`/`toProfileDTO` optional fields).

---

### Task 1: `ListMyPayments` query + `GET /me/payments` handler
**Files:** Modify `internal/db/queries/billing.sql` (add query); Modify `internal/account/handlers.go` (add route + handler + DTO); Create `internal/account/payments_test.go`.

**Produces:** sqlc `ListMyPayments(ctx, ListMyPaymentsParams{ProfileID, Locale, LimitCount}) ([]ListMyPaymentsRow, error)`; `GET /api/v1/me/payments?limit=N` mounted via the existing `account.Handler.Routes`.

- [ ] Append to `internal/db/queries/billing.sql`:
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
  `export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH" && cd /home/sher/Рабочий\ стол/avtotest && make generate`. Check the actual generated `ListMyPaymentsParams`/`ListMyPaymentsRow` field names/types in `internal/db/sqlc/billing.sql.go` before writing the handler (the 3rd param, `LIMIT $3`, is an `int32` — sqlc names positional params by column-adjacent context; verify rather than guess, e.g. it may come out as `LimitCount` or `Limit` depending on sqlc's inference — check the generated struct directly).

- [ ] In `internal/account/handlers.go`, add to `Routes`: `r.Get("/me/payments", h.listMyPayments)` (same method as `getMe`/`getEntitlement`, NOT `AuthedRoutes` — this package's `Handler` is already mounted entirely behind `auth.Required` in `server.go`, mirroring how `getEntitlement` works today).

- [ ] Add the DTO and handler:
  ```go
  type paymentHistoryDTO struct {
      ID          string     `json:"id"`
      TariffCode  string     `json:"tariff_code"`
      TariffName  string     `json:"tariff_name"`
      TariffDays  int        `json:"tariff_days"`
      AmountUzs   int64      `json:"amount_uzs"`
      Provider    string     `json:"provider"`
      Status      string     `json:"status"`
      CreatedAt   time.Time  `json:"created_at"`
      PaidAt      *time.Time `json:"paid_at"`
  }

  func (h *Handler) listMyPayments(w http.ResponseWriter, r *http.Request) {
      claims, ok := auth.FromContext(r.Context())
      if !ok {
          httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
          return
      }
      loc, ok := i18n.Parse(r)
      if !ok {
          httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
          return
      }
      limit := 20
      if s := r.URL.Query().Get("limit"); s != "" {
          n, err := strconv.Atoi(s)
          if err != nil {
              httpx.Error(w, http.StatusBadRequest, "invalid_request", "limit must be an integer")
              return
          }
          if n > 0 {
              limit = n
          }
      }
      rows, err := h.Q.ListMyPayments(r.Context(), sqlc.ListMyPaymentsParams{
          ProfileID: claims.ProfileID,
          Locale:    loc,
          // NOTE: verify this field name against what sqlc actually generated
          // for the $3 LIMIT param (see Task 1's first step) before using it.
          LimitCount: int32(limit),
      })
      if err != nil {
          httpx.Error(w, http.StatusInternalServerError, "internal", "payment history query failed")
          return
      }
      out := make([]paymentHistoryDTO, len(rows))
      for i, row := range rows {
          dto := paymentHistoryDTO{
              ID:         row.ID.String(),
              TariffCode: row.TariffCode,
              TariffName: row.TariffName,
              TariffDays: int(row.TariffDays),
              AmountUzs:  row.AmountUzs,
              Provider:   row.Provider,
              Status:     row.Status,
              CreatedAt:  row.CreatedAt.Time,
          }
          if row.PaidAt.Valid {
              t := row.PaidAt.Time
              dto.PaidAt = &t
          }
          out[i] = dto
      }
      httpx.Data(w, http.StatusOK, out)
  }
  ```
  Add `"strconv"` and `"avtotest.uz/backend/internal/i18n"` imports to `handlers.go`. `row.CreatedAt`/`row.PaidAt` are `pgtype.Timestamptz` (verify against generated code); `row.ID` is `uuid.UUID`; `row.AmountUzs`/`row.TariffDays` types come straight from the `payment`/`tariff` columns (`bigint`→`int64`, `int`→`int32` — verify).

- [ ] **TDD** — write `internal/account/payments_test.go` FIRST (confirm it fails against the not-yet-added route/handler), covering:
  1. Seed 2 payments (different `created_at`, different statuses e.g. one `paid` one `failed`) for one profile + a `tariff_translation` row for the requested locale → `GET /me/payments` returns both, newest first, `tariff_name` from the matching-locale translation.
  2. A payment whose tariff has NO translation for the requested locale but DOES have `uz-Latn` → `tariff_name` falls back to the `uz-Latn` name.
  3. `?limit=1` → only 1 result (the newest).
  4. `?limit=abc` → `400 invalid_request`.
  5. A second profile's payments are never returned (profile isolation).
  6. No auth header → `401 unauthorized`.
  7. A `paid` row's `paid_at` is a real timestamp in the response; a `created`/`pending` row's `paid_at` is JSON `null`.

  `internal/account/handlers_test.go` ALREADY EXISTS (package `account_test`) with a `setup(t) (*httptest.Server, sqlc.Profile)` helper (creates a profile, mounts `account.Handler` behind `auth.Required` on a real `httptest.Server`) and a `doReq(t, ts, method, path, token, body) (int, respEnvelope)` helper that decodes the `{data, error}` envelope — put `payments_test.go` in the SAME `account_test` package and reuse `setup`/`doReq` verbatim rather than inventing new test scaffolding. Check that file for how it mints a valid JWT for `token` (likely a small local helper using `auth.IssueAccess` or similar — follow its exact pattern).

- [ ] `go build ./...` and `go test ./internal/account/... -p 1` green (GREEN evidence).
- [ ] `gofmt -l .` / `go vet ./...` clean.
- [ ] Commit: `feat(account): payment history endpoint (GET /me/payments)`.

---

## Self-Review
- Spec coverage: single endpoint, all statuses returned, limit default/validation, locale fallback, profile isolation, nullable `paid_at` — all covered by Task 1's 7 test cases. ✓
- No new migration/table needed (spec confirmed this) — verified `payment`/`tariff`/`tariff_translation` already exist and are unmodified by this plan.
- Out of scope respected: no FE, no promo/receipt-PDF.
- This is a single self-contained task (no multi-task decomposition needed given the small, well-precedented scope) — dispatch as one implementer + one task reviewer, no separate final whole-branch review needed beyond the task review itself (nothing else in the branch to integrate against).
