# B2B Station Kiosk — Faza 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A driving school enrolls any number of classroom PCs with one org code; each PC authenticates as itself with a hardware-bound Ed25519 key and runs VIP with zero logins.

**Architecture:** The copyable `X-Device-Fingerprint` header is deleted. A station now proves identity by signing a server nonce with a private key sealed under Windows DPAPI (machine scope) and pinned to a hardware id. On success the backend mints a short-lived `typ:"station"` JWT whose `sub` is a per-station shadow `profile` row (`kind='station'`), so every existing profile-keyed subsystem (sessions, answers, progress) works untouched. A small Go agent on the PC holds the key, refreshes the token, reverse-proxies the browser and launches Chrome in kiosk mode.

**Tech Stack:** Go 1.x (pgx v5, chi, golang-jwt v5, go-redis, crypto/ed25519), PostgreSQL, Next.js 15 (App Router, TypeScript, next-intl), Vitest.

**Spec:** `docs/superpowers/specs/2026-08-04-b2b-station-kiosk-design.md`

## Global Constraints

- Faza 1 only. Offline lease, content cache, result queue, MSI/GPO packaging and anomaly detection are Faza 2/3 and get their own plans. **Do not implement them here.**
- Prod has **zero** active stations. No backward compatibility with the fingerprint model — delete it outright.
- Backend tests need Postgres: `make up` first, then `make test`. Full gate: `make check` (lint + test).
- Frontend gate: `make fe-check` (lint + typecheck + test + build).
- The `b2b` package uses raw pgx through `b2b.Store{Pool}`, **not** sqlc. Keep new B2B code on raw pgx. Only `backend/internal/db/queries/*.sql` changes require `make generate`.
- Every user-facing string must exist in all three locale files: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`.
- Station access-token TTL: **15 minutes**. Challenge nonce TTL: **60 seconds**. Enroll window default TTL: **2 hours**.
- Signed challenge message format (exact, used by both server and agent): `avtotest-station-v1|<station_id>|<nonce>|<unix_ts>`
- Commit after every task. Never commit a failing `make check`.

---

### Task 1: Delete the fingerprint model and land the new schema

Demolition and schema in one commit so the tree never sits in a half-migrated state.

**Files:**
- Create: `backend/internal/db/migrations/0056_b2b_station_key.up.sql`
- Create: `backend/internal/db/migrations/0056_b2b_station_key.down.sql`
- Delete: `backend/internal/devicefp/devicefp.go` (and the package directory)
- Delete: `frontend/src/lib/device-fingerprint.ts`
- Modify: `backend/internal/b2b/station.go` — remove `CreateActivateCode`, `CreateActivateCodeAsTeacher`, `ActivateStation`, `newActivateCode`, `ActivateCodeRow`, `maskFingerprint`, and `StationRow.Fingerprint`
- Modify: `backend/internal/b2b/handlers.go` — remove `createStationCode`, `activateStation` handlers and their routes
- Modify: `backend/internal/admin/b2b_handlers.go` — remove station-code endpoints
- Modify: `backend/internal/server/server.go:64` and `:116` — drop `devicefp` from CORS headers and the `api.Use(devicefp.Middleware)` line
- Modify: `backend/internal/testdb/testdb.go:223` — drop `b2b_station_activate_code` from the TRUNCATE list
- Modify: `backend/internal/b2b/station_test.go` — delete `TestStationVIPBindAndGate` (its API no longer exists; Task 2 and 4 replace it)
- Modify: `frontend/src/lib/api-client.ts:1,17-18`
- Modify: `frontend/src/app/api/proxy/[...path]/route.ts:69`
- Modify: `frontend/src/app/[locale]/(app)/teacher/page.tsx:8,223`
- Delete: `frontend/src/app/api/admin/b2b/orgs/[id]/station-codes/route.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: tables `b2b_org_enroll_code`, `b2b_station` (new columns `public_key bytea`, `hwid_hash text`, `agent_version text`, `last_ip inet`, `station_profile_id uuid`), column `profile.kind`.

- [ ] **Step 1: Write the migration**

`backend/internal/db/migrations/0056_b2b_station_key.up.sql`:

```sql
-- Replace the copyable device-fingerprint binding with a hardware-bound
-- Ed25519 keypair, and let a station authenticate as itself via a shadow
-- profile so classroom PCs need no learner accounts.

ALTER TABLE profile
  ADD COLUMN kind text NOT NULL DEFAULT 'user'
  CHECK (kind IN ('user', 'station'));

-- Prod has no live stations; the old binding is worthless, so start clean.
TRUNCATE b2b_station, b2b_station_activate_code;
DROP TABLE b2b_station_activate_code;

ALTER TABLE b2b_station
  DROP COLUMN fingerprint,
  DROP COLUMN activate_code_id,
  ADD COLUMN public_key         bytea NOT NULL,
  ADD COLUMN hwid_hash          text  NOT NULL,
  ADD COLUMN agent_version      text  NOT NULL DEFAULT '',
  ADD COLUMN last_ip            inet,
  ADD COLUMN station_profile_id uuid REFERENCES profile(id) ON DELETE SET NULL;

-- One active bind per physical machine, globally (anti-leak across orgs).
CREATE UNIQUE INDEX b2b_station_active_hwid_uidx
  ON b2b_station (hwid_hash) WHERE status = 'active';

CREATE TABLE b2b_org_enroll_code (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  code       text NOT NULL UNIQUE,
  max_uses   int  NOT NULL CHECK (max_uses > 0),
  used_count int  NOT NULL DEFAULT 0 CHECK (used_count >= 0),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX b2b_org_enroll_code_org_idx
  ON b2b_org_enroll_code (org_id, expires_at DESC);
```

`backend/internal/db/migrations/0056_b2b_station_key.down.sql`:

```sql
DROP TABLE IF EXISTS b2b_org_enroll_code;
DROP INDEX IF EXISTS b2b_station_active_hwid_uidx;

TRUNCATE b2b_station;
ALTER TABLE b2b_station
  DROP COLUMN station_profile_id,
  DROP COLUMN last_ip,
  DROP COLUMN agent_version,
  DROP COLUMN hwid_hash,
  DROP COLUMN public_key,
  ADD COLUMN fingerprint text NOT NULL,
  ADD COLUMN activate_code_id uuid;

CREATE UNIQUE INDEX b2b_station_active_fingerprint_uidx
  ON b2b_station (fingerprint) WHERE status = 'active';

CREATE TABLE b2b_station_activate_code (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  code       text NOT NULL UNIQUE,
  label      text NOT NULL DEFAULT '',
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX b2b_station_activate_code_org_idx
  ON b2b_station_activate_code (org_id, expires_at DESC);

ALTER TABLE profile DROP COLUMN kind;
```

- [ ] **Step 2: Delete the fingerprint code paths**

Delete these files entirely:

```bash
rm -r backend/internal/devicefp
rm frontend/src/lib/device-fingerprint.ts
rm frontend/src/app/api/admin/b2b/orgs/\[id\]/station-codes/route.ts
```

In `backend/internal/server/server.go`, remove the `avtotest.uz/backend/internal/devicefp` import, delete the `api.Use(devicefp.Middleware)` line, and change the CORS header list to:

```go
AllowedHeaders: []string{"Authorization", "Content-Type", "X-Ops-Token"},
```

In `backend/internal/b2b/station.go`, delete `newActivateCode`, `ActivateCodeRow`, `CreateActivateCode`, `CreateActivateCodeAsTeacher`, `ActivateStation`, `maskFingerprint`, the `devicefp` import, and the `Fingerprint` field of `StationRow`. Keep `CountActiveStations`, `ActiveHomeSeats`, `LicenseEndsAt`, `ListStations`, `ListStationsAsTeacher`, `RevokeStation`, `RevokeStationAsTeacher`, `RenameStationAsTeacher`, `SetOrgStatus`, `LicenseExpiringSoon`, `ErrSeatsExhausted`, `ErrOrgSuspended`, `ErrNoLicense`. `ActiveStationVIP` is rewritten in Task 2 — for now delete its body's `devicefp` use by deleting the whole function; Task 2 adds it back with the new signature.

`ListStations` loses its `includeFingerprint` parameter and its `fingerprint` column. New signature and query:

```go
// ListStations returns stations for an org.
func (s Store) ListStations(ctx context.Context, orgID uuid.UUID) ([]StationRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, org_id, label, status, activated_at, last_seen_at, activated_by
		FROM b2b_station WHERE org_id = $1
		ORDER BY
		  CASE status WHEN 'active' THEN 0 ELSE 1 END,
		  activated_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StationRow, 0)
	for rows.Next() {
		var row StationRow
		if err := rows.Scan(&row.ID, &row.OrgID, &row.Label, &row.Status,
			&row.ActivatedAt, &row.LastSeenAt, &row.ActivatedBy); err != nil {
			return nil, err
		}
		row.ActivatedAt = row.ActivatedAt.UTC()
		row.LastSeenAt = row.LastSeenAt.UTC()
		out = append(out, row)
	}
	return out, rows.Err()
}
```

Update `ListStationsAsTeacher` to call `s.ListStations(ctx, orgID)`, and `RenameStationAsTeacher` to drop `fingerprint` from its `RETURNING` list and the `row.Fingerprint = maskFingerprint(...)` line.

In `backend/internal/b2b/handlers.go` delete the `createStationCode` and `activateStation` handlers, the `stationCodeBody` and `activateStationBody` types, and these two route lines:

```go
r.Post("/me/teacher/orgs/{id}/station-codes", h.createStationCode)
r.Post("/me/stations/activate", h.activateStation)
```

In `backend/internal/admin/b2b_handlers.go` delete every handler and route referring to station activation codes (grep for `station-codes` and `CreateActivateCode`).

**Keep the tree compiling.** Deleting `ActiveStationVIP` breaks two callers, so fix them in this same step. In `backend/internal/billing/entitlement.go`, delete the `devicefp` import and the `stationFingerprint` helper (lines 20-22), and cut `Status` back to entitlement-only:

```go
// Status reports whether profileID currently has an active entitlement.
// Station VIP is rewired in the next commit; until then only a personal
// entitlement grants access.
func (s Service) Status(ctx context.Context, profileID uuid.UUID) (active bool, until *time.Time, err error) {
	ends, err := s.Q.ActiveEntitlementEnd(ctx, profileID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, nil, err
		}
		return false, nil, nil
	}
	t := ends.Time
	return true, &t, nil
}
```

Leave the `StationVIPChecker` interface and the `StationVIP` field declared but unused — Task 2 rewrites both. In `backend/internal/server/server.go`, delete the now-dangling wiring:

```go
			var stationVIP billing.StationVIPChecker
			if deps.Pool != nil {
				stationVIP = b2b.Store{Pool: deps.Pool}
			}
```

and drop `StationVIP: stationVIP,` from the `learnerBilling` literal.

Delete `TestStationVIPBindAndGate` from `backend/internal/b2b/station_test.go`.

In `backend/internal/testdb/testdb.go`, change line 223 to:

```go
				b2b_invite, b2b_station, b2b_org_enroll_code,
```

- [ ] **Step 3: Clean the frontend**

`frontend/src/lib/api-client.ts` — delete the `device-fingerprint` import and these two lines:

```ts
  const fp = getDeviceFingerprint();
  if (fp) headers[DEVICE_FP_HEADER] = fp;
```

`frontend/src/app/api/proxy/[...path]/route.ts` — delete line 69 (`const deviceFp = ...`) and every use of `deviceFp` below it.

`frontend/src/app/[locale]/(app)/teacher/page.tsx` — delete the `getDeviceFingerprint` import and the whole "activate this PC" form, including the `fingerprint: getDeviceFingerprint()` payload field. Task 9 rebuilds this page section.

- [ ] **Step 4: Run the full gate**

```bash
make up
make test-db-reset
make check
make fe-check
```

Expected: both PASS. `make test-db-reset` is required because migration 0055 changed shape; stale per-package test databases would keep the old columns.

- [ ] **Step 5: Commit**

```bash
git add -A backend frontend
git commit -m "feat(b2b)!: replace device fingerprint with hardware-bound station schema

The X-Device-Fingerprint header was a client-supplied string: copying it out
of localStorage granted classroom VIP from any machine. Delete it along with
the per-PC activation codes, and add the schema for key-based binding:
public_key, hwid_hash, a shadow station profile and an org-scoped enroll code."
```

---

### Task 2: Station VIP keyed by station id

**Files:**
- Create: `backend/internal/stationctx/stationctx.go`
- Create: `backend/internal/stationctx/stationctx_test.go`
- Modify: `backend/internal/b2b/station.go` — add `ActiveStationVIP` back with a `uuid.UUID` parameter
- Modify: `backend/internal/billing/entitlement.go:20-22,33-37,94-98`
- Create: `backend/internal/b2b/station_vip_test.go`

**Interfaces:**
- Consumes: `b2b_station` schema from Task 1.
- Produces:
  - `stationctx.WithContext(ctx context.Context, stationID uuid.UUID) context.Context`
  - `stationctx.FromContext(ctx context.Context) (uuid.UUID, bool)`
  - `b2b.Store.ActiveStationVIP(ctx context.Context, stationID uuid.UUID) (bool, *time.Time, error)`
  - `billing.StationVIPChecker` with the same signature.

- [ ] **Step 1: Write the failing test for stationctx**

`backend/internal/stationctx/stationctx_test.go`:

```go
package stationctx_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/stationctx"
)

func TestRoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := stationctx.WithContext(context.Background(), id)
	got, ok := stationctx.FromContext(ctx)
	if !ok || got != id {
		t.Fatalf("got=%v ok=%v want=%v", got, ok, id)
	}
}

func TestEmptyContext(t *testing.T) {
	if _, ok := stationctx.FromContext(context.Background()); ok {
		t.Fatal("bare context must carry no station")
	}
}

func TestNilUUIDIsNotStored(t *testing.T) {
	ctx := stationctx.WithContext(context.Background(), uuid.Nil)
	if _, ok := stationctx.FromContext(ctx); ok {
		t.Fatal("uuid.Nil must not register as a station")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/stationctx/ -run TestRoundTrip -v`
Expected: FAIL — package `avtotest.uz/backend/internal/stationctx` does not exist.

- [ ] **Step 3: Implement stationctx**

`backend/internal/stationctx/stationctx.go`:

```go
// Package stationctx carries a verified B2B station id on the request
// context so entitlement checks can grant classroom VIP without a personal
// purchase.
//
// It replaces internal/devicefp, which read an attacker-controlled header.
// Nothing here parses input: the id is written only by auth.Required after a
// station JWT has been verified, so anything found on the context is already
// authenticated.
package stationctx

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey struct{}

// WithContext stores a verified station id on ctx. uuid.Nil is ignored so a
// zero value can never be mistaken for a bound station.
func WithContext(ctx context.Context, stationID uuid.UUID) context.Context {
	if stationID == uuid.Nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, stationID)
}

// FromContext returns the station id if the request was authenticated as a
// classroom station.
func FromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `cd backend && go test ./internal/stationctx/ -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Write the failing test for station VIP**

`backend/internal/b2b/station_vip_test.go`:

```go
package b2b_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/stationctx"
	"avtotest.uz/backend/internal/testdb"
)

func TestActiveStationVIPGrantsAndRevokes(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('Demo School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, home_seats, starts_at, ends_at, note)
		VALUES ($1, 5, 0, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var profileID, stationID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO profile (phone, name, kind)
		VALUES ('st:' || gen_random_uuid(), 'PC-1', 'station') RETURNING id`).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO b2b_station (org_id, public_key, hwid_hash, label, station_profile_id)
		VALUES ($1, $2, 'hw-1', 'PC-1', $3) RETURNING id`,
		orgID, []byte(pub), profileID).Scan(&stationID); err != nil {
		t.Fatal(err)
	}

	bill := billing.Service{Q: sqlc.New(pool), StationVIP: store}

	active, until, err := bill.Status(stationctx.WithContext(ctx, stationID), profileID)
	if err != nil || !active || until == nil {
		t.Fatalf("bound station must have VIP: active=%v until=%v err=%v", active, until, err)
	}

	// A context with no station id gets nothing.
	if active, _, err := bill.Status(ctx, profileID); err != nil || active {
		t.Fatalf("bare context must not grant VIP: active=%v err=%v", active, err)
	}

	// An unknown station id gets nothing.
	if active, _, err := bill.Status(stationctx.WithContext(ctx, uuid.New()), profileID); err != nil || active {
		t.Fatalf("unknown station must not grant VIP: active=%v err=%v", active, err)
	}

	if err := store.SetOrgStatus(ctx, orgID, "suspended"); err != nil {
		t.Fatal(err)
	}
	if active, _, err := bill.Status(stationctx.WithContext(ctx, stationID), profileID); err != nil || active {
		t.Fatalf("suspended org must revoke VIP: active=%v err=%v", active, err)
	}

	if err := store.SetOrgStatus(ctx, orgID, "active"); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeStation(ctx, orgID, stationID); err != nil {
		t.Fatal(err)
	}
	if active, _, err := bill.Status(stationctx.WithContext(ctx, stationID), profileID); err != nil || active {
		t.Fatalf("revoked station must lose VIP: active=%v err=%v", active, err)
	}
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `cd backend && go test ./internal/b2b/ -run TestActiveStationVIPGrantsAndRevokes -v`
Expected: FAIL — `store.ActiveStationVIP` undefined / does not satisfy `billing.StationVIPChecker`.

- [ ] **Step 7: Implement the new checker**

Add to `backend/internal/b2b/station.go`:

```go
// ActiveStationVIP implements billing.StationVIPChecker: the station must be
// active, under a non-suspended org with a live license. The station id comes
// from a verified JWT (see stationctx), never from a request header.
func (s Store) ActiveStationVIP(ctx context.Context, stationID uuid.UUID) (bool, *time.Time, error) {
	if stationID == uuid.Nil {
		return false, nil, nil
	}
	var ends time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT MAX(l.ends_at)
		FROM b2b_station s
		JOIN b2b_org o ON o.id = s.org_id AND o.status = 'active'
		JOIN b2b_org_license l ON l.org_id = s.org_id
		  AND l.starts_at <= now() AND l.ends_at > now()
		WHERE s.id = $1 AND s.status = 'active'
		GROUP BY s.org_id`, stationID).Scan(&ends)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, err
	}
	_, _ = s.Pool.Exec(ctx, `
		UPDATE b2b_station SET last_seen_at = now() WHERE id = $1`, stationID)
	t := ends.UTC()
	return true, &t, nil
}
```

In `backend/internal/billing/entitlement.go`, replace the `devicefp` import with `avtotest.uz/backend/internal/stationctx`, delete the `stationFingerprint` helper (lines 20-22), and change the interface and call site:

```go
// StationVIPChecker grants classroom VIP when the request was authenticated
// as a bound station under a live, non-suspended org license.
type StationVIPChecker interface {
	ActiveStationVIP(ctx context.Context, stationID uuid.UUID) (active bool, until *time.Time, err error)
}
```

```go
	if s.StationVIP == nil {
		return false, nil, nil
	}
	stationID, ok := stationctx.FromContext(ctx)
	if !ok {
		return false, nil, nil
	}
	return s.StationVIP.ActiveStationVIP(ctx, stationID)
```

`Status` must fall through to this station check rather than returning early, so restore its shape:

```go
func (s Service) Status(ctx context.Context, profileID uuid.UUID) (active bool, until *time.Time, err error) {
	ends, err := s.Q.ActiveEntitlementEnd(ctx, profileID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, nil, err
		}
	} else {
		t := ends.Time
		return true, &t, nil
	}
	if s.StationVIP == nil {
		return false, nil, nil
	}
	stationID, ok := stationctx.FromContext(ctx)
	if !ok {
		return false, nil, nil
	}
	return s.StationVIP.ActiveStationVIP(ctx, stationID)
}
```

Restore the server wiring that Task 1 removed. In `backend/internal/server/server.go`, inside `r.Route("/api/v1", ...)`:

```go
			var stationVIP billing.StationVIPChecker
			if deps.Pool != nil {
				stationVIP = b2b.Store{Pool: deps.Pool}
			}
```

and add `StationVIP: stationVIP,` back to the `learnerBilling` literal.

- [ ] **Step 8: Run the tests**

Run: `cd backend && go test ./internal/b2b/ ./internal/billing/ ./internal/stationctx/ ./internal/server/ -count=1`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/stationctx backend/internal/b2b backend/internal/billing
git commit -m "feat(b2b): key classroom VIP off a verified station id

stationctx replaces devicefp: the id is written only after a station JWT
verifies, so no header can forge it."
```

---

### Task 3: Org-scoped enrollment window

**Files:**
- Create: `backend/internal/b2b/enroll_code.go`
- Create: `backend/internal/b2b/enroll_code_test.go`

**Interfaces:**
- Consumes: `b2b_org_enroll_code` from Task 1; `Store.teacherRole` (existing, `backend/internal/b2b/store.go`); `Store.CountActiveStations`; errors `ErrNotFound`, `ErrInvalid`, `ErrConflict`, `ErrOrgSuspended`, `ErrNoLicense` (existing).
- Produces:
  - `type EnrollCodeRow struct { ID, OrgID uuid.UUID; Code string; MaxUses, UsedCount int; ExpiresAt time.Time; RevokedAt *time.Time; CreatedAt time.Time }`
  - `Store.OpenEnrollWindow(ctx, orgID uuid.UUID, ttl time.Duration, createdBy string) (EnrollCodeRow, error)`
  - `Store.OpenEnrollWindowAsTeacher(ctx, actorID, orgID uuid.UUID, ttl time.Duration) (EnrollCodeRow, error)`
  - `Store.CloseEnrollWindowAsTeacher(ctx, actorID, orgID, codeID uuid.UUID) error`
  - `Store.ActiveEnrollCodeAsTeacher(ctx, actorID, orgID uuid.UUID) (*EnrollCodeRow, error)`

- [ ] **Step 1: Write the failing test**

`backend/internal/b2b/enroll_code_test.go`:

```go
package b2b_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
)

