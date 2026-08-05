# B2B Admin Installer — Plan 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An admin downloads one ready-to-run `.exe` per driving school; running it on a classroom PC enrols that PC, installs itself and opens the kiosk on every boot — with nothing typed and no person attached to the school.

**Architecture:** The membership/roles/invite/home-seat machinery and the `/teacher` portal are deleted; the admin panel becomes the only control surface. Each org gets one long-lived installer key whose expiry tracks its licence. The station agent is cross-compiled into the API image at build time; the download endpoint streams that binary with a JSON config appended to its tail, which the agent reads back from its own file on first run.

**Tech Stack:** Go 1.26 (pgx v5, chi, golang-jwt v5, `golang.org/x/sys/windows`), PostgreSQL, Next.js 15 (App Router, TypeScript, next-intl), Vitest, Docker multi-stage build.

**Spec:** `docs/superpowers/specs/2026-08-06-b2b-admin-installer-design.md`

## Global Constraints

- Plan 1 only. The kiosk section expansion (signs, exam, stats, saved, leaderboard) is **Plan 2** — do not build it here.
- Offline tolerance is **not** being built, now or later in this plan. No cache, no lease, no result queue.
- Arena, mistakes and grand mock stay out of the kiosk. Do not add them.
- The `b2b` package uses raw pgx through `b2b.Store{Pool}`, **not** sqlc. Only `backend/internal/db/queries/*.sql` changes require `make generate`.
- Installer key expiry = the org's latest live licence `ends_at`. There is no fixed TTL cap.
- Config trailer layout, exact: `[base bytes][config JSON][uint32 big-endian JSON length][16-byte magic]`. Magic is exactly `AVTOSTATIONCFG01`.
- Every user-facing string must exist in all three locale files: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json` — real translations, not Latin text pasted into the Cyrillic and Russian ones.
- Backend gate: `make check`. Frontend gate: `make fe-check`. Postgres must be up: `make up`. The full backend suite runs under `-p 1` and takes 10-15 minutes; the shell tool caps around 10 minutes, so prefer focused package runs while iterating.
- Run every command in the foreground. Do NOT launch background jobs, monitors, or `&`-detached processes.
- Commit after every task. Never commit a failing gate.
- **Three test steps (Tasks 5, 7 and 8) list the required cases and name the file to copy the harness from, rather than giving literal code.** That is deliberate: those tests sit on repo-specific fixtures — the admin auth harness in `backend/internal/admin/b2b_test.go` and the Vitest component mocks in `frontend/src/app/[locale]/(kiosk)/station/page.test.tsx` — and inventing code that does not match them would be worse than a precise case list. Read the named file first, then write every listed case out in full. A case list is not permission to write fewer tests.

---

### Task 1: Delete the membership, roles, invites and home seats

Demolition and migration in one commit so the tree is never half-migrated.

**Files:**
- Create: `backend/internal/db/migrations/0058_b2b_drop_membership.up.sql`
- Create: `backend/internal/db/migrations/0058_b2b_drop_membership.down.sql`
- Modify: `backend/internal/b2b/handlers.go` — delete `AuthedRoutes` and every handler it names
- Modify: `backend/internal/b2b/store.go` — delete `teacherRole`, `requireOwner`, member/invite functions
- Modify: `backend/internal/b2b/enroll_code.go` — delete the three `AsTeacher` wrappers
- Modify: `backend/internal/b2b/station.go` — delete `ActiveHomeSeats`, `ListStationsAsTeacher`, `RevokeStationAsTeacher`, `RenameStationAsTeacher`
- Modify: `backend/internal/admin/b2b_handlers.go` — delete `addB2BMember`, `grantB2BMember`, `inviteB2BMember`, `removeB2BMember`, `changeB2BMemberRole`
- Modify: `backend/internal/admin/b2b.go` — delete member/invite queries; drop `home_seats` from licence create and org detail
- Modify: `backend/internal/admin/handlers.go` — delete the five member/invite routes
- Modify: `backend/internal/server/server.go` — drop the `b2b.Handler.AuthedRoutes` mount
- Modify: `backend/internal/testdb/testdb.go` — drop `b2b_invite`, `b2b_org_member` from TRUNCATE
- Delete: `frontend/src/app/[locale]/(app)/teacher/` (whole directory)
- Modify: `frontend/src/app/[locale]/admin/(shell)/b2b/orgs/[id]/page.tsx` — delete member list, add-member, invite and grant sections
- Delete: `frontend/src/app/api/admin/b2b/orgs/[id]/members/` and `.../invites/` (whole directories)
- Modify: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json` — delete the `Teacher` namespace and the orphaned `AdminB2B` member keys
- Modify: `backend/internal/b2b/*_test.go`, `backend/internal/admin/b2b_test.go` — delete tests for deleted functions

**Interfaces:**
- Consumes: nothing.
- Produces: `b2b_org`, `b2b_org_license` (without `home_seats`), `b2b_station`, `b2b_org_enroll_code` — the only B2B tables left.

- [ ] **Step 1: Write the migration**

`backend/internal/db/migrations/0058_b2b_drop_membership.up.sql`:

```sql
-- Admin-only B2B: a school is a licence plus N stations. Nobody is attached to
-- it, so membership, invites and the never-used home-seat SKU all go.

DROP TABLE b2b_invite;
DROP TABLE b2b_org_member;

ALTER TABLE b2b_org_license DROP COLUMN home_seats;
```

`backend/internal/db/migrations/0058_b2b_drop_membership.down.sql`:

```sql
-- Recreates the shape only. The rows are gone for good: this migration dropped
-- the tables that held them, and nothing archived the contents first.

ALTER TABLE b2b_org_license
  ADD COLUMN home_seats int NOT NULL DEFAULT 0 CHECK (home_seats >= 0);

CREATE TABLE b2b_org_member (
  org_id     uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  profile_id uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  role       text NOT NULL DEFAULT 'student'
             CHECK (role IN ('owner', 'teacher', 'student')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, profile_id)
);
CREATE INDEX b2b_org_member_profile_idx ON b2b_org_member(profile_id);
```

Read `backend/internal/db/migrations/0033_b2b_org.up.sql` for `b2b_invite`'s original shape and reproduce it in the down file the same way.

- [ ] **Step 2: Delete the backend teacher portal**

In `backend/internal/b2b/handlers.go`, delete the whole `AuthedRoutes` method and every handler it references: `listOrgs`, `getOrg`, `inviteMember`, `listInvites`, `removeMember`, `changeRole`, `listLicenses`, `orgStats`, `exportCSV`, `listStations`, `revokeStation`, `renameStation`, `openEnrollWindow`, `getEnrollWindow`, `closeEnrollWindow`, `myInvites`, `acceptInvite`, and their request-body types. Keep `PublicRoutes`, `store()`, `writeStoreErr`, `clientIP` and everything `station_handlers.go` uses.

In `backend/internal/server/server.go`, delete the `b2bH.AuthedRoutes(learnerAuth)` line. Keep the `b2bH.PublicRoutes(api)` mount and the handler construction.

In `backend/internal/b2b/store.go`, delete `teacherRole`, `requireOwner` and every member/invite function. In `enroll_code.go` delete `OpenEnrollWindowAsTeacher`, `CloseEnrollWindowAsTeacher`, `ActiveEnrollCodeAsTeacher`. In `station.go` delete `ActiveHomeSeats`, `ListStationsAsTeacher`, `RevokeStationAsTeacher`, `RenameStationAsTeacher`.

Delete the tests covering those functions — in `enroll_code_test.go` the teacher-wrapper and role-gate tests, and any station test calling the `AsTeacher` variants. Keep every test for `OpenEnrollWindow`, `EnrollStation`, `StationAuth` and the seat cap.

- [ ] **Step 3: Delete the admin member surface**

In `backend/internal/admin/handlers.go`, delete these five route lines:

```go
br.Post("/b2b/orgs/{id}/members", h.addB2BMember)
br.Post("/b2b/orgs/{id}/invites", h.inviteB2BMember)
br.Post("/b2b/orgs/{id}/members/{profileID}/grant", h.grantB2BMember)
br.Delete("/b2b/orgs/{id}/members/{profileID}", h.removeB2BMember)
br.Patch("/b2b/orgs/{id}/members/{profileID}", h.changeB2BMemberRole)
```

Delete those five handlers from `b2b_handlers.go` and their bodies/types. In `b2b.go` delete the member and invite queries, and remove `home_seats` from the licence-create statement, the org-detail query and the CSV export. The org detail and stats currently report a member count — remove that field rather than leaving it hard-coded to zero, and update `backend/internal/admin/b2b_test.go` accordingly.

- [ ] **Step 4: Delete the frontend teacher portal and admin member UI**

```bash
rm -r "frontend/src/app/[locale]/(app)/teacher"
rm -r "frontend/src/app/api/admin/b2b/orgs/[id]/members"
rm -r "frontend/src/app/api/admin/b2b/orgs/[id]/invites"
```

In `frontend/src/app/[locale]/admin/(shell)/b2b/orgs/[id]/page.tsx` delete the member table, the add-member form, the invite-by-phone form, the grant button, the role `<select>`, and every piece of state and handler that exists only for them (`addMember`, `inviteByPhone`, `changeRole`, `removeMember`, `grantMember`, `profileID`, `invitePhone`). Delete `home_seats` from the licence form and from the displayed org detail. Keep the licence list, the station list and its revoke button.

Delete the `Teacher` namespace from all three locale files, and the `AdminB2B` keys that no longer have a caller. Removing a key that is still referenced fails `make fe-check` at build time, which is the check that proves this step is complete.

- [ ] **Step 5: Update the test-database truncate list**

In `backend/internal/testdb/testdb.go`, remove `b2b_invite` and `b2b_org_member` from the `TRUNCATE TABLE` list. Leaving a dropped table there makes every test in every package fail with a confusing error.

- [ ] **Step 6: Run the full gates**

```bash
make up
make test-db-reset
make check
make fe-check
```

Expected: both PASS. `make test-db-reset` is required because migration 0058 changes shape; stale per-package test databases would keep the dropped tables.

- [ ] **Step 7: Verify nothing survived**

```bash
grep -rn "teacherRole\|requireOwner\|b2b_org_member\|b2b_invite\|home_seats\|AsTeacher" backend/internal frontend/src
```