func TestOpenEnrollWindowSizesToFreeSeats(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, home_seats, starts_at, ends_at, note)
		VALUES ($1, 30, 0, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	code, err := store.OpenEnrollWindow(ctx, orgID, 2*time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if code.MaxUses != 30 {
		t.Fatalf("max_uses=%d, want 30 (all seats free)", code.MaxUses)
	}
	if !strings.HasPrefix(code.Code, "AVTO-") {
		t.Fatalf("code=%q, want AVTO- prefix", code.Code)
	}
	if code.ExpiresAt.Sub(time.Now().UTC()) > 2*time.Hour+time.Minute {
		t.Fatalf("expires_at too far out: %v", code.ExpiresAt)
	}

	// Opening a second window closes the first: a school must never have two
	// live codes in circulation.
	code2, err := store.OpenEnrollWindow(ctx, orgID, 2*time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	var revoked *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at FROM b2b_org_enroll_code WHERE id = $1`, code.ID).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if revoked == nil {
		t.Fatal("opening a new window must revoke the previous one")
	}
	if code2.Code == code.Code {
		t.Fatal("second window must mint a different code")
	}
}

func TestOpenEnrollWindowRefusesSuspendedOrgAndDeadLicense(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}

	// No license yet.
	if _, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test"); err == nil {
		t.Fatal("expected ErrNoLicense without a live license")
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, home_seats, starts_at, ends_at, note)
		VALUES ($1, 10, 0, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOrgStatus(ctx, orgID, "suspended"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test"); err == nil {
		t.Fatal("expected ErrOrgSuspended for a suspended org")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/b2b/ -run TestOpenEnrollWindow -v`
Expected: FAIL — `store.OpenEnrollWindow` undefined.

- [ ] **Step 3: Implement**

`backend/internal/b2b/enroll_code.go`:

```go
package b2b

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// EnrollCodeRow is an org-scoped enrollment window. One code enrolls every PC
// in a school, capped by free seats and a short expiry — the per-PC codes it
// replaced made a 100-machine rollout unmanageable.
type EnrollCodeRow struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"org_id"`
	Code      string     `json:"code"`
	MaxUses   int        `json:"max_uses"`
	UsedCount int        `json:"used_count"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// defaultEnrollTTL and maxEnrollTTL bound how long a code stays live. Short
// is the point: a leaked code that expires in two hours is worth little.
const (
	defaultEnrollTTL = 2 * time.Hour
	maxEnrollTTL     = 24 * time.Hour
)

// enrollAlphabet excludes I, O, 0 and 1 — codes get read aloud and retyped by
// school IT staff.
const enrollAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newEnrollCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 0, 13) // AVTO- + 4 + - + 4
	out = append(out, "AVTO-"...)
	for i, b := range buf {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, enrollAlphabet[int(b)%len(enrollAlphabet)])
	}
	return string(out), nil
}

// OpenEnrollWindow revokes any live window and mints a new one sized to the
// org's free seats.
func (s Store) OpenEnrollWindow(ctx context.Context, orgID uuid.UUID, ttl time.Duration, createdBy string) (EnrollCodeRow, error) {
	if ttl <= 0 {
		ttl = defaultEnrollTTL
	}
	if ttl > maxEnrollTTL {
		ttl = maxEnrollTTL
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM b2b_org WHERE id = $1 FOR UPDATE`, orgID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EnrollCodeRow{}, ErrNotFound
		}
		return EnrollCodeRow{}, err
	}
	if status != "active" {
		return EnrollCodeRow{}, ErrOrgSuspended
	}

	var seats int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(seats), 0) FROM b2b_org_license
		WHERE org_id = $1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&seats); err != nil {
		return EnrollCodeRow{}, err
	}
	if seats <= 0 {
		return EnrollCodeRow{}, ErrNoLicense
	}
	var used int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM b2b_station WHERE org_id = $1 AND status = 'active'`, orgID).Scan(&used); err != nil {
		return EnrollCodeRow{}, err
	}
	free := seats - used
	if free <= 0 {
		return EnrollCodeRow{}, ErrSeatsExhausted
	}

	if _, err := tx.Exec(ctx, `
		UPDATE b2b_org_enroll_code SET revoked_at = now()
		WHERE org_id = $1 AND revoked_at IS NULL AND expires_at > now()`, orgID); err != nil {
		return EnrollCodeRow{}, err
	}

	code, err := newEnrollCode()
	if err != nil {
		return EnrollCodeRow{}, err
	}
	row, err := insertEnrollCode(ctx, tx, orgID, code, int(free), time.Now().UTC().Add(ttl), createdBy)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EnrollCodeRow{}, err
	}
	return row, nil
}

func insertEnrollCode(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, code string, maxUses int, expires time.Time, createdBy string) (EnrollCodeRow, error) {
	var row EnrollCodeRow
	var revoked pgtype.Timestamptz
	err := tx.QueryRow(ctx, `
		INSERT INTO b2b_org_enroll_code (org_id, code, max_uses, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, org_id, code, max_uses, used_count, expires_at, revoked_at, created_at`,
		orgID, code, maxUses, expires, createdBy,
	).Scan(&row.ID, &row.OrgID, &row.Code, &row.MaxUses, &row.UsedCount,
		&row.ExpiresAt, &revoked, &row.CreatedAt)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	if revoked.Valid {
		t := revoked.Time.UTC()
		row.RevokedAt = &t
	}
	row.ExpiresAt = row.ExpiresAt.UTC()
	row.CreatedAt = row.CreatedAt.UTC()
	return row, nil
}

// OpenEnrollWindowAsTeacher requires owner/teacher membership.
func (s Store) OpenEnrollWindowAsTeacher(ctx context.Context, actorID, orgID uuid.UUID, ttl time.Duration) (EnrollCodeRow, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return EnrollCodeRow{}, err
	}
	return s.OpenEnrollWindow(ctx, orgID, ttl, "profile:"+actorID.String())
}

// CloseEnrollWindowAsTeacher revokes a live window early.
func (s Store) CloseEnrollWindowAsTeacher(ctx context.Context, actorID, orgID, codeID uuid.UUID) error {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return err
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE b2b_org_enroll_code SET revoked_at = now()
		WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL`, codeID, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ActiveEnrollCodeAsTeacher returns the live window, or nil when none is open.
func (s Store) ActiveEnrollCodeAsTeacher(ctx context.Context, actorID, orgID uuid.UUID) (*EnrollCodeRow, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	var row EnrollCodeRow
	var revoked pgtype.Timestamptz
	err := s.Pool.QueryRow(ctx, `
		SELECT id, org_id, code, max_uses, used_count, expires_at, revoked_at, created_at
		FROM b2b_org_enroll_code
		WHERE org_id = $1 AND revoked_at IS NULL AND expires_at > now()
		  AND used_count < max_uses
		ORDER BY created_at DESC LIMIT 1`, orgID,
	).Scan(&row.ID, &row.OrgID, &row.Code, &row.MaxUses, &row.UsedCount,
		&row.ExpiresAt, &revoked, &row.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("active enroll code: %w", err)
	}
	row.ExpiresAt = row.ExpiresAt.UTC()
	row.CreatedAt = row.CreatedAt.UTC()
	return &row, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd backend && go test ./internal/b2b/ -run TestOpenEnrollWindow -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/b2b/enroll_code.go backend/internal/b2b/enroll_code_test.go
git commit -m "feat(b2b): add org-scoped enrollment windows

One short-lived code sized to free seats replaces one code per PC, so a
100-machine school is a single GPO push instead of 100 manual activations."
```

---

### Task 4: Station enrollment with a seat cap that survives a stampede

**Files:**
- Create: `backend/internal/b2b/enroll.go`
- Create: `backend/internal/b2b/enroll_test.go`

**Interfaces:**
- Consumes: `EnrollCodeRow` and errors from Task 3.
- Produces:
  - `type EnrollInput struct { Code string; PublicKey ed25519.PublicKey; HWIDHash string; Label string; AgentVersion string }`
  - `type EnrollResult struct { StationID, OrgID, ProfileID uuid.UUID; Label string; LicenseEndsAt time.Time }`
  - `Store.EnrollStation(ctx context.Context, in EnrollInput) (EnrollResult, error)`

- [ ] **Step 1: Write the failing tests**

`backend/internal/b2b/enroll_test.go`:

```go
package b2b_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
)

// seatedOrg inserts an active org with `seats` classroom seats live for 30 days.
func seatedOrg(t *testing.T, pool *pgxpool.Pool, seats int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, home_seats, starts_at, ends_at, note)
		VALUES ($1, $2, 0, now(), now() + interval '30 days', 'test')`, orgID, seats); err != nil {
		t.Fatal(err)
	}
	return orgID
}

func newPub(t *testing.T) ed25519.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestEnrollStationBindsAndCapsSeats(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 2)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-1",
		Label: "PC-1", AgentVersion: "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.OrgID != orgID || first.StationID == uuid.Nil || first.ProfileID == uuid.Nil {
		t.Fatalf("bad result: %+v", first)
	}

	// The shadow profile is a station, not a learner.
	var kind, phone string
	if err := pool.QueryRow(ctx,
		`SELECT kind, phone FROM profile WHERE id = $1`, first.ProfileID).Scan(&kind, &phone); err != nil {
		t.Fatal(err)
	}
	if kind != "station" || !strings.HasPrefix(phone, "st:") {
		t.Fatalf("shadow profile kind=%q phone=%q", kind, phone)
	}

	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-2", Label: "PC-2",
	}); err != nil {
		t.Fatal(err)
	}

	// Third machine: seats are gone.
	_, err = store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-3", Label: "PC-3",
	})
	if !errors.Is(err, b2b.ErrSeatsExhausted) {
		t.Fatalf("err=%v, want ErrSeatsExhausted", err)
	}
}

func TestEnrollStationRejectsBadCodes(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 5)

	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: "AVTO-ZZZZ-ZZZZ", PublicKey: newPub(t), HWIDHash: "hw-x", Label: "PC",
	}); !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("unknown code err=%v, want ErrNotFound", err)
	}

	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET expires_at = now() - interval '1 minute' WHERE id = $1`,
		code.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-y", Label: "PC",
	}); err == nil {
		t.Fatal("expired code must be refused")
	}

	code2, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET revoked_at = now() WHERE id = $1`, code2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code2.Code, PublicKey: newPub(t), HWIDHash: "hw-z", Label: "PC",
	}); err == nil {
		t.Fatal("revoked code must be refused")
	}
}

func TestEnrollStationRebindsSameMachine(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 2)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-same", Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Re-imaged machine, same hardware, fresh key: the old bind is revoked and
	// the seat is reused rather than double-spent.
	second, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-same", Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.StationID == first.StationID {
		t.Fatal("re-enroll must create a new station row")
	}
	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != 1 {
		t.Fatalf("used=%d err=%v, want 1 active station", used, err)
	}
}

func TestEnrollStationConcurrentStampedeRespectsSeats(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	const seats = 5
	const machines = 20
	orgID := seatedOrg(t, pool, seats)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make([]error, machines)
	for i := 0; i < machines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = store.EnrollStation(ctx, b2b.EnrollInput{
				Code:      code.Code,
				PublicKey: newPub(t),
				HWIDHash:  "hw-" + uuid.NewString(),
				Label:     "PC",
			})
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != seats {
		t.Fatalf("%d enrollments succeeded, want exactly %d", ok, seats)
	}
	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != seats {
		t.Fatalf("used=%d err=%v, want %d", used, err, seats)
	}
}
```

The revoked-code case updates the row directly rather than through a
test-only exported method: adding an authz-bypassing helper to `Store` would
put a production-surface method in the codebase whose only caller is a test.

- [ ] **Step 2: Run to verify they fail**

Run: `cd backend && go test ./internal/b2b/ -run TestEnrollStation -v`
Expected: FAIL — `b2b.EnrollInput` undefined.

- [ ] **Step 3: Implement enrollment**

`backend/internal/b2b/enroll.go`:

```go
package b2b

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// EnrollInput is one classroom PC presenting an org code and the public half
// of a keypair it generated locally. The private half never leaves that
// machine, which is what makes the binding uncopyable.
type EnrollInput struct {
	Code         string
	PublicKey    ed25519.PublicKey
	HWIDHash     string
	Label        string
	AgentVersion string
}

// EnrollResult is what the agent persists after a successful enrollment.
type EnrollResult struct {
	StationID     uuid.UUID `json:"station_id"`
	OrgID         uuid.UUID `json:"org_id"`
	ProfileID     uuid.UUID `json:"-"`
	Label         string    `json:"label"`
	LicenseEndsAt time.Time `json:"license_ends_at"`
}

// maxLabelLen keeps operator-supplied PC names from bloating list responses.
const maxLabelLen = 64

func (in EnrollInput) validate() error {
	if strings.TrimSpace(in.Code) == "" {
		return fmt.Errorf("%w: code required", ErrInvalid)
	}
	if len(in.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: public_key must be %d bytes", ErrInvalid, ed25519.PublicKeySize)
	}
	h := strings.TrimSpace(in.HWIDHash)
	if len(h) != 64 {
		return fmt.Errorf("%w: hwid_hash must be a 64-char sha256 hex digest", ErrInvalid)
	}
	if _, err := hex.DecodeString(h); err != nil {
		return fmt.Errorf("%w: hwid_hash must be hex", ErrInvalid)
	}
	return nil
}

// EnrollStation binds one machine to an org under a live enrollment window.
//
// The whole check-and-insert runs inside one transaction that takes
// `SELECT ... FROM b2b_org FOR UPDATE` first. That lock is not decoration:
// seat accounting reads b2b_station and writes a new row, two statements with
// no row in common, so under READ COMMITTED a GPO rollout starting 100 agents
// at once would have every one of them read the same pre-rollout count and
// sail past a 30-seat cap. Serializing on the org row is what makes the cap
// hold.
func (s Store) EnrollStation(ctx context.Context, in EnrollInput) (EnrollResult, error) {
	if err := in.validate(); err != nil {
		return EnrollResult{}, err
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	hwid := strings.TrimSpace(in.HWIDHash)
	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = "PC"
	}
	if len(label) > maxLabelLen {
		label = label[:maxLabelLen]
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return EnrollResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		codeID    uuid.UUID
		orgID     uuid.UUID
		maxUses   int
		usedCount int
	)
	err = tx.QueryRow(ctx, `
		SELECT id, org_id, max_uses, used_count
		FROM b2b_org_enroll_code
		WHERE code = $1 AND revoked_at IS NULL AND expires_at > now()`,
		code).Scan(&codeID, &orgID, &maxUses, &usedCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EnrollResult{}, ErrNotFound
		}
		return EnrollResult{}, err
	}

	// Lock the org before any seat arithmetic.
	var orgStatus string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM b2b_org WHERE id = $1 FOR UPDATE`, orgID).Scan(&orgStatus); err != nil {
		return EnrollResult{}, err
	}
	if orgStatus != "active" {
		return EnrollResult{}, ErrOrgSuspended
	}

	// Re-read the counter under the lock; a concurrent enroll may have used it.
	if err := tx.QueryRow(ctx,
		`SELECT used_count FROM b2b_org_enroll_code WHERE id = $1`, codeID).Scan(&usedCount); err != nil {
		return EnrollResult{}, err
	}
	if usedCount >= maxUses {
		return EnrollResult{}, ErrSeatsExhausted
	}

	var seats int64
	var licenseEnds time.Time
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(seats), 0), COALESCE(MAX(ends_at), now())
		FROM b2b_org_license
		WHERE org_id = $1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&seats, &licenseEnds)
	if err != nil {
		return EnrollResult{}, err
	}
	if seats <= 0 {
		return EnrollResult{}, ErrNoLicense
	}

	// Re-imaging the same machine reuses its seat instead of burning a new one.
	if _, err := tx.Exec(ctx, `
		UPDATE b2b_station SET status = 'revoked'
		WHERE hwid_hash = $1 AND status = 'active'`, hwid); err != nil {
		return EnrollResult{}, err
	}

	var active int64
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM b2b_station WHERE org_id = $1 AND status = 'active'`, orgID).Scan(&active); err != nil {
		return EnrollResult{}, err
	}
	if active >= seats {
		return EnrollResult{}, ErrSeatsExhausted
	}

	var profileID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO profile (phone, name, kind)
		VALUES ('st:' || gen_random_uuid(), $1, 'station')
		RETURNING id`, label).Scan(&profileID); err != nil {
		return EnrollResult{}, fmt.Errorf("create shadow profile: %w", err)
	}

	var stationID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO b2b_station
		  (org_id, public_key, hwid_hash, label, status, activated_by, agent_version, station_profile_id)
		VALUES ($1, $2, $3, $4, 'active', 'enroll', $5, $6)
		RETURNING id`,
		orgID, []byte(in.PublicKey), hwid, label, in.AgentVersion, profileID).Scan(&stationID); err != nil {
		return EnrollResult{}, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET used_count = used_count + 1 WHERE id = $1`, codeID); err != nil {
		return EnrollResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EnrollResult{}, err
	}

	return EnrollResult{
		StationID:     stationID,
		OrgID:         orgID,
		ProfileID:     profileID,
		Label:         label,
		LicenseEndsAt: licenseEnds.UTC(),
	}, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd backend && go test ./internal/b2b/ -run TestEnrollStation -v -count=1`
Expected: PASS (4 tests, including the 20-goroutine stampede landing exactly 5 enrollments).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/b2b/enroll.go backend/internal/b2b/enroll_test.go backend/internal/b2b/enroll_code.go
git commit -m "feat(b2b): enroll classroom stations against an org code

Locks the org row before seat arithmetic so a simultaneous 100-machine GPO
rollout cannot overshoot a 30-seat license, and mints a shadow station
profile so the PC needs no learner account."
```

---

### Task 5: Station access tokens

**Files:**
- Modify: `backend/internal/auth/jwt.go`
- Create: `backend/internal/auth/station_jwt_test.go`
- Modify: `backend/internal/auth/middleware.go:26-42`

**Interfaces:**
- Consumes: `stationctx` from Task 2.
- Produces:
  - `auth.Claims` gains `StationID uuid.UUID` (zero for learners).
  - `auth.IssueStationAccess(secret []byte, stationID, profileID uuid.UUID, ttl time.Duration) (string, error)`
  - `auth.Required` accepts both `typ:"learner"` and `typ:"station"`, and puts the station id on the context via `stationctx.WithContext`.