Expected: matches only inside `backend/internal/db/migrations/` (the 0033/0055 history and 0058's own down file). Anything else is a leftover.

- [ ] **Step 8: Commit**

```bash
git add -A backend frontend
git commit -m "feat(b2b)!: drop membership, roles, invites and home seats

Getting a PC onto a school licence required a person: someone registered, an
admin attached them to the org, changed their role from the hardcoded student,
and only then could open an enrolment window from /teacher. That chain broke on
the first real school, and roles bought nothing at this scale. The admin panel
already listed and revoked stations, so /teacher held exactly one unique
button. All of it goes; the admin panel becomes the only control surface."
```

---

### Task 2: Move the station module under backend/ and build the exe in the API image

**Files:**
- Move: `station/` → `backend/station/` (whole directory, `git mv`)
- Modify: `backend/Dockerfile`
- Modify: `backend/station/README.md` — fix the paths in its build instructions

**Interfaces:**
- Consumes: nothing.
- Produces: `/station/avtotest-station.exe` inside the API image; the station module importable as `avtotest.uz/station` from `backend/station/`.

- [ ] **Step 1: Move the module**

```bash
git mv station backend/station
```

The API image's build context is `../backend` (see `deploy/docker-compose.prod.yml` and `deploy/docker-compose.app.yml`), so a module at the repo root is invisible to it. Moving it in is the smallest change that works: the module keeps its own `go.mod`, so the shipped binary still carries none of the backend's pgx/chi/redis dependencies — that separation is about the module, not the directory.

The alternative — repointing the build context at the repo root, or using Compose `additional_contexts` — would touch two compose files, need a new root `.dockerignore`, and make the build context carry the whole frontend. Not worth it.

- [ ] **Step 2: Verify the module still builds from its new home**

```bash
cd backend/station && go test ./... -count=1
cd backend/station && GOOS=windows GOARCH=amd64 go build ./...
cd backend/station && go vet ./... && gofmt -l .
```

Expected: all pass, `gofmt -l` prints nothing. Nothing imports the module by path, so the move needs no import rewrites — confirm with `grep -rn "avtotest.uz/station" backend --include=*.go` returning only files inside `backend/station/`.

- [ ] **Step 3: Cross-compile the agent in the API image**

In `backend/Dockerfile`, add the station build to the existing `RUN go build` chain in the build stage:

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/encryptpan ./cmd/encryptpan \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck

# The classroom agent ships to school PCs from the admin panel, so the API image
# carries a ready-to-serve Windows build. Cross-compiled here rather than
# committed as a binary or copied onto the server by hand: either of those
# drifts from the source, which is exactly how deploy/nginx-drivergo.uz.conf
# ended up describing an architecture production never ran.
RUN cd station \
 && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath \
      -ldflags="-s -w -X main.version=${STATION_VERSION:-1.0.0}" \
      -o /out/avtotest-station.exe ./cmd/avtotest-station
```

and copy it into the final stage next to the other binaries:

```dockerfile
COPY --from=build /out/avtotest-station.exe /station/avtotest-station.exe
```

Note `backend/.dockerignore` excludes `**/*_test.go` and `*.md`, so the station tests and README are not in the build context. The build only needs the non-test sources, so that is fine.

- [ ] **Step 4: Build the image and confirm the binary is inside**

```bash
docker build -t avtotest-api:plantest -f backend/Dockerfile backend/
docker run --rm --entrypoint /station/avtotest-station.exe avtotest-api:plantest -h 2>&1 | head -3 || true
```

The `docker run` will fail — it is a Windows binary on a Linux image, and the image is distroless. That failure is expected; what matters is that it fails with an exec-format error rather than "no such file", which proves the file is present. If you prefer an unambiguous check, use:

```bash
docker create --name plantest avtotest-api:plantest && \
  docker cp plantest:/station/avtotest-station.exe /tmp/from-image.exe && \
  docker rm plantest && \
  file /tmp/from-image.exe
```

Expected: `PE32+ executable ... MS Windows`.

- [ ] **Step 5: Commit**

```bash
git add -A backend
git commit -m "build(station): move the agent under backend/ and ship it in the API image

The API image builds from context ../backend, so a module at the repo root was
invisible to it. Moving the directory keeps the separate go.mod -- and so the
dependency isolation that module boundary exists for -- while letting the image
cross-compile the Windows agent it now has to serve."
```

---

### Task 3: License-bound installer key

**Files:**
- Modify: `backend/internal/b2b/enroll_code.go`
- Modify: `backend/internal/b2b/enroll_code_test.go`

**Interfaces:**
- Consumes: `Store.LicenseEndsAt(ctx, orgID) (*time.Time, error)` (exists in `station.go`), `EnrollCodeRow`, `ErrNotFound`, `ErrOrgSuspended`, `ErrNoLicense`, `ErrSeatsExhausted`.
- Produces:
  - `Store.ActiveInstallerKey(ctx context.Context, orgID uuid.UUID) (*EnrollCodeRow, error)` — live key or `(nil, nil)`
  - `Store.OpenInstallerKey(ctx context.Context, orgID uuid.UUID, createdBy string) (EnrollCodeRow, error)` — returns the live key unchanged if one exists, else mints one
  - `Store.RotateInstallerKey(ctx context.Context, orgID uuid.UUID, createdBy string) (EnrollCodeRow, error)` — revokes the live key, mints a new one

- [ ] **Step 1: Write the failing tests**

Add to `backend/internal/b2b/enroll_code_test.go`:

```go
func TestOpenInstallerKeyIsIdempotent(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 30, now(), now() + interval '365 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	first, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}
	if first.MaxUses != 30 {
		t.Fatalf("max_uses=%d, want 30", first.MaxUses)
	}
	// Expiry tracks the licence, not a fixed TTL: a 365-day licence must not
	// yield a code that dies before a 30-PC rollout finishes.
	if d := time.Until(first.ExpiresAt); d < 300*24*time.Hour {
		t.Fatalf("expires in %v, want ~365 days", d)
	}

	second, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}
	if second.Code != first.Code || second.ID != first.ID {
		t.Fatalf("second call minted a new key (%s) instead of reusing %s", second.Code, first.Code)
	}
}

func TestRotateInstallerKeyRevokesTheOldOne(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 5, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	old, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := store.RotateInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Code == old.Code {
		t.Fatal("rotate returned the same code")
	}

	// The old key must no longer enrol.
	_, err = store.EnrollStation(ctx, b2b.EnrollInput{
		Code: old.Code, PublicKey: newPub(t), HWIDHash: testHWID("rotate-old"), Label: "PC",
	})
	if !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("old code err=%v, want ErrNotFound", err)
	}
	// The new one must.
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: fresh.Code, PublicKey: newPub(t), HWIDHash: testHWID("rotate-new"), Label: "PC",
	}); err != nil {
		t.Fatalf("new code failed to enrol: %v", err)
	}
}

func TestActiveInstallerKeyNilWithoutOne(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	row, err := store.ActiveInstallerKey(ctx, orgID)
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if row != nil {
		t.Fatalf("row=%+v, want nil", row)
	}
}

func TestOpenInstallerKeyNeedsALiveLicense(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenInstallerKey(ctx, orgID, "admin:test"); !errors.Is(err, b2b.ErrNoLicense) {
		t.Fatalf("err=%v, want ErrNoLicense", err)
	}
}
```

`newPub` and `testHWID` already exist in `backend/internal/b2b/enroll_test.go` (same `b2b_test` package) — do not redefine them.

- [ ] **Step 2: Run to verify they fail**

Run: `cd backend && go test ./internal/b2b/ -run TestOpenInstallerKey -v`
Expected: FAIL — `store.OpenInstallerKey` undefined.

- [ ] **Step 3: Rework `enroll_code.go`**

Delete `defaultEnrollTTL` and `maxEnrollTTL`. Change the internal creator to take an absolute expiry, and add the three exported methods:

```go
// mintEnrollCode revokes any live code and inserts a fresh one expiring at
// expiresAt. The caller holds the org row lock.
func mintEnrollCode(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, expiresAt time.Time, maxUses int, createdBy string) (EnrollCodeRow, error) {
	if _, err := tx.Exec(ctx, `
		UPDATE b2b_org_enroll_code SET revoked_at = now()
		WHERE org_id = $1 AND revoked_at IS NULL AND expires_at > now()`, orgID); err != nil {
		return EnrollCodeRow{}, err
	}
	code, err := newEnrollCode()
	if err != nil {
		return EnrollCodeRow{}, err
	}
	return insertEnrollCode(ctx, tx, orgID, code, maxUses, expiresAt, createdBy)
}

// installerKeyTx is the shared body of OpenInstallerKey and RotateInstallerKey.
// reuse=true returns a live key untouched; reuse=false always mints a new one.
//
// The org row is locked before any seat arithmetic for the same reason
// EnrollStation locks it: the free-seat count feeds max_uses, and two
// concurrent admins would otherwise both size a key against a stale count.
func (s Store) installerKeyTx(ctx context.Context, orgID uuid.UUID, createdBy string, reuse bool) (EnrollCodeRow, error) {
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
	var licenseEnds time.Time
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(seats), 0), COALESCE(MAX(ends_at), now())
		FROM b2b_org_license
		WHERE org_id = $1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&seats, &licenseEnds)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	if seats <= 0 {
		return EnrollCodeRow{}, ErrNoLicense
	}

	if reuse {
		row, err := activeEnrollCodeTx(ctx, tx, orgID)
		if err != nil {
			return EnrollCodeRow{}, err
		}
		if row != nil {
			if err := tx.Commit(ctx); err != nil {
				return EnrollCodeRow{}, err
			}
			return *row, nil
		}
	}

	var used int64
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM b2b_station WHERE org_id = $1 AND status = 'active'`, orgID).Scan(&used); err != nil {
		return EnrollCodeRow{}, err
	}
	free := seats - used
	if free <= 0 {
		return EnrollCodeRow{}, ErrSeatsExhausted
	}

	row, err := mintEnrollCode(ctx, tx, orgID, licenseEnds.UTC(), int(free), createdBy)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EnrollCodeRow{}, err
	}
	return row, nil
}

// OpenInstallerKey returns the org's live installer key, minting one only if
// there is none. It is idempotent on purpose: an admin installing 30 PCs over
// several days downloads the installer more than once, and a fresh key each
// time would kill the copies already handed out.
func (s Store) OpenInstallerKey(ctx context.Context, orgID uuid.UUID, createdBy string) (EnrollCodeRow, error) {
	return s.installerKeyTx(ctx, orgID, createdBy, true)
}

// RotateInstallerKey revokes the live key and mints a new one. Stations already
// enrolled are unaffected — they authenticate with their own Ed25519 key, not
// with this one.
func (s Store) RotateInstallerKey(ctx context.Context, orgID uuid.UUID, createdBy string) (EnrollCodeRow, error) {
	return s.installerKeyTx(ctx, orgID, createdBy, false)
}

// ActiveInstallerKey returns the live key, or nil when none is open.
func (s Store) ActiveInstallerKey(ctx context.Context, orgID uuid.UUID) (*EnrollCodeRow, error) {
	return activeEnrollCodeTx(ctx, s.Pool, orgID)
}
```

Extract the existing `ActiveEnrollCodeAsTeacher` body into a shared helper that takes anything with `QueryRow`, so both the transactional and pool-backed paths use one query:

```go
// pgxQuerier is satisfied by both *pgxpool.Pool and pgx.Tx.
type pgxQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func activeEnrollCodeTx(ctx context.Context, q pgxQuerier, orgID uuid.UUID) (*EnrollCodeRow, error) {
	var row EnrollCodeRow
	var revoked pgtype.Timestamptz
	err := q.QueryRow(ctx, `
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
		return nil, fmt.Errorf("active installer key: %w", err)
	}
	row.ExpiresAt = row.ExpiresAt.UTC()
	row.CreatedAt = row.CreatedAt.UTC()
	return &row, nil
}
```

Keep `OpenEnrollWindow` only if something still calls it after Task 1; if nothing does, delete it — `grep -rn "OpenEnrollWindow" backend` decides.

- [ ] **Step 4: Run the tests**

```bash
cd backend && go test ./internal/b2b/ -count=1
cd backend && golangci-lint run ./internal/b2b/...
```

Expected: PASS, 0 lint issues.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/b2b
git commit -m "feat(b2b): make the installer key live as long as the licence

A 2-hour window dies before a 30-PC rollout finishes. The real bound was never
the expiry, it is the seat cap, so the key now tracks the licence and repeated
downloads reuse it rather than invalidating the copies already handed out."
```

---

### Task 4: The config trailer, both sides

Backend writes it, the agent reads it. One format, so both sides land together.

**Files:**
- Create: `backend/internal/b2b/installer.go`
- Create: `backend/internal/b2b/installer_test.go`
- Create: `backend/station/internal/embedcfg/embedcfg.go`
- Create: `backend/station/internal/embedcfg/embedcfg_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `b2b.InstallerConfig{Code, API, Frontend, Org, Locale string}` with JSON tags `code`, `api`, `frontend`, `org`, `locale`
  - `b2b.AppendConfig(w io.Writer, base io.Reader, cfg InstallerConfig) error`
  - `b2b.InstallerFilename(orgName string, orgID uuid.UUID) string`
  - `embedcfg.Config{Code, API, Frontend, Org, Locale string}`
  - `embedcfg.Read(path string) (Config, error)`, `embedcfg.ErrNoConfig`

- [ ] **Step 1: Write the failing backend test**

`backend/internal/b2b/installer_test.go`:

```go
package b2b_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
)

func TestAppendConfigLayout(t *testing.T) {
	base := []byte("PRETEND-EXE-BYTES")
	cfg := b2b.InstallerConfig{
		Code: "AVTO-K7M2-P9XQ", API: "https://drivergo.uz",
		Frontend: "https://drivergo.uz", Org: "avto", Locale: "uz-Latn",
	}

	var out bytes.Buffer
	if err := b2b.AppendConfig(&out, bytes.NewReader(base), cfg); err != nil {
		t.Fatal(err)
	}
	blob := out.Bytes()

	// The base must be byte-identical and come first: the appended trailer is
	// what keeps the PE image itself untouched.
	if !bytes.HasPrefix(blob, base) {
		t.Fatal("base bytes were modified")
	}

	// Trailer: [json][uint32 BE len][16-byte magic]
	if got := string(blob[len(blob)-16:]); got != "AVTOSTATIONCFG01" {
		t.Fatalf("magic=%q", got)
	}
	n := binary.BigEndian.Uint32(blob[len(blob)-20 : len(blob)-16])
	jsonStart := len(blob) - 20 - int(n)
	if jsonStart != len(base) {
		t.Fatalf("json starts at %d, want %d (right after the base)", jsonStart, len(base))
	}

	var back b2b.InstallerConfig
	if err := json.Unmarshal(blob[jsonStart:len(blob)-20], &back); err != nil {
		t.Fatal(err)
	}
	if back != cfg {
		t.Fatalf("round trip changed the config: %+v", back)
	}
}

func TestInstallerFilename(t *testing.T) {
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")

	if got := b2b.InstallerFilename("1-sonli avtomaktab", id); got != "avtotest-station-1-sonli-avtomaktab.exe" {
		t.Fatalf("got %q", got)
	}
	// A name with no ASCII left after sanitising falls back to the org id, so
	// the download never lands as an unnamed or empty-slug file.
	got := b2b.InstallerFilename("Автомактаб", id)
	if !strings.HasPrefix(got, "avtotest-station-11111111") || !strings.HasSuffix(got, ".exe") {
		t.Fatalf("got %q, want the org-id fallback", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/b2b/ -run TestAppendConfigLayout -v`
Expected: FAIL — `b2b.InstallerConfig` undefined.

- [ ] **Step 3: Implement the writer**

`backend/internal/b2b/installer.go`:

```go
package b2b

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// configMagic marks a station binary that carries an appended configuration.
// Exactly 16 bytes; the reader in the station module matches on it verbatim,
// so changing it breaks every installer already handed out.
const configMagic = "AVTOSTATIONCFG01"

// maxConfigLen bounds what the reader will trust from a file's tail.
const maxConfigLen = 8 << 10

// InstallerConfig is what a downloaded agent needs to reach its own school
// without anything being typed on the PC.
type InstallerConfig struct {
	Code     string `json:"code"`
	API      string `json:"api"`
	Frontend string `json:"frontend"`
	Org      string `json:"org"`
	Locale   string `json:"locale"`
}

// AppendConfig streams base to w and appends cfg as a trailer:
//
//	[base bytes][config JSON][uint32 big-endian JSON length][magic]
//
// Windows tolerates trailing bytes after a PE image, so the binary stays
// runnable and its signature-free copy stays byte-identical up to the trailer.
// The length prefix means the reader seeks straight to the JSON instead of
// scanning for a marker that could occur inside the binary by chance.
func AppendConfig(w io.Writer, base io.Reader, cfg InstallerConfig) error {
	if _, err := io.Copy(w, base); err != nil {
		return fmt.Errorf("copy station binary: %w", err)
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if len(payload) > maxConfigLen {
		return fmt.Errorf("installer config too large: %d bytes", len(payload))
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(payload)))
	if _, err := w.Write(n[:]); err != nil {
		return err
	}
	_, err = io.WriteString(w, configMagic)
	return err
}

// InstallerFilename builds the download name. Org names are frequently Cyrillic
// and browsers mangle non-ASCII filenames differently, so the slug is ASCII-only
// and falls back to the org id when nothing usable survives.
func InstallerFilename(orgName string, orgID uuid.UUID) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(orgName) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = orgID.String()[:8]
	}
	return "avtotest-station-" + slug + ".exe"
}
```

- [ ] **Step 4: Write the failing station-side test**

`backend/station/internal/embedcfg/embedcfg_test.go`:

```go
package embedcfg_test

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"avtotest.uz/station/internal/embedcfg"
)

// build produces a file in the same layout the backend's AppendConfig writes:
// [base][json][uint32 BE len][16-byte magic].
func build(t *testing.T, base, jsonBody string) string {
	t.Helper()
	buf := []byte(base + jsonBody)
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(jsonBody)))
	buf = append(buf, n[:]...)
	buf = append(buf, []byte("AVTOSTATIONCFG01")...)

	p := filepath.Join(t.TempDir(), "agent.exe")
	if err := os.WriteFile(p, buf, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadRoundTrip(t *testing.T) {
	p := build(t, "PRETEND-EXE-BYTES",
		`{"code":"AVTO-K7M2-P9XQ","api":"https://drivergo.uz","frontend":"https://drivergo.uz","org":"avto","locale":"uz-Latn"}`)

	cfg, err := embedcfg.Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Code != "AVTO-K7M2-P9XQ" || cfg.API != "https://drivergo.uz" || cfg.Locale != "uz-Latn" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestReadRejectsBadTrailers(t *testing.T) {
	t.Run("no trailer", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "plain.exe")
		if err := os.WriteFile(p, []byte("JUST-AN-EXE"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := embedcfg.Read(p); !errors.Is(err, embedcfg.ErrNoConfig) {
			t.Fatalf("err=%v, want ErrNoConfig", err)
		}
	})

	t.Run("wrong magic", func(t *testing.T) {
		p := build(t, "BASE", `{"code":"x"}`)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		raw[len(raw)-1] = 'X'
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := embedcfg.Read(p); !errors.Is(err, embedcfg.ErrNoConfig) {
			t.Fatalf("err=%v, want ErrNoConfig", err)
		}
	})

	t.Run("length longer than the file", func(t *testing.T) {
		p := build(t, "BASE", `{"code":"x"}`)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		binary.BigEndian.PutUint32(raw[len(raw)-20:len(raw)-16], 1<<20)
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := embedcfg.Read(p); err == nil {
			t.Fatal("want an error for a length past the start of the file")
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		p := build(t, "BASE", `{"code":`)
		if _, err := embedcfg.Read(p); err == nil {
			t.Fatal("want an error for malformed json")
		}
	})
}
```

- [ ] **Step 5: Run to verify it fails**

Run: `cd backend/station && go test ./internal/embedcfg/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 6: Implement the reader**

`backend/station/internal/embedcfg/embedcfg.go`:

```go
// Package embedcfg reads the configuration the admin panel appends to a
// downloaded agent, so a classroom PC needs nothing typed into it.
//
// The layout is written by the backend's b2b.AppendConfig and must stay
// byte-compatible with it:
//
//	[base bytes][config JSON][uint32 big-endian JSON length][16-byte magic]
//
// The two sides live in separate Go modules, so the format is duplicated
// rather than shared. Each side has a test pinning the exact layout; changing
// one without the other turns a silent field mismatch into a failing test.
package embedcfg

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const magic = "AVTOSTATIONCFG01"

const trailerLen = 4 + len(magic)

// maxConfigLen mirrors the writer's bound; a length past it means a corrupt or
// hostile tail rather than a configuration this program wrote.
const maxConfigLen = 8 << 10

// ErrNoConfig means the file carries no appended configuration — an ordinary
// unconfigured build, which is not an error at the call site.
var ErrNoConfig = errors.New("no embedded configuration")

// Config is what the admin panel baked into this copy of the agent.
type Config struct {
	Code     string `json:"code"`
	API      string `json:"api"`
	Frontend string `json:"frontend"`
	Org      string `json:"org"`
	Locale   string `json:"locale"`
}

// Read returns the configuration appended to the file at path.
func Read(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer func() { _ = f.Close() }()

	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return Config{}, err
	}
	if size < int64(trailerLen) {
		return Config{}, ErrNoConfig
	}

	trailer := make([]byte, trailerLen)
	if _, err := f.ReadAt(trailer, size-int64(trailerLen)); err != nil {
		return Config{}, err
	}
	if string(trailer[4:]) != magic {
		return Config{}, ErrNoConfig
	}

	n := int64(binary.BigEndian.Uint32(trailer[:4]))
	if n == 0 || n > maxConfigLen || n > size-int64(trailerLen) {
		return Config{}, fmt.Errorf("embedded config length %d is not plausible for a %d-byte file", n, size)
	}

	payload := make([]byte, n)
	if _, err := f.ReadAt(payload, size-int64(trailerLen)-n); err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse embedded config: %w", err)
	}
	return cfg, nil
}
```

- [ ] **Step 7: Run both sides**

```bash
cd backend && go test ./internal/b2b/ -run "TestAppendConfig|TestInstallerFilename" -v
cd backend/station && go test ./... -count=1
cd backend/station && gofmt -l .
```

Expected: PASS on both, `gofmt -l` silent.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/b2b/installer.go backend/internal/b2b/installer_test.go backend/station/internal/embedcfg
git commit -m "feat(b2b): append per-school config to the station binary

A length-prefixed trailer rather than a scanned marker, so the reader seeks
straight to the payload and cannot be fooled by the same bytes occurring
inside the executable. Writer and reader live in separate modules, so each
pins the exact layout in a test."
```

---

### Task 5: Admin installer endpoints

**Files:**
- Create: `backend/internal/admin/b2b_installer.go`
- Create: `backend/internal/admin/b2b_installer_test.go`
- Modify: `backend/internal/admin/handlers.go` — four new routes
- Modify: `backend/internal/config/config.go` — `STATION_BINARY_PATH`

**Interfaces:**
- Consumes: `Store.OpenInstallerKey`, `Store.RotateInstallerKey`, `Store.ActiveInstallerKey` (Task 3); `b2b.AppendConfig`, `b2b.InstallerConfig`, `b2b.InstallerFilename` (Task 4).
- Produces:
  - `GET /admin/v1/b2b/orgs/{id}/installer` → `{"data": {...}}` or `{"data": null}`
  - `POST /admin/v1/b2b/orgs/{id}/installer` → `{"data": {...}}`
  - `POST /admin/v1/b2b/orgs/{id}/installer/rotate` → `{"data": {...}}`
  - `GET /admin/v1/b2b/orgs/{id}/installer.exe?locale=uz-Latn` → binary stream
  - Response shape: `{code, max_uses, used_count, expires_at}`

- [ ] **Step 1: Write the failing test**

`backend/internal/admin/b2b_installer_test.go` — follow the fixture and admin-auth pattern already used by `backend/internal/admin/b2b_test.go`; read it first and reuse its helpers rather than inventing new ones. Cover:

```go
// TestInstallerKeyEndpoints asserts the idempotence the download flow depends
// on: an admin installing 30 PCs over several days must be able to fetch the
// installer repeatedly without invalidating the copies already handed out.
func TestInstallerKeyEndpoints(t *testing.T) {
	// Arrange an org with a 30-seat, 365-day licence (same helpers as b2b_test.go).
	//
	// 1. GET  /installer          -> 200, data == null
	// 2. POST /installer          -> 200, code non-empty, max_uses == 30
	// 3. POST /installer again    -> 200, SAME code
	// 4. GET  /installer          -> 200, same code, used_count == 0
	// 5. POST /installer/rotate   -> 200, DIFFERENT code
	// 6. GET  /installer          -> 200, the rotated code
}

// TestInstallerExeCarriesTheConfig proves the download is a real binary with a
// readable trailer, not just a 200.
func TestInstallerExeCarriesTheConfig(t *testing.T) {
	// Point the handler at a fake base binary written to t.TempDir(), open a
	// key, then GET /installer.exe?locale=ru and assert:
	//   - Content-Disposition names avtotest-station-<slug>.exe
	//   - the body starts with the fake base bytes
	//   - the trailer parses back to the org's code, api, frontend, org name
	//     and locale "ru"
	// Parse the trailer with the same layout b2b.AppendConfig writes:
	// [base][json][uint32 BE len][16-byte magic "AVTOSTATIONCFG01"].
}

// TestInstallerExeWithoutAKey asserts the download refuses rather than minting
// one as a side effect of a GET.
func TestInstallerExeWithoutAKey(t *testing.T) {
	// GET /installer.exe on an org with no key -> 409, code "no_installer_key".
}

// TestInstallerRefusesOrgWithoutLicense
func TestInstallerRefusesOrgWithoutLicense(t *testing.T) {
	// POST /installer on an org with no live licence -> 409, code "no_license".
}
```

Write these out fully against the existing admin test harness — the comments above are the required cases, not a substitute for the code.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/admin/ -run TestInstaller -v`
Expected: FAIL — routes 404 / handlers undefined.

- [ ] **Step 3: Add the config value**

In `backend/internal/config/config.go`, next to the other path-shaped settings:

```go
// StationBinaryPath is the Windows agent the admin panel serves. The API image
// cross-compiles it at build time (see backend/Dockerfile); overriding this is
// only for local development, where no such build exists.
StationBinaryPath string
```

and in the loader:

```go
StationBinaryPath: getenv("STATION_BINARY_PATH", "/station/avtotest-station.exe"),
```

- [ ] **Step 4: Implement the handlers**

`backend/internal/admin/b2b_installer.go`:

```go
package admin

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/httpx"
)

type installerKeyDTO struct {
	Code      string    `json:"code"`
	MaxUses   int       `json:"max_uses"`
	UsedCount int       `json:"used_count"`
	ExpiresAt time.Time `json:"expires_at"`
}

func toInstallerKeyDTO(row b2b.EnrollCodeRow) installerKeyDTO {
	return installerKeyDTO{
		Code: row.Code, MaxUses: row.MaxUses,
		UsedCount: row.UsedCount, ExpiresAt: row.ExpiresAt,
	}
}

// allowedInstallerLocales mirrors the frontend's locale list. An unknown value
// would produce an installer whose kiosk URL 404s on every classroom PC, so it
// is rejected here rather than discovered in a school.
var allowedInstallerLocales = map[string]bool{"uz-Latn": true, "uz-Cyrl": true, "ru": true}

func (h *Handler) getB2BInstaller(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	row, err := h.b2bStore().ActiveInstallerKey(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "installer key query failed")
		return
	}
	if row == nil {
		httpx.Data(w, http.StatusOK, nil)
		return
	}
	httpx.Data(w, http.StatusOK, toInstallerKeyDTO(*row))
}

func (h *Handler) openB2BInstaller(w http.ResponseWriter, r *http.Request) {
	h.installerKeyWrite(w, r, false)
}

func (h *Handler) rotateB2BInstaller(w http.ResponseWriter, r *http.Request) {
	h.installerKeyWrite(w, r, true)
}

func (h *Handler) installerKeyWrite(w http.ResponseWriter, r *http.Request, rotate bool) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	actor := adminActorLabel(r)
	var (
		row b2b.EnrollCodeRow
		err error
	)
	if rotate {
		row, err = h.b2bStore().RotateInstallerKey(r.Context(), orgID, actor)
	} else {
		row, err = h.b2bStore().OpenInstallerKey(r.Context(), orgID, actor)
	}
	if err != nil {
		writeInstallerErr(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, toInstallerKeyDTO(row))
}

func (h *Handler) downloadB2BInstaller(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = "uz-Latn"
	}
	if !allowedInstallerLocales[locale] {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be uz-Latn, uz-Cyrl or ru")
		return
	}

	row, err := h.b2bStore().ActiveInstallerKey(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "installer key query failed")
		return
	}
	if row == nil {
		// A GET must not mint a key as a side effect.
		httpx.Error(w, http.StatusConflict, "no_installer_key", "open an installer key first")
		return
	}

	var orgName string
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT name FROM b2b_org WHERE id = $1`, orgID).Scan(&orgName); err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
		return
	}

	base, err := os.Open(h.StationBinaryPath)
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "installer_unavailable",
			"station binary is not present in this build")
		return
	}
	defer func() { _ = base.Close() }()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+b2b.InstallerFilename(orgName, orgID)+`"`)
	// Streamed, so no Content-Length: the trailer's size is not known until the
	// base has been copied, and buffering a 7 MB binary per download to learn it
	// buys nothing.
	if err := b2b.AppendConfig(w, base, b2b.InstallerConfig{
		Code:     row.Code,
		API:      h.PublicBaseURL,
		Frontend: h.PublicBaseURL,
		Org:      orgName,
		Locale:   locale,
	}); err != nil {
		// The status is already sent; the truncated download is the signal.
		return
	}
}

func writeInstallerErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, b2b.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
	case errors.Is(err, b2b.ErrOrgSuspended):
		httpx.Error(w, http.StatusConflict, "org_suspended", "org is suspended")
	case errors.Is(err, b2b.ErrNoLicense):
		httpx.Error(w, http.StatusConflict, "no_license", "org has no active licence")
	case errors.Is(err, b2b.ErrSeatsExhausted):
		httpx.Error(w, http.StatusConflict, "seats_exhausted", "all licensed seats are in use")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "installer key failed")
	}
}
```

`adminActorLabel(r)` — reuse whatever the admin package already uses to label an acting admin in `created_by` columns; grep `created_by` in `backend/internal/admin/` and follow it. If nothing exists, use the admin id from the request claims.

`h.StationBinaryPath` and `h.PublicBaseURL` are new fields on `admin.Handler` — add them and populate both where the handler is constructed in `backend/internal/server/server.go`.

- [ ] **Step 5: Register the routes**

In `backend/internal/admin/handlers.go`, alongside the other B2B routes. The read goes with the other `br.Get` block, the writes with the `br.Post` block:

```go
br.Get("/b2b/orgs/{id}/installer", h.getB2BInstaller)
br.Get("/b2b/orgs/{id}/installer.exe", h.downloadB2BInstaller)
br.Post("/b2b/orgs/{id}/installer", h.openB2BInstaller)
br.Post("/b2b/orgs/{id}/installer/rotate", h.rotateB2BInstaller)
```

- [ ] **Step 6: Add the Next.js proxy routes**

Create `frontend/src/app/api/admin/b2b/orgs/[id]/installer/route.ts` and `.../installer/rotate/route.ts` following the shape of the existing `.../stations/route.ts`. The `.exe` download must stream the body through unchanged and forward `Content-Disposition` — do not `JSON.parse` it. Read `frontend/src/app/api/admin/b2b/orgs/[id]/export-csv/route.ts` first; it already forwards a non-JSON body and is the closest model.

- [ ] **Step 7: Run the tests**

```bash
cd backend && go test ./internal/admin/ ./internal/b2b/ -count=1
cd backend && go build ./... && go vet ./...
cd backend && golangci-lint run ./internal/admin/... ./internal/b2b/... ./internal/config/...
```

Expected: PASS, 0 lint issues.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/admin backend/internal/config frontend/src/app/api/admin
git commit -m "feat(admin): serve a per-school station installer

GET streams the image's Windows agent with the org's key appended, so nothing
is typed on a classroom PC. POST is idempotent and the download refuses to mint
a key as a side effect, so repeated downloads never invalidate installers
already handed out."
```

---

### Task 6: Agent reads its embedded configuration

**Files:**
- Modify: `backend/station/cmd/avtotest-station/main.go`
- Modify: `backend/station/cmd/avtotest-station/main_test.go`

**Interfaces:**
- Consumes: `embedcfg.Read(path) (Config, error)`, `embedcfg.ErrNoConfig` (Task 4).
- Produces: no new exported symbols; behaviour only.

- [ ] **Step 1: Write the failing test**

Add to `backend/station/cmd/avtotest-station/main_test.go`:

```go
// TestEmbeddedConfigBeatsFlagDefaults proves a downloaded installer needs no
// arguments: the appended config supplies the code and the URLs, and a flag
// left at its compiled-in default must not silently win over it.
func TestEmbeddedConfigBeatsFlagDefaults(t *testing.T) {
	embedded := embedcfg.Config{
		Code: "AVTO-K7M2-P9XQ", API: "https://drivergo.uz",
		Frontend: "https://drivergo.uz", Org: "avto", Locale: "ru",
	}
	// Flag values are the compiled-in defaults and none was passed.
	got := resolveConfig(embedded, "", "https://drivergo.uz", "https://drivergo.uz", "uz-Latn",
		false, false, false)

	if got.Code != "AVTO-K7M2-P9XQ" {
		t.Fatalf("Code=%q, want the embedded code", got.Code)
	}
	if got.Locale != "ru" {
		t.Fatalf("Locale=%q, want the embedded ru over the uz-Latn default", got.Locale)
	}
	if got.Org != "avto" {
		t.Fatalf("Org=%q", got.Org)
	}
}

// TestFlagsWinWhenNothingIsEmbedded keeps the manual install path working for a
// plain unconfigured build.
func TestFlagsWinWhenNothingIsEmbedded(t *testing.T) {
	got := resolveConfig(embedcfg.Config{}, "AVTO-TYPED-BYHAND",
		"https://drivergo.uz", "https://drivergo.uz", "uz-Latn", false, false, false)

	if got.Code != "AVTO-TYPED-BYHAND" {
		t.Fatalf("Code=%q, want the flag value", got.Code)
	}
	if got.API != "https://drivergo.uz" || got.Locale != "uz-Latn" {
		t.Fatalf("got=%+v, want the flag values untouched", got)
	}
}

// TestExplicitFlagOverridesEmbedded lets an operator point one PC at staging
// without rebuilding an installer for it.
func TestExplicitFlagOverridesEmbedded(t *testing.T) {
	embedded := embedcfg.Config{
		Code: "AVTO-K7M2-P9XQ", API: "https://drivergo.uz",
		Frontend: "https://drivergo.uz", Locale: "uz-Latn",
	}
	got := resolveConfig(embedded, "", "https://staging.example", "https://drivergo.uz", "uz-Latn",
		true /* apiSet */, false, false)

	if got.API != "https://staging.example" {
		t.Fatalf("API=%q, want the explicitly passed flag to win", got.API)
	}
	// Everything not passed still comes from the binary.
	if got.Code != "AVTO-K7M2-P9XQ" {
		t.Fatalf("Code=%q, want the embedded code", got.Code)
	}
}
```

- [ ] **Step 2: Implement**

Add a pure resolver so precedence is testable without touching the filesystem or the network:

```go
// resolved is the agent's effective configuration after merging what was baked
// into this copy with what the operator passed on the command line.
type resolved struct {
	Code     string
	API      string
	Frontend string
	Locale   string
	Org      string
}

// resolveConfig merges an embedded config with flag values. An explicitly-set
// flag always wins, so one PC can be pointed at staging without a rebuild;
// otherwise the embedded value wins over the compiled-in default, which is what
// makes a downloaded installer need no arguments at all.
func resolveConfig(embedded embedcfg.Config, flagCode, flagAPI, flagFrontend, flagLocale string, apiSet, frontendSet, localeSet bool) resolved {
	out := resolved{
		Code: flagCode, API: flagAPI, Frontend: flagFrontend,
		Locale: flagLocale, Org: embedded.Org,
	}
	if out.Code == "" {
		out.Code = embedded.Code
	}
	if !apiSet && embedded.API != "" {
		out.API = embedded.API
	}
	if !frontendSet && embedded.Frontend != "" {
		out.Frontend = embedded.Frontend
	}
	if !localeSet && embedded.Locale != "" {
		out.Locale = embedded.Locale
	}
	return out
}
```

In `main`, read the embedded config before using any flag value:

```go
	embedded := embedcfg.Config{}
	if exe, err := os.Executable(); err == nil {
		if cfg, err := embedcfg.Read(exe); err == nil {
			embedded = cfg
		} else if !errors.Is(err, embedcfg.ErrNoConfig) {
			log.Printf("embedded config: %v (falling back to flags)", err)
		}
	}