- [ ] **Step 1: Write the failing test**

`backend/internal/auth/station_jwt_test.go`:

```go
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/stationctx"
)

func TestStationTokenCarriesStationID(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-000000")
	stationID, profileID := uuid.New(), uuid.New()

	token, err := auth.IssueStationAccess(secret, stationID, profileID, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := auth.ParseAccess(secret, token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.ProfileID != profileID {
		t.Fatalf("sub=%v, want shadow profile %v", claims.ProfileID, profileID)
	}
	if claims.StationID != stationID {
		t.Fatalf("sid=%v, want %v", claims.StationID, stationID)
	}
	if claims.Role != "station" {
		t.Fatalf("role=%q, want station", claims.Role)
	}
}

func TestRequiredPutsStationOnContext(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-000000")
	stationID, profileID := uuid.New(), uuid.New()
	token, err := auth.IssueStationAccess(secret, stationID, profileID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	var seen uuid.UUID
	var ok bool
	h := auth.Required(secret)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, ok = stationctx.FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !ok || seen != stationID {
		t.Fatalf("station on ctx = %v (ok=%v), want %v", seen, ok, stationID)
	}
}

func TestRequiredLeavesLearnerContextStationless(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-000000")
	token, err := auth.IssueAccess(secret, uuid.New(), "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	var ok bool
	h := auth.Required(secret)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, ok = stationctx.FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if ok {
		t.Fatal("a learner token must not put a station on the context")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/auth/ -run TestStationToken -v`
Expected: FAIL — `auth.IssueStationAccess` undefined.

- [ ] **Step 3: Implement**

In `backend/internal/auth/jwt.go`, add the station type constant next to `learnerTyp`, extend `Claims`, add the issuer and teach `ParseAccess` about it:

```go
// stationTyp is the `typ` claim on a classroom-station access token. A station
// authenticates as a machine, not a person: `sub` is its shadow profile so
// every profile-keyed subsystem keeps working, and `sid` is the station row
// that entitlement checks read.
const stationTyp = "station"

type Claims struct {
	ProfileID uuid.UUID
	Role      string
	// StationID is set only for station tokens; uuid.Nil for learners.
	StationID uuid.UUID
}

// IssueStationAccess mints a short-lived access token for a bound station.
func IssueStationAccess(secret []byte, stationID, profileID uuid.UUID, ttl time.Duration) (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  profileID.String(),
		"sid":  stationID.String(),
		"role": "station",
		"typ":  stationTyp,
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
		"jti":  uuid.NewString(),
	})
	return t.SignedString(secret)
}
```

In `ParseAccess`, replace the single-type guard with an allowlist of two and parse `sid`:

```go
	typ, _ := mc["typ"].(string)
	if typ != learnerTyp && typ != stationTyp {
		return Claims{}, fmt.Errorf("token type %q not allowed on learner routes", typ)
	}
	sub, _ := mc["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub")
	}
	role, _ := mc["role"].(string)
	out := Claims{ProfileID: id, Role: role}
	if typ == stationTyp {
		sid, _ := mc["sid"].(string)
		stationID, err := uuid.Parse(sid)
		if err != nil {
			return Claims{}, fmt.Errorf("invalid sid")
		}
		out.StationID = stationID
	}
	return out, nil
```

In `backend/internal/auth/middleware.go`, add the `avtotest.uz/backend/internal/stationctx` import and change the last line of `Required`'s handler:

```go
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			ctx = stationctx.WithContext(ctx, claims.StationID)
			next.ServeHTTP(w, r.WithContext(ctx))
```

- [ ] **Step 4: Run the tests**

Run: `cd backend && go test ./internal/auth/ -count=1`
Expected: PASS — the three new tests plus every existing auth test.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/auth
git commit -m "feat(auth): add station access tokens

A station token's sub is its shadow profile, so sessions and progress keep
working unchanged; its sid reaches entitlement through stationctx, which only
Required can write."
```

---

### Task 6: Challenge/response station login

**Files:**
- Create: `backend/internal/b2b/station_auth.go`
- Create: `backend/internal/b2b/station_auth_test.go`

**Interfaces:**
- Consumes: `auth.IssueStationAccess` (Task 5); `b2b_station` columns (Task 1).
- Produces:
  - `type StationAuth struct { Pool *pgxpool.Pool; Redis *redis.Client; Secret []byte }`
  - `StationAuth.Challenge(ctx context.Context, stationID uuid.UUID) (ChallengeResult, error)` where `type ChallengeResult struct { Nonce string; ExpiresIn int }`
  - `StationAuth.Token(ctx context.Context, in TokenInput) (TokenResult, error)` where
    `type TokenInput struct { StationID uuid.UUID; Nonce string; TS int64; Sig []byte; HWIDHash string; AgentVersion string; IP string }` and
    `type TokenResult struct { AccessToken string; ExpiresIn int; LicenseEndsAt time.Time; OrgName string }`
  - `b2b.SignedMessage(stationID uuid.UUID, nonce string, ts int64) []byte` — shared with the agent so both sides build the same bytes.
  - `var ErrStationAuth = errors.New("station auth failed")`

- [ ] **Step 1: Write the failing test**

`backend/internal/b2b/station_auth_test.go`:

```go
package b2b_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
	"avtotest.uz/backend/internal/testredis"
)

func TestStationAuthHappyPath(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := testredis.New(t)
	store := b2b.Store{Pool: pool}
	secret := []byte("test-secret-that-is-long-enough-000000")
	sa := b2b.StationAuth{Pool: pool, Redis: rdb, Secret: secret}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 3)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const hwid = "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900"
	res, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: pub, HWIDHash: hwid, Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	ch, err := sa.Challenge(ctx, res.StationID)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Unix()
	sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, ch.Nonce, ts))

	tok, err := sa.Token(ctx, b2b.TokenInput{
		StationID: res.StationID, Nonce: ch.Nonce, TS: ts, Sig: sig, HWIDHash: hwid,
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := auth.ParseAccess(secret, tok.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.StationID != res.StationID || claims.ProfileID != res.ProfileID {
		t.Fatalf("claims=%+v, want station=%v profile=%v", claims, res.StationID, res.ProfileID)
	}

	// The nonce is single use.
	sig2 := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, ch.Nonce, ts))
	if _, err := sa.Token(ctx, b2b.TokenInput{
		StationID: res.StationID, Nonce: ch.Nonce, TS: ts, Sig: sig2, HWIDHash: hwid,
	}); !errors.Is(err, b2b.ErrStationAuth) {
		t.Fatalf("replayed nonce err=%v, want ErrStationAuth", err)
	}
}