```

Determine `apiSet` / `frontendSet` / `localeSet` with `flag.Visit`, which reports only flags the operator actually passed:

```go
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
```

Also fix the compiled-in defaults, which point at a domain this deployment does not use: `-api` and `-frontend` both default to `https://drivergo.uz`.

- [ ] **Step 3: Run the tests**

```bash
cd backend/station && go test ./... -count=1
cd backend/station && GOOS=windows GOARCH=amd64 go build ./...
cd backend/station && go vet ./... && gofmt -l .
```

Expected: PASS, cross-build clean, `gofmt -l` silent.

- [ ] **Step 4: Commit**

```bash
git add backend/station
git commit -m "feat(station): take the school's config from the binary's own tail

A downloaded installer now needs no arguments. An explicitly passed flag still
wins, so one PC can be pointed at staging without a rebuild, and a plain
unconfigured build keeps the manual -code path."
```

---

### Task 7: Self-install, autostart and uninstall

**Files:**
- Create: `backend/station/internal/selfinstall/selfinstall.go`
- Create: `backend/station/internal/selfinstall/selfinstall_windows.go`
- Create: `backend/station/internal/selfinstall/selfinstall_other.go`
- Create: `backend/station/internal/selfinstall/selfinstall_test.go`
- Modify: `backend/station/cmd/avtotest-station/main.go`

**Interfaces:**
- Consumes: nothing from earlier tasks beyond the resolved config.
- Produces:
  - `selfinstall.Target(stateDir string) string` — where the agent should live
  - `selfinstall.Ensure(stateDir string) (installedPath string, didInstall bool, err error)` — copy self into place and register autostart, idempotent
  - `selfinstall.Remove(stateDir string) error` — deregister autostart and delete the installed copy plus state

- [ ] **Step 1: Write the failing test**

`backend/station/internal/selfinstall/selfinstall_test.go` — the copy logic is platform-independent and therefore testable on Linux; the registry half is not.

```go
func TestEnsureCopiesOnceAndIsIdempotent(t *testing.T) {
	// Point stateDir at t.TempDir(); run Ensure twice.
	// First call: didInstall == true, the target file exists and its bytes are
	// identical to the running test binary's own file.
	// Second call: didInstall == false and the file's modification time is
	// unchanged -- proving a re-run does not rewrite it.
}

func TestEnsureSkipsWhenAlreadyRunningFromTarget(t *testing.T) {
	// When the executable is already at Target(stateDir), Ensure must not copy
	// a file onto itself -- on Windows that fails outright with a sharing
	// violation, so this is the case that breaks every second boot.
}

func TestRemoveDeletesTheInstalledCopy(t *testing.T) {
	// After Ensure, Remove leaves no target file behind and does not error when
	// called twice.
}
```