func TestStationAuthRejects(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := testredis.New(t)
	store := b2b.Store{Pool: pool}
	sa := b2b.StationAuth{Pool: pool, Redis: rdb, Secret: []byte("test-secret-that-is-long-enough-000000")}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 3)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const hwid = "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900"
	res, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: pub, HWIDHash: hwid, Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	fresh := func(t *testing.T) (string, int64) {
		t.Helper()
		ch, err := sa.Challenge(ctx, res.StationID)
		if err != nil {
			t.Fatal(err)
		}
		return ch.Nonce, time.Now().Unix()
	}

	t.Run("wrong key", func(t *testing.T) {
		nonce, ts := fresh(t)
		_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
		sig := ed25519.Sign(otherPriv, b2b.SignedMessage(res.StationID, nonce, ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("hwid mismatch", func(t *testing.T) {
		nonce, ts := fresh(t)
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, nonce, ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig,
			HWIDHash: "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("unknown nonce", func(t *testing.T) {
		ts := time.Now().Unix()
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, "not-a-nonce", ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: "not-a-nonce", TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("stale timestamp", func(t *testing.T) {
		nonce, _ := fresh(t)
		ts := time.Now().Add(-10 * time.Minute).Unix()
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, nonce, ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("revoked station", func(t *testing.T) {
		if err := store.RevokeStation(ctx, orgID, res.StationID); err != nil {
			t.Fatal(err)
		}
		nonce, ts := fresh(t)
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, nonce, ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})
}
```

- [ ] **Step 2: Check for a Redis test helper, and add one if missing**

Run: `cd backend && ls internal/testredis 2>/dev/null || grep -rln "miniredis\|redis.NewClient" internal --include=*_test.go | head`

If `internal/testredis` does not exist, create `backend/internal/testredis/testredis.go`:

```go
// Package testredis hands tests an isolated Redis database.
//
// It uses REDIS_TEST_URL (or the compose default) and picks a per-test
// logical DB, flushing it on cleanup so nonces from one test never satisfy
// another's replay check.
package testredis

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

// New returns a flushed client bound to logical DB 15.
func New(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		url = "redis://localhost:6379/15"
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse REDIS_TEST_URL: %v", err)
	}
	c := redis.NewClient(opt)
	ctx := context.Background()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable at %s: %v", url, err)
	}
	if err := c.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flushdb: %v", err)
	}
	t.Cleanup(func() {
		_ = c.FlushDB(context.Background()).Err()
		_ = c.Close()
	})
	return c
}
```

If an equivalent helper already exists, use it and adjust the test imports instead.

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/b2b/ -run TestStationAuth -v`
Expected: FAIL — `b2b.StationAuth` undefined.

- [ ] **Step 4: Implement**

`backend/internal/b2b/station_auth.go`:

```go
package b2b

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"avtotest.uz/backend/internal/auth"
)

// ErrStationAuth is the single error every station-login failure returns.
// The reasons (bad signature, wrong hardware, replayed nonce, revoked
// station) are deliberately indistinguishable to the caller: telling an
// attacker which half of the credential was wrong is free reconnaissance.
var ErrStationAuth = errors.New("station auth failed")

const (
	// stationNonceTTL bounds how long a challenge stays answerable.
	stationNonceTTL = time.Minute
	// stationTokenTTL is short because renewal is free for a live agent and
	// expensive for anyone holding a stolen token.
	stationTokenTTL = 15 * time.Minute
	// stationClockSkew is how far the agent's clock may drift from ours.
	stationClockSkew = 2 * time.Minute
	// signedMessagePrefix domain-separates these signatures from any other
	// use of the same key.
	signedMessagePrefix = "avtotest-station-v1"
)

// StationAuth verifies station identity and mints access tokens.
type StationAuth struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Secret []byte
}

// ChallengeResult is a one-shot nonce for the agent to sign.
type ChallengeResult struct {
	Nonce     string `json:"nonce"`
	ExpiresIn int    `json:"expires_in"`
}

// TokenInput is a signed challenge answer.
type TokenInput struct {
	StationID    uuid.UUID
	Nonce        string
	TS           int64
	Sig          []byte
	HWIDHash     string
	AgentVersion string
	IP           string
}

// TokenResult is a live station session.
type TokenResult struct {
	AccessToken   string    `json:"access_token"`
	ExpiresIn     int       `json:"expires_in"`
	LicenseEndsAt time.Time `json:"license_ends_at"`
	OrgName       string    `json:"org_name"`
}

// SignedMessage builds the exact bytes both the server and the agent sign.
// Any change here breaks every deployed agent, so it is versioned by prefix.
func SignedMessage(stationID uuid.UUID, nonce string, ts int64) []byte {
	return []byte(signedMessagePrefix + "|" + stationID.String() + "|" + nonce + "|" + strconv.FormatInt(ts, 10))
}

func nonceKey(stationID uuid.UUID, nonce string) string {
	return "station:nonce:" + stationID.String() + ":" + nonce
}

// Challenge issues a nonce bound to one station.
func (a StationAuth) Challenge(ctx context.Context, stationID uuid.UUID) (ChallengeResult, error) {
	if stationID == uuid.Nil {
		return ChallengeResult{}, ErrStationAuth
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return ChallengeResult{}, err
	}
	nonce := base64.RawURLEncoding.EncodeToString(buf)
	if err := a.Redis.Set(ctx, nonceKey(stationID, nonce), "1", stationNonceTTL).Err(); err != nil {
		return ChallengeResult{}, fmt.Errorf("store station nonce: %w", err)
	}
	return ChallengeResult{Nonce: nonce, ExpiresIn: int(stationNonceTTL.Seconds())}, nil
}

// Token verifies a signed challenge and mints a station access token.
func (a StationAuth) Token(ctx context.Context, in TokenInput) (TokenResult, error) {
	if in.StationID == uuid.Nil || in.Nonce == "" || len(in.Sig) != ed25519.SignatureSize {
		return TokenResult{}, ErrStationAuth
	}

	skew := time.Since(time.Unix(in.TS, 0))
	if skew < -stationClockSkew || skew > stationClockSkew {
		return TokenResult{}, ErrStationAuth
	}

	// DEL returns the number of keys removed, so claiming the nonce and
	// checking it existed is one atomic step — two agents replaying the same
	// nonce cannot both pass.
	claimed, err := a.Redis.Del(ctx, nonceKey(in.StationID, in.Nonce)).Result()
	if err != nil {
		return TokenResult{}, fmt.Errorf("claim station nonce: %w", err)
	}
	if claimed != 1 {
		return TokenResult{}, ErrStationAuth
	}

	var (
		pub       []byte
		hwid      string
		profileID uuid.UUID
		orgName   string
		ends      time.Time
	)
	err = a.Pool.QueryRow(ctx, `
		SELECT s.public_key, s.hwid_hash, s.station_profile_id, o.name, MAX(l.ends_at)
		FROM b2b_station s
		JOIN b2b_org o ON o.id = s.org_id AND o.status = 'active'
		JOIN b2b_org_license l ON l.org_id = s.org_id
		  AND l.starts_at <= now() AND l.ends_at > now()
		WHERE s.id = $1 AND s.status = 'active'
		GROUP BY s.public_key, s.hwid_hash, s.station_profile_id, o.name`,
		in.StationID).Scan(&pub, &hwid, &profileID, &orgName, &ends)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TokenResult{}, ErrStationAuth
		}
		return TokenResult{}, err
	}
	if hwid != in.HWIDHash {
		return TokenResult{}, ErrStationAuth
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), SignedMessage(in.StationID, in.Nonce, in.TS), in.Sig) {
		return TokenResult{}, ErrStationAuth
	}

	token, err := auth.IssueStationAccess(a.Secret, in.StationID, profileID, stationTokenTTL)
	if err != nil {
		return TokenResult{}, err
	}

	_, _ = a.Pool.Exec(ctx, `
		UPDATE b2b_station
		SET last_seen_at = now(),
		    agent_version = COALESCE(NULLIF($2, ''), agent_version),
		    last_ip = NULLIF($3, '')::inet
		WHERE id = $1`, in.StationID, in.AgentVersion, in.IP)

	return TokenResult{
		AccessToken:   token,
		ExpiresIn:     int(stationTokenTTL.Seconds()),
		LicenseEndsAt: ends.UTC(),
		OrgName:       orgName,
	}, nil
}
```

- [ ] **Step 5: Run the tests**

Run: `cd backend && go test ./internal/b2b/ -run TestStationAuth -v -count=1`
Expected: PASS (2 top-level tests, 5 subtests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/b2b/station_auth.go backend/internal/b2b/station_auth_test.go backend/internal/testredis
git commit -m "feat(b2b): verify stations by signed challenge

Nonce claim is an atomic Redis DEL, so a replay cannot race a live login,
and every rejection returns one indistinguishable error."
```

---

### Task 7: Station HTTP endpoints

**Files:**
- Create: `backend/internal/b2b/station_handlers.go`
- Create: `backend/internal/b2b/station_handlers_test.go`
- Modify: `backend/internal/b2b/handlers.go` — add `PublicRoutes` and the teacher enroll-window routes
- Modify: `backend/internal/server/server.go` — wire `PublicRoutes`

**Interfaces:**
- Consumes: `Store.EnrollStation` (Task 4), `StationAuth` (Task 6), enroll-window methods (Task 3), `auth.Limiter` (existing).
- Produces these routes under `/api/v1`:
  - `POST /b2b/stations/enroll` — body `{code, public_key (base64), hwid_hash, label, agent_version}` → `{station_id, org_id, label, license_ends_at}`
  - `POST /b2b/stations/challenge` — body `{station_id}` → `{nonce, expires_in}`
  - `POST /b2b/stations/token` — body `{station_id, nonce, ts, sig (base64), hwid_hash, agent_version}` → `{access_token, expires_in, license_ends_at, org_name}`
  - `POST /me/teacher/orgs/{id}/enroll-window` → `EnrollCodeRow`
  - `GET /me/teacher/orgs/{id}/enroll-window` → `EnrollCodeRow` or `null`
  - `DELETE /me/teacher/orgs/{id}/enroll-window/{codeID}` → 204

- [ ] **Step 1: Write the failing test**

`backend/internal/b2b/station_handlers_test.go`:

```go
package b2b_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
	"avtotest.uz/backend/internal/testredis"
)

func TestStationEndpointsEndToEnd(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := testredis.New(t)
	ctx := context.Background()
	secret := []byte("test-secret-that-is-long-enough-000000")

	store := b2b.Store{Pool: pool}
	orgID := seatedOrg(t, pool, 2)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	h := &b2b.Handler{Pool: pool, Redis: rdb, Secret: secret, Lim: auth.Limiter{R: rdb}}
	r := chi.NewRouter()
	r.Route("/api/v1", h.PublicRoutes)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	post := func(t *testing.T, path string, body any) (*http.Response, map[string]any) {
		t.Helper()
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(srv.URL+path, "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp, out
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const hwid = "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900"

	resp, body := post(t, "/api/v1/b2b/stations/enroll", map[string]any{
		"code":          code.Code,
		"public_key":    base64.StdEncoding.EncodeToString(pub),
		"hwid_hash":     hwid,
		"label":         "PC-1",
		"agent_version": "1.0.0",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enroll status=%d body=%v", resp.StatusCode, body)
	}
	data, _ := body["data"].(map[string]any)
	stationID, _ := data["station_id"].(string)
	if stationID == "" {
		t.Fatalf("no station_id in %v", body)
	}

	resp, body = post(t, "/api/v1/b2b/stations/challenge", map[string]any{"station_id": stationID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("challenge status=%d body=%v", resp.StatusCode, body)
	}
	data, _ = body["data"].(map[string]any)
	nonce, _ := data["nonce"].(string)
	if nonce == "" {
		t.Fatalf("no nonce in %v", body)
	}

	sid, err := uuidParse(stationID)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Unix()
	sig := ed25519.Sign(priv, b2b.SignedMessage(sid, nonce, ts))

	resp, body = post(t, "/api/v1/b2b/stations/token", map[string]any{
		"station_id": stationID,
		"nonce":      nonce,
		"ts":         ts,
		"sig":        base64.StdEncoding.EncodeToString(sig),
		"hwid_hash":  hwid,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token status=%d body=%v", resp.StatusCode, body)
	}
	data, _ = body["data"].(map[string]any)
	accessToken, _ := data["access_token"].(string)
	claims, err := auth.ParseAccess(secret, accessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.StationID != sid {
		t.Fatalf("claims.StationID=%v, want %v", claims.StationID, sid)
	}

	// A wrong hwid on an otherwise valid flow is a flat 401.
	resp, body = post(t, "/api/v1/b2b/stations/challenge", map[string]any{"station_id": stationID})
	data, _ = body["data"].(map[string]any)
	nonce2, _ := data["nonce"].(string)
	ts2 := time.Now().Unix()
	sig2 := ed25519.Sign(priv, b2b.SignedMessage(sid, nonce2, ts2))
	resp, _ = post(t, "/api/v1/b2b/stations/token", map[string]any{
		"station_id": stationID, "nonce": nonce2, "ts": ts2,
		"sig":       base64.StdEncoding.EncodeToString(sig2),
		"hwid_hash": "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("hwid mismatch status=%d, want 401", resp.StatusCode)
	}
}
```

Add this helper at the bottom of the same file (the package already imports `github.com/google/uuid` in other test files, but each file needs its own import):

```go
func uuidParse(s string) (uuid.UUID, error) { return uuid.Parse(s) }
```

and add `"github.com/google/uuid"` to this file's import block.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/b2b/ -run TestStationEndpointsEndToEnd -v`
Expected: FAIL — `Handler` has no field `Redis` / no method `PublicRoutes`.

- [ ] **Step 3: Implement the handlers**

Extend the `Handler` struct in `backend/internal/b2b/handlers.go`:

```go
type Handler struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Secret []byte
	Lim    auth.Limiter
}
```

(Keep the existing fields it already has; add only the missing ones. Import `github.com/redis/go-redis/v9` and `avtotest.uz/backend/internal/auth`.)

Add the teacher routes to `AuthedRoutes`:

```go
	r.Post("/me/teacher/orgs/{id}/enroll-window", h.openEnrollWindow)
	r.Get("/me/teacher/orgs/{id}/enroll-window", h.getEnrollWindow)
	r.Delete("/me/teacher/orgs/{id}/enroll-window/{codeID}", h.closeEnrollWindow)
```

`backend/internal/b2b/station_handlers.go`:

```go
package b2b

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// PublicRoutes are the three unauthenticated station endpoints. They are
// public by necessity — a classroom PC has no session until it has proved it
// holds a station key — so each one is rate limited by client IP.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Post("/b2b/stations/enroll", h.enrollStation)
	r.Post("/b2b/stations/challenge", h.stationChallenge)
	r.Post("/b2b/stations/token", h.stationToken)
}

func (h *Handler) stationAuth() StationAuth {
	return StationAuth{Pool: h.Pool, Redis: h.Redis, Secret: h.Secret}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
}

// allow applies a fixed-window IP limit; a missing limiter (tests) allows.
func (h *Handler) allow(r *http.Request, action string, limit int, window time.Duration) bool {
	if h.Lim.R == nil {
		return true
	}
	ok, err := h.Lim.Allow(r.Context(), "station:"+action+":"+clientIP(r), limit, window)
	return err == nil && ok
}

type enrollStationBody struct {
	Code         string `json:"code"`
	PublicKey    string `json:"public_key"`
	HWIDHash     string `json:"hwid_hash"`
	Label        string `json:"label"`
	AgentVersion string `json:"agent_version"`
}

func (h *Handler) enrollStation(w http.ResponseWriter, r *http.Request) {
	if !h.allow(r, "enroll", 60, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many enrollment attempts")
		return
	}
	var body enrollStationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	pub, err := base64.StdEncoding.DecodeString(body.PublicKey)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "public_key must be base64")
		return
	}
	out, err := h.store().EnrollStation(r.Context(), EnrollInput{
		Code:         body.Code,
		PublicKey:    pub,
		HWIDHash:     body.HWIDHash,
		Label:        body.Label,
		AgentVersion: body.AgentVersion,
	})
	if err != nil {
		writeStoreErr(w, err, "enrollment failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type stationChallengeBody struct {
	StationID string `json:"station_id"`
}

func (h *Handler) stationChallenge(w http.ResponseWriter, r *http.Request) {
	if !h.allow(r, "challenge", 600, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many challenge requests")
		return
	}
	var body stationChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	stationID, err := uuid.Parse(body.StationID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid station id")
		return
	}
	out, err := h.stationAuth().Challenge(r.Context(), stationID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "server_error", "challenge failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type stationTokenBody struct {
	StationID    string `json:"station_id"`
	Nonce        string `json:"nonce"`
	TS           int64  `json:"ts"`
	Sig          string `json:"sig"`
	HWIDHash     string `json:"hwid_hash"`
	AgentVersion string `json:"agent_version"`
}

func (h *Handler) stationToken(w http.ResponseWriter, r *http.Request) {
	if !h.allow(r, "token", 600, time.Hour) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many token requests")
		return
	}
	var body stationTokenBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "invalid json")
		return
	}
	stationID, err := uuid.Parse(body.StationID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid station id")
		return
	}
	sig, err := base64.StdEncoding.DecodeString(body.Sig)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "sig must be base64")
		return
	}
	out, err := h.stationAuth().Token(r.Context(), TokenInput{
		StationID:    stationID,
		Nonce:        body.Nonce,
		TS:           body.TS,
		Sig:          sig,
		HWIDHash:     body.HWIDHash,
		AgentVersion: body.AgentVersion,
		IP:           clientIP(r),
	})
	if err != nil {
		if errors.Is(err, ErrStationAuth) {
			httpx.Error(w, http.StatusUnauthorized, "station_unauthorized", "station authentication failed")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "server_error", "token failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type enrollWindowBody struct {
	TTLMinutes int `json:"ttl_minutes"`
}

func (h *Handler) openEnrollWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	var body enrollWindowBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	ttl := time.Duration(body.TTLMinutes) * time.Minute
	out, err := h.store().OpenEnrollWindowAsTeacher(r.Context(), claims.ProfileID, orgID, ttl)
	if err != nil {
		writeStoreErr(w, err, "open enroll window failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getEnrollWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	out, err := h.store().ActiveEnrollCodeAsTeacher(r.Context(), claims.ProfileID, orgID)
	if err != nil {
		writeStoreErr(w, err, "enroll window query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) closeEnrollWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	codeID, err := uuid.Parse(chi.URLParam(r, "codeID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid code id")
		return
	}
	if err := h.store().CloseEnrollWindowAsTeacher(r.Context(), claims.ProfileID, orgID, codeID); err != nil {
		writeStoreErr(w, err, "close enroll window failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Extend `writeStoreErr` in `backend/internal/b2b/handlers.go` so the new domain errors map cleanly (keep the existing cases and add):

```go
	case errors.Is(err, ErrNoLicense):
		httpx.Error(w, http.StatusConflict, "no_license", "no active classroom license")
	case errors.Is(err, ErrStationAuth):
		httpx.Error(w, http.StatusUnauthorized, "station_unauthorized", "station authentication failed")
```

- [ ] **Step 4: Wire the routes in the server**

In `backend/internal/server/server.go`, inside the `r.Route("/api/v1", func(api chi.Router) {` block, replace the existing `tb2b := &b2b.Handler{Pool: deps.Pool}` construction with one that carries the new dependencies, and mount the public routes before the authed ones:

```go
			b2bH := &b2b.Handler{
				Pool:   deps.Pool,
				Redis:  deps.Redis,
				Secret: []byte(cfg.JWTSecret),
				Lim:    auth.Limiter{R: deps.Redis},
			}
			if deps.Pool != nil && deps.Redis != nil {
				b2bH.PublicRoutes(api)
			}
```

and further down, where the learner-authed handlers are registered, use the same value:

```go
				b2bH.AuthedRoutes(learnerAuth)
```

removing the old `tb2b` variable entirely.

- [ ] **Step 5: Run the tests**

```bash
cd backend && go test ./internal/b2b/ ./internal/server/ -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/b2b backend/internal/server
git commit -m "feat(b2b): expose station enroll, challenge and token endpoints

All three are public because a classroom PC has no session before it proves
key possession, so each carries a per-IP rate limit."
```

---

### Task 8: Keep station profiles out of learner-facing aggregates

**Files:**
- Modify: `backend/internal/db/queries/leaderboard.sql:1-32`
- Create: `backend/internal/leaderboard/station_exclusion_test.go`
- Modify: `backend/internal/admin/` user-list query — find it with the grep in Step 1

**Interfaces:**
- Consumes: `profile.kind` from Task 1.
- Produces: no new exported symbols; `CountCorrectAnswersByProfileInRange` and `CountCorrectAnswersByProfileByDayInRange` skip station profiles.

- [ ] **Step 1: Find every place that lists profiles**

```bash
cd backend && grep -rn "FROM profile\|JOIN profile" internal/db/queries internal/admin --include=*.sql --include=*.go
```

Every query that produces a **learner-facing list or ranking** needs `kind = 'user'`. Queries that look up a single profile by id (`GetProfileByID`, `LockProfileForGrant`) must **not** be filtered — a station's own requests resolve through them.

- [ ] **Step 2: Write the failing test**

`backend/internal/leaderboard/station_exclusion_test.go`:

```go
package leaderboard_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// A classroom PC racks up correct answers all day. It must never appear on a
// learner leaderboard.
func TestStationProfilesAreNotRanked(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	ctx := context.Background()

	var stationProfile uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO profile (phone, name, kind)
		VALUES ('st:' || gen_random_uuid(), 'PC-1', 'station') RETURNING id`).Scan(&stationProfile); err != nil {
		t.Fatal(err)
	}
	seedCorrectAnswers(t, pool, stationProfile, 5)

	from := time.Now().Add(-24 * time.Hour)
	to := time.Now().Add(time.Hour)
	rows, err := q.CountCorrectAnswersByProfileInRange(ctx, sqlc.CountCorrectAnswersByProfileInRangeParams{
		FromTs: pgTimestamptz(from), ToTs: pgTimestamptz(to),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ProfileID == stationProfile {
			t.Fatal("station profile leaked into the leaderboard aggregate")
		}
	}
}
```

Add these helpers at the bottom of the same file, with `"github.com/jackc/pgx/v5/pgtype"` and `"github.com/jackc/pgx/v5/pgxpool"` in the imports. If `backend/internal/leaderboard/` already has equivalents, use those instead of duplicating:

```go
func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// seedCorrectAnswers gives profileID n correct answers in a finished practice
// session, building the minimum content rows the FKs demand.
func seedCorrectAnswers(t *testing.T, pool *pgxpool.Pool, profileID uuid.UUID, n int) {
	t.Helper()
	ctx := context.Background()

	var categoryID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO category (code) VALUES ('cat-' || gen_random_uuid()) RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}

	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO exam_session (profile_id, mode, locale, total, status)
		VALUES ($1, 'practice', 'uz-Latn', $2, 'passed') RETURNING id`,
		profileID, n).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < n; i++ {
		var questionID, answerID uuid.UUID
		if err := pool.QueryRow(ctx, `
			INSERT INTO question (source_ext_id, category_id, content_hash)
			VALUES ('q-' || gen_random_uuid(), $1, 'hash') RETURNING id`,
			categoryID).Scan(&questionID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `
			INSERT INTO answer (question_id, position, is_correct)
			VALUES ($1, 1, true) RETURNING id`, questionID).Scan(&answerID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			UPDATE question SET correct_answer_id = $2 WHERE id = $1`, questionID, answerID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO session_answer (session_id, question_id, answer_id, is_correct, position)
			VALUES ($1, $2, $3, true, 1)`, sessionID, questionID, answerID); err != nil {
			t.Fatal(err)
		}
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/leaderboard/ -run TestStationProfilesAreNotRanked -v`
Expected: FAIL — the station profile appears in the aggregate.

- [ ] **Step 4: Filter the queries**

In `backend/internal/db/queries/leaderboard.sql`, add a profile join to both aggregate queries. `CountCorrectAnswersByProfileInRange` becomes:

```sql
SELECT
  es.profile_id,
  count(*)::int AS correct_count,
  max(sa.answered_at)::timestamptz AS last_answered_at
FROM session_answer sa
JOIN exam_session es ON es.id = sa.session_id
JOIN profile p ON p.id = es.profile_id AND p.kind = 'user'
WHERE sa.is_correct
  AND sa.answered_at >= sqlc.arg(from_ts)
  AND sa.answered_at < sqlc.arg(to_ts)
GROUP BY es.profile_id;
```

and `CountCorrectAnswersByProfileByDayInRange`:

```sql
SELECT
  es.profile_id,
  date_trunc('day', sa.answered_at AT TIME ZONE 'UTC')::timestamptz AS day,
  count(*)::int AS correct_count,
  max(sa.answered_at)::timestamptz AS last_answered_at
FROM session_answer sa
JOIN exam_session es ON es.id = sa.session_id
JOIN profile p ON p.id = es.profile_id AND p.kind = 'user'
WHERE sa.is_correct
  AND sa.answered_at >= sqlc.arg(from_ts)
  AND sa.answered_at < sqlc.arg(to_ts)
GROUP BY es.profile_id, date_trunc('day', sa.answered_at AT TIME ZONE 'UTC');
```

Apply the same `kind = 'user'` filter to the admin user-list query found in Step 1.

- [ ] **Step 5: Expose `kind` on GET /me**

The kiosk shell in Task 9 has to tell a station apart from a learner, and `role` cannot carry that: a shadow profile's `role` column is the default `'user'`, only its `kind` says otherwise. Add the field to `profileDTO` in `backend/internal/account/handlers.go:33-45`:

```go
	Role         string  `json:"role"`
	Kind         string  `json:"kind"`
```

and to `toProfileDTO` (line 47-66):

```go
		Role:         p.Role,
		Kind:         p.Kind,
```

`p.Kind` exists on `sqlc.Profile` after `make generate` picks up the new column.

- [ ] **Step 6: Regenerate sqlc and run the tests**

```bash
make generate
cd backend && go test ./internal/leaderboard/ ./internal/admin/ ./internal/account/ -count=1
```

Expected: PASS. If `make generate` reports drift in unrelated files, commit only the leaderboard/admin/account outputs.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/db backend/internal/leaderboard backend/internal/admin backend/internal/account
git commit -m "fix(leaderboard): exclude station shadow profiles from rankings

A classroom PC answers all day under one profile; ranking it would bury every
real learner."
```

---

### Task 9: Teacher enrollment UI and the kiosk shell

**Files:**
- Modify: `frontend/src/app/[locale]/(app)/teacher/page.tsx`
- Create: `frontend/src/app/[locale]/(app)/station/page.tsx`
- Create: `frontend/src/app/api/proxy` passthrough — none needed; existing proxy covers it
- Modify: `frontend/messages/uz-Latn.json`, `frontend/messages/uz-Cyrl.json`, `frontend/messages/ru.json`
- Create: `frontend/src/app/[locale]/(app)/teacher/enroll-window.test.tsx`

**Interfaces:**
- Consumes: `POST/GET/DELETE /me/teacher/orgs/{id}/enroll-window` (Task 7); the `kind` field on `GET /me` (Task 8).
- Produces: `EnrollWindowPanel({ orgId }: { orgId: string })` as the default export of `enroll-window-panel.tsx`; the `/station` route.

- [ ] **Step 1: Read the existing teacher page and its API helper**

```bash
sed -n 1,80p frontend/src/app/[locale]/\(app\)/teacher/page.tsx
sed -n 1,40p frontend/src/lib/api-client.ts
```

Follow the fetch/error/loading conventions already in that page. Do not introduce a new data-fetching library.

- [ ] **Step 2: Write the failing test**

`frontend/src/app/[locale]/(app)/teacher/enroll-window.test.tsx`:

```tsx
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import EnrollWindowPanel from "./enroll-window-panel";

const apiGet = vi.fn();
const apiPost = vi.fn();
const apiDelete = vi.fn();

vi.mock("@/lib/api-client", () => ({
  apiGet: (...args: unknown[]) => apiGet(...args),
  apiPost: (...args: unknown[]) => apiPost(...args),
  apiDelete: (...args: unknown[]) => apiDelete(...args),
}));

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
}));

describe("EnrollWindowPanel", () => {
  beforeEach(() => {
    apiGet.mockReset();
    apiPost.mockReset();
    apiDelete.mockReset();
  });

  it("opens a window and shows the code", async () => {
    apiGet.mockResolvedValue(null);
    apiPost.mockResolvedValue({
      id: "code-1",
      code: "AVTO-ABCD-EFGH",
      max_uses: 30,
      used_count: 0,
      expires_at: new Date(Date.now() + 7_200_000).toISOString(),
    });

    render(<EnrollWindowPanel orgId="org-1" />);

    await waitFor(() => expect(screen.getByText("enrollNone")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "enrollOpen" }));

    await waitFor(() => expect(screen.getByText("AVTO-ABCD-EFGH")).toBeInTheDocument());
    expect(apiPost).toHaveBeenCalledWith("me/teacher/orgs/org-1/enroll-window", { ttl_minutes: 120 });
  });

  it("closes an open window", async () => {
    apiGet.mockResolvedValue({
      id: "code-1",
      code: "AVTO-ABCD-EFGH",
      max_uses: 30,
      used_count: 4,
      expires_at: new Date(Date.now() + 7_200_000).toISOString(),
    });
    apiDelete.mockResolvedValue(undefined);

    render(<EnrollWindowPanel orgId="org-1" />);

    await waitFor(() => expect(screen.getByText("AVTO-ABCD-EFGH")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "enrollClose" }));

    await waitFor(() =>
      expect(apiDelete).toHaveBeenCalledWith("me/teacher/orgs/org-1/enroll-window/code-1"),
    );
  });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd frontend && npm run test -- enroll-window`
Expected: FAIL — `./enroll-window-panel` does not exist.

- [ ] **Step 4: Build the teacher enrollment panel**

The panel is its own file rather than more state inside `teacher/page.tsx`: that page already carries orgs, members, stats, invites and stations, and enrollment is a self-contained concern with its own fetch cycle.

`frontend/src/app/[locale]/(app)/teacher/enroll-window-panel.tsx`:

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiDelete, apiGet, apiPost } from "@/lib/api-client";
import { Button } from "@/components/ui/button";

type EnrollWindow = {
  id: string;
  code: string;
  max_uses: number;
  used_count: number;
  expires_at: string;
};

/** Minutes left until iso, floored at 0. */
function minutesLeft(iso: string): number {
  return Math.max(0, Math.floor((new Date(iso).getTime() - Date.now()) / 60_000));
}

export default function EnrollWindowPanel({ orgId }: { orgId: string }) {
  const t = useTranslations("Teacher");
  const [window, setWindow] = useState<EnrollWindow | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [minutes, setMinutes] = useState(0);

  const load = useCallback(async () => {
    try {
      setWindow(await apiGet<EnrollWindow | null>(`me/teacher/orgs/${orgId}/enroll-window`));
    } catch {
      setError(t("errorLoad"));
    }
  }, [orgId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  // Recompute the countdown once a minute, without re-fetching: the window's
  // expiry is fixed at creation, so only the clock moves.
  useEffect(() => {
    if (!window) return;
    setMinutes(minutesLeft(window.expires_at));
    const id = setInterval(() => setMinutes(minutesLeft(window.expires_at)), 60_000);
    return () => clearInterval(id);
  }, [window]);

  const open = async () => {
    setBusy(true);
    setError(null);
    try {
      setWindow(
        await apiPost<EnrollWindow>(`me/teacher/orgs/${orgId}/enroll-window`, { ttl_minutes: 120 }),
      );
    } catch {
      setError(t("enrollError"));
    } finally {
      setBusy(false);
    }
  };

  const close = async () => {
    if (!window) return;
    setBusy(true);
    setError(null);
    try {
      await apiDelete(`me/teacher/orgs/${orgId}/enroll-window/${window.id}`);
      setWindow(null);
    } catch {
      setError(t("enrollError"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="rounded-lg border p-4">
      <h2 className="mb-2 text-lg font-semibold">{t("enrollTitle")}</h2>

      {window ? (
        <div className="space-y-3">
          <p className="select-all font-mono text-3xl tracking-widest">{window.code}</p>
          <p className="text-sm opacity-80">
            {t("enrollUsed", { used: window.used_count, max: window.max_uses })}
          </p>
          <p className="text-sm opacity-80">
            {t("enrollExpires", { minutes })}
          </p>
          <p className="text-sm opacity-70">{t("enrollHint", { max: window.max_uses })}</p>
          <Button variant="outline" onClick={close} disabled={busy}>
            {t("enrollClose")}
          </Button>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-sm opacity-80">{t("enrollNone")}</p>
          <Button onClick={open} disabled={busy}>
            {t("enrollOpen")}
          </Button>
        </div>
      )}

      {error ? <p className="mt-3 text-sm text-red-500">{error}</p> : null}
    </section>
  );
}
```

Render it from `teacher/page.tsx` where the deleted activation form used to sit, passing the selected org:

```tsx
{selected ? <EnrollWindowPanel orgId={selected.org.id} /> : null}
```

Also drop the now-dead `Station.fingerprint` field from the `Station` type (line 60) and the `stationLabel` / `activateCode` / `lastCode` state that served the removed form.

New keys in the existing `Teacher` namespace of all three locale files. `uz-Latn.json`:

```json
"enrollTitle": "PC ulash oynasi",
"enrollOpen": "Oynani ochish (2 soat)",
"enrollClose": "Oynani yopish",
"enrollNone": "Hozir ochiq oyna yo'q",
"enrollUsed": "{used} / {max} PC ulandi",
"enrollExpires": "{minutes} daqiqadan keyin tugaydi",
"enrollHint": "Har bir kompyuterda dasturni o'rnating va shu kodni kiriting. Kod {max} tagacha PC uchun amal qiladi.",
"enrollError": "Amal bajarilmadi"
```

`uz-Cyrl.json`:

```json
"enrollTitle": "PC улаш ойнаси",
"enrollOpen": "Ойнани очиш (2 соат)",
"enrollClose": "Ойнани ёпиш",
"enrollNone": "Ҳозир очиқ ойна йўқ",
"enrollUsed": "{used} / {max} PC уланди",
"enrollExpires": "{minutes} дақиқадан кейин тугайди",
"enrollHint": "Ҳар бир компьютерда дастурни ўрнатинг ва шу кодни киритинг. Код {max} тагача PC учун амал қилади.",
"enrollError": "Амал бажарилмади"
```

`ru.json`:

```json
"enrollTitle": "Окно подключения ПК",
"enrollOpen": "Открыть окно (2 часа)",
"enrollClose": "Закрыть окно",
"enrollNone": "Открытого окна нет",
"enrollUsed": "Подключено {used} / {max} ПК",
"enrollExpires": "Истекает через {minutes} мин.",
"enrollHint": "Установите программу на каждом компьютере и введите этот код. Код действует до {max} ПК.",
"enrollError": "Не удалось выполнить"
```

- [ ] **Step 5: Build the kiosk shell**

`frontend/src/app/[locale]/(app)/station/page.tsx`:

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";
import { Button } from "@/components/ui/button";

// `kind` is the only field that distinguishes a station: a shadow profile's
// `role` is the default "user" like any learner's.
type Me = { kind: string; name: string };

export default function StationPage() {
  const t = useTranslations("Station");
  const locale = useLocale();
  const [me, setMe] = useState<Me | null>(null);
  const [checked, setChecked] = useState(false);
  // Bumping this key remounts the entry screen, which is what "next student"
  // means here: no server state to reset, just a clean screen.
  const [sitting, setSitting] = useState(0);

  const load = useCallback(async () => {
    try {
      setMe(await apiGet<Me>("me"));
    } catch {
      setMe(null);
    } finally {
      setChecked(true);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (!checked) return null;

  if (!me || me.kind !== "station") {
    return <p className="p-8 text-center text-lg">{t("notStation")}</p>;
  }

  return (
    <main key={sitting} className="mx-auto max-w-2xl space-y-6 p-8">
      <h1 className="text-3xl font-bold">{t("title")}</h1>

      <div className="grid gap-4 sm:grid-cols-2">
        <Link href={`/${locale}/practice`} className="rounded-lg border p-6 text-center text-xl">
          {t("practice")}
        </Link>
        <Link href={`/${locale}/exam`} className="rounded-lg border p-6 text-center text-xl">
          {t("exam")}
        </Link>
      </div>

      <Button className="w-full py-6 text-xl" onClick={() => setSitting((n) => n + 1)}>
        {t("newStudent")}
      </Button>
    </main>
  );
}
```

Before writing the `Link` targets, confirm the real practice and exam route names:

```bash
ls frontend/src/app/\[locale\]/\(app\)/
```

Use whatever those directories are actually called rather than the placeholders above.

New `Station` namespace in all three locale files. `uz-Latn.json`:

```json
"Station": {
  "title": "Sinfxona",
  "practice": "Mashq qilish",
  "exam": "Imtihon",
  "newStudent": "Yangi o'quvchi",
  "notStation": "Bu sahifa faqat sinfxona kompyuteri uchun"
}
```

`uz-Cyrl.json`:

```json
"Station": {
  "title": "Синфхона",
  "practice": "Машқ қилиш",
  "exam": "Имтиҳон",
  "newStudent": "Янги ўқувчи",
  "notStation": "Бу саҳифа фақат синфхона компьютери учун"
}
```

`ru.json`:

```json
"Station": {
  "title": "Класс",
  "practice": "Тренировка",
  "exam": "Экзамен",
  "newStudent": "Новый ученик",
  "notStation": "Эта страница только для компьютера класса"
}
```

- [ ] **Step 6: Run the frontend gate**

```bash
make fe-check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend
git commit -m "feat(b2b): teacher enrollment window and kiosk shell

One code panel replaces per-PC activation, and /station is a login-free
classroom entry point."
```

---

### Task 10: The station agent

**Files:**
- Create: `station/go.mod` (module `avtotest.uz/station`)
- Create: `station/cmd/avtotest-station/main.go`
- Create: `station/internal/hwid/hwid.go`, `hwid_windows.go`, `hwid_other.go`
- Create: `station/internal/keystore/keystore.go`, `keystore_windows.go`, `keystore_other.go`
- Create: `station/internal/agent/agent.go`, `agent_test.go`
- Create: `station/internal/proxy/proxy.go`, `proxy_test.go`
- Create: `station/internal/kiosk/launch.go`
- Create: `station/README.md`

**Interfaces:**
- Consumes: `POST /api/v1/b2b/stations/{enroll,challenge,token}` (Task 7); the message format `avtotest-station-v1|<station_id>|<nonce>|<unix_ts>`.
- Produces:
  - `hwid.Collect() (string, error)` — 64-char lowercase sha256 hex
  - `keystore.Store` interface `{ Load() (ed25519.PrivateKey, error); Save(ed25519.PrivateKey) error }`, constructor `keystore.Open(dir string) (Store, error)`
  - `agent.Agent` with `Enroll(ctx context.Context, code, label string) error` and `Token(ctx context.Context) (string, error)`
  - `proxy.New(frontendBase, apiBase string, token func(context.Context) (string, error)) http.Handler`

- [ ] **Step 1: Create the module**

```bash
mkdir -p station/cmd/avtotest-station station/internal/{hwid,keystore,agent,proxy,kiosk}
cd station && go mod init avtotest.uz/station && go get golang.org/x/sys@latest
```

The agent is a separate module on purpose: it ships to school PCs and must not drag the backend's pgx, chi and Redis dependencies into a binary that talks only HTTP.

- [ ] **Step 2: Write the failing proxy test**

`station/internal/proxy/proxy_test.go`:

```go
package proxy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"avtotest.uz/station/internal/proxy"
)

func TestProxyInjectsStationTokenOnAPICalls(t *testing.T) {
	var gotAuth, gotPath string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"data":{"ok":true}}`)
	}))
	t.Cleanup(api.Close)

	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "PAGE "+r.URL.Path)
	}))
	t.Cleanup(front.Close)

	h := proxy.New(front.URL, api.URL, func(context.Context) (string, error) {
		return "station-token-123", nil
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	// The browser keeps calling the frontend's proxy path; the agent rewrites
	// it onto the API and attaches the station token.
	resp, err := http.Get(srv.URL + "/api/proxy/me")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if gotAuth != "Bearer station-token-123" {
		t.Fatalf("Authorization=%q, want the station token", gotAuth)
	}
	if gotPath != "/api/v1/me" {
		t.Fatalf("upstream path=%q, want /api/v1/me", gotPath)
	}

	// Everything else is the Next.js app, untouched.
	resp2, err := http.Get(srv.URL + "/uz/station")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	body, _ := io.ReadAll(resp2.Body)
	if string(body) != "PAGE /uz/station" {
		t.Fatalf("body=%q, want the frontend page", body)
	}
}

func TestProxyFailsClosedWithoutToken(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(api.Close)

	h := proxy.New("http://127.0.0.1:1", api.URL, func(context.Context) (string, error) {
		return "", context.DeadlineExceeded
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/api/proxy/me")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503 when no station token is available", resp.StatusCode)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd station && go test ./internal/proxy/ -v`
Expected: FAIL — package `proxy` does not exist.

- [ ] **Step 4: Implement the proxy**

`station/internal/proxy/proxy.go`:

```go
// Package proxy serves the classroom browser from 127.0.0.1.
//
// Everything the page requests is same-origin, so there is no CORS to
// negotiate and no cookie to steal. API calls are rewritten from the
// frontend's /api/proxy/* path onto the backend's /api/v1/* and signed with
// the station token — which is why the token never reaches JavaScript.
package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// apiPrefix is the path the Next.js client already calls; keeping it means the
// web app needs no station-specific branch.
const apiPrefix = "/api/proxy/"

// New routes API calls to apiBase with a station token attached and every
// other request to frontendBase untouched.
func New(frontendBase, apiBase string, token func(context.Context) (string, error)) http.Handler {
	frontURL, err := url.Parse(frontendBase)
	if err != nil {
		panic("proxy: bad frontend base: " + err.Error())
	}
	apiURL, err := url.Parse(apiBase)
	if err != nil {
		panic("proxy: bad api base: " + err.Error())
	}

	frontProxy := httputil.NewSingleHostReverseProxy(frontURL)

	apiProxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = apiURL.Scheme
			r.URL.Host = apiURL.Host
			r.Host = apiURL.Host
			r.URL.Path = "/api/v1/" + strings.TrimPrefix(r.URL.Path, apiPrefix)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, apiPrefix) {
			frontProxy.ServeHTTP(w, r)
			return
		}
		tok, err := token(r.Context())
		if err != nil || tok == "" {
			// Fail closed: serving an unauthenticated API call would silently
			// downgrade the classroom to the free tier mid-lesson.
			http.Error(w, `{"error":{"code":"station_offline","message":"station token unavailable"}}`,
				http.StatusServiceUnavailable)
			return
		}
		r.Header.Set("Authorization", "Bearer "+tok)
		apiProxy.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 5: Run the proxy tests**

Run: `cd station && go test ./internal/proxy/ -v`
Expected: PASS (2 tests).

- [ ] **Step 6: Write the failing agent test**

`station/internal/agent/agent_test.go`:

```go
package agent_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/keystore"
)

func TestEnrollThenTokenSignsTheChallenge(t *testing.T) {
	dir := t.TempDir()
	const hwid = "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900"
	const stationID = "11111111-2222-3333-4444-555555555555"
	const nonce = "test-nonce"

	var pub ed25519.PublicKey
	var verified bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/b2b/stations/enroll":
			var body struct {
				PublicKey string `json:"public_key"`
				HWIDHash  string `json:"hwid_hash"`
				Code      string `json:"code"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.HWIDHash != hwid || body.Code != "AVTO-TEST-CODE" {
				t.Errorf("bad enroll body: %+v", body)
			}
			raw, err := base64.StdEncoding.DecodeString(body.PublicKey)
			if err != nil {
				t.Errorf("public_key not base64: %v", err)
			}
			pub = ed25519.PublicKey(raw)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"station_id": stationID, "org_id": stationID, "label": "PC-1"},
			})
		case "/api/v1/b2b/stations/challenge":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"nonce": nonce, "expires_in": 60},
			})
		case "/api/v1/b2b/stations/token":
			var body struct {
				StationID string `json:"station_id"`
				Nonce     string `json:"nonce"`
				TS        int64  `json:"ts"`
				Sig       string `json:"sig"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			sig, err := base64.StdEncoding.DecodeString(body.Sig)
			if err != nil {
				t.Errorf("sig not base64: %v", err)
			}
			msg := []byte("avtotest-station-v1|" + body.StationID + "|" + body.Nonce + "|" + strconv.FormatInt(body.TS, 10))
			verified = ed25519.Verify(pub, msg, sig)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"access_token": "tok-abc", "expires_in": 900},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{
		APIBase: srv.URL,
		StateDir: dir,
		Keys:    ks,
		HWID:    hwid,
		Version: "test",
	}

	ctx := context.Background()
	if err := a.Enroll(ctx, "AVTO-TEST-CODE", "PC-1"); err != nil {
		t.Fatal(err)
	}
	tok, err := a.Token(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "tok-abc" {
		t.Fatalf("token=%q, want tok-abc", tok)
	}
	if !verified {
		t.Fatal("server could not verify the agent's signature")
	}

	// A second call inside the TTL reuses the cached token: no new challenge.
	if _, err := a.Token(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestAgentSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := ks.Load()
	if err != nil {
		t.Fatal(err)
	}
	ks2, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	priv2, err := ks2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !priv.Equal(priv2) {
		t.Fatal("keystore must return the same key across restarts")
	}
}
```

- [ ] **Step 7: Run to verify it fails**

Run: `cd station && go test ./internal/agent/ -v`
Expected: FAIL — packages `agent` and `keystore` do not exist.

- [ ] **Step 8: Implement the keystore**

`station/internal/keystore/keystore.go`:

```go
// Package keystore holds the station's Ed25519 private key.
//
// On Windows the key is sealed with DPAPI at machine scope, so the file is
// undecryptable on any other machine — copying it to a home PC yields
// nothing. The non-Windows implementation is a plain 0600 file for
// development only and refuses to look like the real thing.
package keystore

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
)

// Store loads or creates the station key.
type Store interface {
	Load() (ed25519.PrivateKey, error)
	Save(ed25519.PrivateKey) error
}

// ErrCorruptKey means the stored key could not be unsealed — usually because
// the file was copied from a different machine.
var ErrCorruptKey = errors.New("station key could not be unsealed on this machine")

type fileStore struct {
	path string
	seal func([]byte) ([]byte, error)
	open func([]byte) ([]byte, error)
}

// Open returns the platform keystore rooted at dir, creating dir if needed.
func Open(dir string) (Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &fileStore{
		path: filepath.Join(dir, "station.key"),
		seal: seal,
		open: unseal,
	}, nil
}

// Load returns the existing key, generating and persisting one on first run.
func (s *fileStore) Load() (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		plain, err := s.open(raw)
		if err != nil {
			return nil, ErrCorruptKey
		}
		if len(plain) != ed25519.PrivateKeySize {
			return nil, ErrCorruptKey
		}
		return ed25519.PrivateKey(plain), nil
	case errors.Is(err, os.ErrNotExist):
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		if err := s.Save(priv); err != nil {
			return nil, err
		}
		return priv, nil
	default:
		return nil, err
	}
}

// Save seals and writes the key.
func (s *fileStore) Save(priv ed25519.PrivateKey) error {
	sealed, err := s.seal(priv)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, sealed, 0o600)
}
```

`station/internal/keystore/keystore_windows.go`:

```go
//go:build windows

package keystore

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// cryptProtectLocalMachine ties the ciphertext to this machine rather than to
// the interactive user — the agent runs as a service, and the classroom PC
// has no logged-in operator.
const cryptProtectLocalMachine = 0x4

func seal(plain []byte) ([]byte, error) {
	in := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, cryptProtectLocalMachine, &out); err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) }()
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

func unseal(sealed []byte) ([]byte, error) {
	in := windows.DataBlob{Size: uint32(len(sealed)), Data: &sealed[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, cryptProtectLocalMachine, &out); err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) }()
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
```

`station/internal/keystore/keystore_other.go`:

```go
//go:build !windows

package keystore

// seal and unseal are identity functions off Windows. This build is for
// development only: without DPAPI the key file is copyable, so a non-Windows
// agent must never be handed to a school.
func seal(plain []byte) ([]byte, error) { return plain, nil }

func unseal(sealed []byte) ([]byte, error) { return sealed, nil }
```

- [ ] **Step 9: Implement HWID collection**

`station/internal/hwid/hwid.go`:

```go
// Package hwid derives a stable identifier for the physical machine.
//
// It is the second half of the binding: even if the sealed key were somehow
// extracted, the server refuses a token whose hwid does not match the one
// recorded at enrollment, so a cloned disk image authenticates nowhere.
package hwid

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrNoIdentity means no machine-specific value could be read.
var ErrNoIdentity = errors.New("no stable hardware identity available")

// Collect returns a 64-char lowercase sha256 hex digest of the machine's
// identifying values.
func Collect() (string, error) {
	parts, err := rawParts()
	if err != nil {
		return "", err
	}
	kept := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, strings.ToLower(p))
		}
	}
	if len(kept) == 0 {
		return "", ErrNoIdentity
	}
	sum := sha256.Sum256([]byte(strings.Join(kept, "|")))
	return hex.EncodeToString(sum[:]), nil
}
```

`station/internal/hwid/hwid_windows.go`:

```go
//go:build windows

package hwid

import "golang.org/x/sys/windows/registry"

// rawParts reads MachineGuid, the install-time machine identity Windows keeps
// stable across reboots and user changes but regenerates on reimage.
func rawParts() ([]string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return nil, err
	}
	defer func() { _ = k.Close() }()
	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return nil, err
	}
	return []string{"machineguid:" + guid}, nil
}
```

`station/internal/hwid/hwid_other.go`:

```go
//go:build !windows

package hwid

import "os"

// rawParts reads /etc/machine-id. Development only — see keystore_other.go.
func rawParts() ([]string, error) {
	b, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return nil, err
	}
	return []string{"machine-id:" + string(b)}, nil
}
```

- [ ] **Step 10: Implement the agent**

`station/internal/agent/agent.go`:

```go
// Package agent talks to the AvtoTest backend on behalf of one classroom PC.
package agent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"avtotest.uz/station/internal/keystore"
)

// tokenRenewMargin renews before expiry so a lesson never stalls on a
// round-trip that could have happened a minute earlier.
const tokenRenewMargin = 2 * time.Minute

// Agent holds one station's identity and its cached access token.
type Agent struct {
	APIBase  string
	StateDir string
	Keys     keystore.Store
	HWID     string
	Version  string
	HTTP     *http.Client

	mu        sync.Mutex
	token     string
	tokenTill time.Time
	state     State
	loaded    bool
}

// State is what survives a restart.
type State struct {
	StationID string `json:"station_id"`
	OrgID     string `json:"org_id"`
	Label     string `json:"label"`
}

// ErrNotEnrolled means this PC has never been bound to a school.
var ErrNotEnrolled = errors.New("station is not enrolled")

func (a *Agent) client() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (a *Agent) statePath() string { return filepath.Join(a.StateDir, "station.json") }

func (a *Agent) loadState() error {
	if a.loaded {
		return nil
	}
	b, err := os.ReadFile(a.statePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			a.loaded = true
			return nil
		}
		return err
	}
	if err := json.Unmarshal(b, &a.state); err != nil {
		return err
	}
	a.loaded = true
	return nil
}

func (a *Agent) saveState() error {
	b, err := json.Marshal(a.state)
	if err != nil {
		return err
	}
	return os.WriteFile(a.statePath(), b, 0o600)
}

// post sends a JSON body and decodes the {"data": ...} envelope into out.
func (a *Agent) post(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.APIBase+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	var env struct {
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("%s: bad response (%d)", path, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		if env.Error != nil {
			return fmt.Errorf("%s: %s (%s)", path, env.Error.Message, env.Error.Code)
		}
		return fmt.Errorf("%s: status %d", path, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// Enroll binds this machine to a school using a one-time org code.
func (a *Agent) Enroll(ctx context.Context, code, label string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	priv, err := a.Keys.Load()
	if err != nil {
		return err
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return errors.New("station key is not ed25519")
	}

	var out struct {
		StationID string `json:"station_id"`
		OrgID     string `json:"org_id"`
		Label     string `json:"label"`
	}
	err = a.post(ctx, "/api/v1/b2b/stations/enroll", map[string]any{
		"code":          code,
		"public_key":    base64.StdEncoding.EncodeToString(pub),
		"hwid_hash":     a.HWID,
		"label":         label,
		"agent_version": a.Version,
	}, &out)
	if err != nil {
		return err
	}
	a.state = State{StationID: out.StationID, OrgID: out.OrgID, Label: out.Label}
	a.loaded = true
	return a.saveState()
}

// Token returns a live station access token, renewing it when it is close to
// expiring.
func (a *Agent) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.loadState(); err != nil {
		return "", err
	}
	if a.state.StationID == "" {
		return "", ErrNotEnrolled
	}
	if a.token != "" && time.Until(a.tokenTill) > tokenRenewMargin {
		return a.token, nil
	}

	var ch struct {
		Nonce string `json:"nonce"`
	}
	if err := a.post(ctx, "/api/v1/b2b/stations/challenge",
		map[string]any{"station_id": a.state.StationID}, &ch); err != nil {
		return "", err
	}

	priv, err := a.Keys.Load()
	if err != nil {
		return "", err
	}
	ts := time.Now().Unix()
	msg := []byte("avtotest-station-v1|" + a.state.StationID + "|" + ch.Nonce + "|" + strconv.FormatInt(ts, 10))
	sig := ed25519.Sign(priv, msg)

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	err = a.post(ctx, "/api/v1/b2b/stations/token", map[string]any{
		"station_id":    a.state.StationID,
		"nonce":         ch.Nonce,
		"ts":            ts,
		"sig":           base64.StdEncoding.EncodeToString(sig),
		"hwid_hash":     a.HWID,
		"agent_version": a.Version,
	}, &tok)
	if err != nil {
		return "", err
	}
	a.token = tok.AccessToken
	a.tokenTill = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	return a.token, nil
}
```

- [ ] **Step 11: Run the agent tests**

Run: `cd station && go test ./... -count=1`
Expected: PASS.

- [ ] **Step 12: Implement the kiosk launcher and main**

`station/internal/kiosk/launch.go`:

```go
// Package kiosk starts the local browser in full-screen kiosk mode.
package kiosk

import (
	"errors"
	"os/exec"
	"runtime"
)

// ErrNoBrowser means neither Chrome nor Edge was found.
var ErrNoBrowser = errors.New("no supported browser found (install Google Chrome or Microsoft Edge)")

// candidates lists browsers in preference order.
func candidates() []string {
	if runtime.GOOS == "windows" {
		return []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		}
	}
	return []string{"google-chrome", "chromium", "chromium-browser"}
}

// Launch opens url in kiosk mode and returns the running process.
func Launch(url string) (*exec.Cmd, error) {
	for _, bin := range candidates() {
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		cmd := exec.Command(path,
			"--kiosk",
			"--app="+url,
			"--no-first-run",
			"--disable-session-crashed-bubble",
			"--disable-features=TranslateUI",
		)
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return cmd, nil
	}
	return nil, ErrNoBrowser
}
```

`station/cmd/avtotest-station/main.go`:

```go
// Command avtotest-station runs one classroom PC: it holds the station key,
// keeps an access token live, serves the browser from localhost and opens the
// kiosk.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/hwid"
	"avtotest.uz/station/internal/keystore"
	"avtotest.uz/station/internal/kiosk"
	"avtotest.uz/station/internal/proxy"
)

// version is stamped at build time: go build -ldflags "-X main.version=1.0.0"
var version = "dev"

func main() {
	var (
		code     = flag.String("code", "", "one-time org enrollment code (first run only)")
		label    = flag.String("label", "", "PC name shown to the school (default: hostname)")
		apiBase  = flag.String("api", "https://api.avtotest.uz", "backend base URL")
		frontend = flag.String("frontend", "https://avtotest.uz", "frontend base URL")
		addr     = flag.String("addr", "127.0.0.1:17817", "local listen address")
		stateDir = flag.String("state", defaultStateDir(), "directory for the station key and state")
		noKiosk  = flag.Bool("no-kiosk", false, "serve only; do not launch a browser")
	)
	flag.Parse()

	id, err := hwid.Collect()
	if err != nil {
		log.Fatalf("hardware id: %v", err)
	}
	keys, err := keystore.Open(*stateDir)
	if err != nil {
		log.Fatalf("keystore: %v", err)
	}
	name := *label
	if name == "" {
		name, _ = os.Hostname()
	}

	a := &agent.Agent{APIBase: *apiBase, StateDir: *stateDir, Keys: keys, HWID: id, Version: version}
	ctx := context.Background()

	if _, err := a.Token(ctx); errors.Is(err, agent.ErrNotEnrolled) {
		if *code == "" {
			log.Fatal("this PC is not enrolled yet: run again with -code AVTO-XXXX-XXXX")
		}
		if err := a.Enroll(ctx, *code, name); err != nil {
			log.Fatalf("enrollment failed: %v", err)
		}
		log.Printf("enrolled as %q", name)
		if _, err := a.Token(ctx); err != nil {
			log.Fatalf("first token failed: %v", err)
		}
	} else if err != nil {
		log.Fatalf("station token: %v", err)
	}

	handler := proxy.New(*frontend, *apiBase, a.Token)
	url := fmt.Sprintf("http://%s/uz/station", *addr)

	if !*noKiosk {
		if _, err := kiosk.Launch(url); err != nil {
			log.Printf("kiosk launch: %v (open %s manually)", err, url)
		}
	}
	log.Printf("avtotest-station %s serving %s", version, url)
	log.Fatal(http.ListenAndServe(*addr, handler))
}

// defaultStateDir keeps the key beside the program data, not in a user
// profile, because the agent runs as a machine service.
func defaultStateDir() string {
	if dir := os.Getenv("ProgramData"); dir != "" {
		return filepath.Join(dir, "AvtoTest", "station")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ".avtotest-station"
	}
	return filepath.Join(dir, "avtotest-station")
}
```

- [ ] **Step 13: Write the agent README**

`station/README.md` must document: what the agent is, the one-line install for IT staff (`avtotest-station.exe -code AVTO-XXXX-XXXX`), where the key and state live (`%ProgramData%\AvtoTest\station`), that the key is DPAPI machine-scoped and copying it is useless, what happens when a PC is re-imaged (re-run with a fresh code; the old bind is revoked and the seat is reused), and that non-Windows builds are development-only because they do not seal the key.

- [ ] **Step 14: Build for Windows and run the whole gate**

```bash
cd station && GOOS=windows GOARCH=amd64 go build ./... && go vet ./... && go test ./... -count=1
cd .. && make check && make fe-check
```

Expected: all PASS. The Windows cross-build is what proves the `//go:build windows` files compile — no test runs them on Linux.

- [ ] **Step 15: Commit**

```bash
git add station
git commit -m "feat(station): add the classroom PC agent

Holds a DPAPI-sealed key, keeps a station token live, proxies the browser
from localhost so JavaScript never sees the token, and opens Chrome in kiosk
mode."
```

---

## Faza 1 Definition of Done

- `make check` and `make fe-check` pass.
- `cd station && GOOS=windows go build ./... && go test ./...` passes.
- A 20-goroutine enrollment stampede against a 5-seat license lands exactly 5 stations.
- No code path grants VIP from a request header; `grep -rn "devicefp\|X-Device-Fingerprint" backend frontend` returns nothing.
- One org code enrolls N PCs, and PC N+1 is refused once seats are gone.

## Deferred to Faza 2 (separate plan)

Signed 72-hour offline lease, clock-rollback guard, content cache in the agent proxy, offline result queue, and the `lease` field on the token response.

## Deferred to Faza 3 (separate plan)

MSI/GPO silent installer, agent auto-update, concurrent-IP anomaly detection, teacher-facing per-PC statistics.