Write these fully. Note the autostart registration is a no-op on non-Windows, so these tests exercise the copy path only; say so in a comment.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend/station && go test ./internal/selfinstall/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement the platform-independent half**

`backend/station/internal/selfinstall/selfinstall.go` holds `Target`, `Ensure` and `Remove`. `Ensure` must:
- resolve `os.Executable()`, and return `(target, false, nil)` immediately when it already equals `Target(stateDir)` — on Windows a running image cannot be copied over itself
- create the state dir, copy the running binary to `Target(stateDir)` with `0o755`, writing to a temp name in the same directory and renaming into place so a crash mid-copy cannot leave a truncated agent
- call `registerAutostart(target)` — the platform hook
- return `(target, true, nil)`

`Remove` calls `unregisterAutostart()` then deletes the target file; both halves are idempotent and a missing file is not an error.

Follow `backend/station/internal/keystore/` for the platform split: a shared file with the logic, plus `_windows.go` / `_other.go` supplying `registerAutostart(path string) error` and `unregisterAutostart() error`.

- [ ] **Step 4: Implement the Windows half**

`selfinstall_windows.go` writes the autostart value under
`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, value name `AvtoTestStation`,
data the quoted full path to the installed copy. Use `golang.org/x/sys/windows/registry`, already an indirect dependency of the module through `hwid_windows.go`.

HKCU rather than HKLM is deliberate and belongs in a comment: HKLM needs administrator rights, and a classroom PC usually runs as an ordinary user, so requiring elevation would mean the agent silently fails to persist on exactly the machines it targets.

`selfinstall_other.go` makes both hooks no-ops returning nil, with a comment saying the development build does not persist and must never ship to a school — matching the wording already in `keystore_other.go`.

- [ ] **Step 5: Wire it into main**

After the embedded config is resolved and before enrolment, when a config was embedded:

```go
	if embedded.Code != "" {
		installed, didInstall, err := selfinstall.Ensure(*stateDir)
		if err != nil {
			log.Printf("self-install: %v (continuing from the current location)", err)
		} else if didInstall {
			log.Printf("installed to %s and registered autostart", installed)
		}
	}
```

Self-install runs only for a configured installer, so a developer running the plain build does not get an autostart entry written on their machine.

Add the `-uninstall` flag: when set, call `selfinstall.Remove(*stateDir)`, print what was removed and the reminder that the station must also be revoked in the admin panel to free its seat, then exit — before any enrolment or kiosk launch.

- [ ] **Step 6: Extend the self-test**

Add two checks to the existing `-selftest` output, keeping its numbering and pass/fail style:
- autostart round-trip: register, read the value back, assert it equals the installed path, then restore the previous value
- install target writable: assert `Target(stateDir)`'s directory can be created and written

On Linux both must report the same "not applicable on this build" line the DPAPI checks already use for the development keystore, so the Linux run stays honest.

- [ ] **Step 7: Run everything**

```bash
cd backend/station && go test ./... -count=1
cd backend/station && GOOS=windows GOARCH=amd64 go build ./...
cd backend/station && go vet ./... && gofmt -l .
cd backend/station && go run ./cmd/avtotest-station -selftest; echo "exit=$?"
```

Expected: tests pass, cross-build clean, `gofmt -l` silent. The Linux `-selftest` still exits non-zero because the development keystore does not seal — that is the correct result and proves the check still has teeth.

- [ ] **Step 8: Commit**

```bash
git add backend/station
git commit -m "feat(station): install into place and come back on every boot

Copies itself into the state directory and registers HKCU autostart, which
needs no administrator rights -- a classroom PC usually runs as an ordinary
user, so requiring elevation would fail on exactly the machines this targets.
Re-running is a no-op, and -uninstall reverses it."
```

---

### Task 8: Admin UI installer panel

**Files:**
- Create: `frontend/src/app/[locale]/admin/(shell)/b2b/orgs/[id]/installer-panel.tsx`
- Create: `frontend/src/app/[locale]/admin/(shell)/b2b/orgs/[id]/installer-panel.test.tsx`
- Modify: `frontend/src/app/[locale]/admin/(shell)/b2b/orgs/[id]/page.tsx`
- Modify: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`

**Interfaces:**
- Consumes: the four admin installer endpoints (Task 5).
- Produces: `InstallerPanel({ orgId }: { orgId: string })` as the default export.

- [ ] **Step 1: Write the failing test**

`installer-panel.test.tsx` — model the mocking on the deleted-then-rebuilt patterns in this repo; `frontend/src/app/[locale]/(app)/teacher/enroll-window.test.tsx` was the closest example before Task 1 removed it, so use `frontend/src/app/[locale]/(kiosk)/station/page.test.tsx` as the live model instead. Cover:

- mounts with `GET` returning `null` → renders the "no key" state and an open button
- clicking open `POST`s and renders the returned code, `used_count / max_uses` and the expiry date
- clicking rotate `POST`s to `/rotate` and renders the new code
- the download link points at `/api/admin/b2b/orgs/<id>/installer.exe?locale=<selected>` and changes when the locale select changes

Assert the exact request URLs, not just that a fetch happened.

- [ ] **Step 2: Run to verify it fails**

Run: `cd frontend && npm run test -- installer-panel`
Expected: FAIL — the module does not exist.

- [ ] **Step 3: Build the panel**

A self-contained client component with its own fetch cycle, mirroring how the org page's other sections are written. It shows:
- the current key, in a large monospace block, or a "no key yet" line
- `used_count / max_uses` and the expiry date
- a locale `<select>` with `uz-Latn`, `uz-Cyrl`, `ru`
- a primary **download** control — a plain `<a download href=...>` so the browser handles the stream, not `fetch`
- an **open key** button when there is none, and a **rotate** button when there is
- rotate must ask for confirmation first and say plainly that installers already handed out stop working while enrolled PCs keep running

Render it from `page.tsx` directly above the station list.

New keys in the `AdminB2B` namespace of all three locale files. `uz-Latn`:

```json
"installerTitle": "O'rnatish fayli",
"installerNone": "Hali kalit ochilmagan",
"installerOpen": "Kalitni ochish",
"installerRotate": "Kalitni almashtirish",
"installerDownload": "O'rnatish faylini yuklab olish",
"installerUsed": "{used} / {max} PC ulangan",
"installerExpires": "{date} gacha amal qiladi",
"installerLocale": "Kiosk tili",
"installerRotateConfirm": "Kalit almashtiriladi. Tarqatilgan eski fayllar ishlamay qoladi, ulangan PClar esa ishlashda davom etadi. Davom etamizmi?",
"installerHint": "Faylni har bir kompyuterda ishga tushiring. Kod fayl ichida — hech narsa kiritilmaydi.",
"installerError": "Amal bajarilmadi"
```

Translate all eleven for `uz-Cyrl.json` and `ru.json`. Do not leave Latin text in either.

- [ ] **Step 4: Run the frontend gate**

```bash
cd frontend && npm run test -- installer-panel
make fe-check
```

Expected: both PASS. A missing or mistyped message key fails at build time, which is what proves the i18n work is complete.

- [ ] **Step 5: Commit**

```bash
git add frontend
git commit -m "feat(admin): installer panel on the org page

One download per school, repeatable without invalidating what is already
deployed; rotation is behind a confirmation that says what it breaks."
```

---

### Task 9: Documentation

**Files:**
- Modify: `docs/b2b/school-station-pricing.md`
- Modify: `docs/b2b/school-contract-template.md`
- Modify: `backend/station/README.md`

**Interfaces:**
- Consumes: everything above.
- Produces: no code.

- [ ] **Step 1: Rewrite the onboarding section**

`docs/b2b/school-station-pricing.md` currently describes the deleted flow: per-PC codes entered on `/teacher`, learners logging in with their own accounts, and a home-seat SKU. Replace the "Sinfxonani ishga tushirish va login qoidasi" section with what now happens:

1. Admin creates the org and adds a licence for N stations.
2. Admin opens the installer key and downloads `avtotest-station-<slug>.exe` from the org page.
3. The file is run once on each classroom PC. Nothing is typed; the PC installs itself and comes back on every boot.
4. The kiosk runs with no login. VIP comes from the school's licence, not from any learner account.

Delete the home-seat row and the parallel-session discussion, which described learner logins that no longer exist. Keep the packages table and the resale prohibition.

`docs/b2b/school-contract-template.md`: remove the "Home seat (ixtiyoriy): ___" line and the clause about learner personal accounts, since a classroom station now has none.

- [ ] **Step 2: Update the agent README**

`backend/station/README.md`: the paths moved to `backend/station/`, and the primary install path is now the downloaded installer, not `-code`. Document both, the `-uninstall` flag, and keep the `-selftest` / `-selftest-import` procedure exactly as it is — it is still the only way to confirm DPAPI works on real hardware.

State plainly that offline is not supported: if the internet drops, the classroom stops immediately, because every question comes from the server.

- [ ] **Step 3: Verify no stale instructions survive**

```bash
grep -rn "teacher\|home_seat\|home seat\|/me/teacher" docs/b2b backend/station/README.md
```

Expected: no hits describing the deleted flow.

- [ ] **Step 4: Commit**

```bash
git add docs backend/station/README.md
git commit -m "docs(b2b): describe the installer onboarding flow

The pricing sheet still told operators to hand out per-PC codes on /teacher and
to give every learner an account -- a flow that no longer exists in the product."
```

---

## Definition of Done

- `make check` and `make fe-check` pass.
- `cd backend/station && go test ./... && GOOS=windows GOARCH=amd64 go build ./...` passes.
- `grep -rn "teacherRole\|b2b_org_member\|b2b_invite\|home_seats\|AsTeacher" backend/internal frontend/src` matches only migration history.
- An admin can open a key, download the same installer twice, and rotate it.
- The downloaded file is a PE32+ binary whose trailer parses back to that org's code.
- `-selftest` on Linux still reports the development keystore as unsealed and exits non-zero.

## Not in this plan

Kiosk section expansion — signs, exam, stats, saved and the read-only leaderboard — is **Plan 2**, together with widening `kiosk-path.test.tsx` to check navigation targets and not only route registration.

Offline tolerance is not planned at all. Arena, mistakes and grand mock stay out of the kiosk.
