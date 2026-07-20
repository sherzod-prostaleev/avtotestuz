# M1 Plan 08 — E2E Verification + Staging Deploy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The final plan of Milestone 1. Two things, and only two: (1) **automated end-to-end verification** — a CI-wired test that drives the whole M1 learner journey through the real Flutter web build against the real Go backend + Postgres + Redis (not the isolated unit/widget tests of Plans 06-07, and not a one-time manual pass — a repeatable, headless, merge-gating E2E suite), and (2) **a real, reachable staging environment** — the Go backend and the Flutter web build deployed to a hosted staging box (not just local `make up`), with staging-vs-dev secrets/config, a staging Postgres on the self-migrating path, a deploy mechanism, a post-deploy health/smoke check, and a documented rollback. This is what turns "green tests on a laptop" into "a URL a human can open and click through the whole M1 product."

**Architecture:** No new application features. This plan adds **operational infrastructure** around the existing code: a production `Dockerfile` for the Go API (which already self-migrates and reads all config from env vars — see `backend/internal/config/config.go` and `backend/cmd/api/main.go`), a static-serving container for the `flutter build web` output, a new CI `e2e` job composing the full stack, a set of `integration_test/` flows covering the M1 journey, a staging deploy workflow, and a smoke/rollback runbook. The Flutter app stays exactly as Plans 06-07 left it; the backend stays exactly as Plans 01-05 left it. If any task finds it *needs* an app-code change to be deployable/testable (e.g. a missing readiness detail), that is a finding to flag and scope narrowly, not a license to add features.

**Tech Stack:** Existing — Go 1.22+ API, Postgres 16, Redis 7, MinIO (S3-compatible media), Flutter 3.44.6 web build, golang-migrate (embedded, self-applied on boot), Docker + docker-compose, GitHub Actions. New surface introduced by this plan: Docker multi-stage builds, a static web server (nginx or an equivalently tiny static server) for the Flutter output, `chromedriver`-driven `flutter drive` E2E (the recipe already established in Plan 06 Task 9), and one deploy workflow.

**Plan sequence for M1:** 01-05 backend (complete) → 06 Flutter foundation (complete) → 07 Flutter test flows (complete) → **08 E2E + staging deploy (this plan, the last of M1)**. After this plan, M1 is done; M2 (monetization: tariffs, Payme/Click, promo/referral, GRAND MOCK, guest-demo, public SSG site) begins — none of which is in scope here (see Scope Boundaries and Self-Review).

## Environment note (carried over from Plans 06/07 — still applies)

```bash
export PATH="$HOME/.local/flutter/bin:$PATH"
export CHROME_EXECUTABLE=google-chrome-stable
# Backend Go toolchain (established convention):
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
```

**Use `dart analyze`, not `flutter analyze`** (LSP Content-Length bug on this repo's Cyrillic path — CI unaffected). **Use `flutter test --concurrency=1`, not bare `flutter test`** (concurrency quirk on this path — CI unaffected). **Dev backend runs on `PORT=8090`** (port 8080 occupied in this environment — established since Plan 06). Local Flutter web points at `http://localhost:8090/api/v1` unless overridden via `--dart-define=API_BASE_URL=...`.

**Backend DB test-contention gotcha (from Plan 07's operational note — applies to this plan's E2E work):** `go test ./... -p 1` and any process that runs `testdb.Truncate` share the single `avtotest_test` database via cross-table `TRUNCATE CASCADE`; two concurrent invocations deadlock/duplicate-key. **Do not run this plan's local backend-touching verification concurrently with another backend agent** against the shared dev/test DB. The E2E stack this plan builds must stand up its **own** ephemeral Postgres (its own container/DB), never the shared `avtotest_test` — in CI that is automatic (fresh service container); locally, use a dedicated compose stack/DB, not `make test`'s database.

**E2E recipe is already established — reuse it, do not rediscover it.** Plan 06 Task 9 and Plan 07 Task 11 both ran `integration_test` flows against a real backend via a **separate `chromedriver` process** + `flutter drive --driver=test_driver/integration_test.dart --target=integration_test/<flow>.dart -d web-server --browser-name=chrome` (the naive `flutter test -d chrome` / `flutter drive -d chrome` forms do **not** work for web integration tests on this setup — see `README.md`'s Flutter section and the comment at the top of `app/test_driver/integration_test.dart`). This plan generalizes that same recipe into a CI job; it does not invent a new one.

**OTP in automated E2E — a real constraint, get it right up front.** The backend's `sandbox` OTP channel returns the code as `debug_code` in the `POST /auth/otp/request` JSON response. The Flutter UI surfaces that code on `OtpVerifyScreen` **only under `kDebugMode`** (Plan 06 Task 7). `flutter drive` runs a **debug** build (`kDebugMode == true`), so the E2E flows (Tasks 3-4) can read the code from the UI exactly as Plan 06's `auth_flow_test` did. But a deployed **release** web build is **not** `kDebugMode`, so the code is **not** shown in the deployed UI — therefore the post-deploy staging smoke (Task 7) must read `debug_code` from the backend **API** directly (curl the JSON), not from the release UI. Every E2E/staging environment in this plan MUST set `OTP_CHANNEL=sandbox` (never `telegram`/`sms`) so no real OTP delivery is needed.

**No screenshot/browser-automation tool is available to agents** — same as Plans 06/07. Verification standard: `dart analyze` + `flutter test --concurrency=1` + `flutter build web` for the app; `make check` (lint+test) for the backend; `docker build`/`docker compose` for the new container work; and the `flutter drive` E2E pass for the journey flows. The one **live staging deploy** (Task 8) is a documented, one-time operator pass with real commands/responses recorded — not something an agent can fully self-serve if the deploy target (see Open Decision D18) isn't provisioned yet.

## Open Decision — D18 (staging deploy target) — BLOCKS Tasks 6-8, needs user confirmation

Following this repo's D1–D17 decision-numbering convention (master spec §2), this plan surfaces one **new** decision that must be confirmed by the user **before Tasks 6-8 execute**:

> **D18 — Staging deploy target/host.** Nothing in the repo currently implies a hosting target: there is no `Dockerfile`, no `fly.toml`/`render.yaml`/`Procfile`, no cloud config, no deploy workflow — only `docker-compose.yml` (postgres/redis/minio infra) and a test-only `.github/workflows/ci.yml`. The master spec (§22) commits **production** to a **UZ VPS running Docker** (personal-data-residency law, §20 — Uzbek citizens' personal data must live on servers physically in Uzbekistan). **Staging holds only test/sandbox data** (test phone numbers via the sandbox OTP channel, no real citizen PII, no real payments — billing is M2), so the strict UZ-residency constraint does **not** bind staging as hard as prod.
>
> **Recommended default (reasoning below), pending user confirmation:** a **single small Docker host** (a VPS) running the existing `docker-compose` stack plus the two new app containers (backend + web), deployed over SSH with `docker compose pull && up -d`. Reasoning: (a) it reuses the exact prod-shape the spec already commits to (§22 "prod: UZ VPS, Docker"), so staging de-risks the eventual prod deploy instead of introducing a throwaway-only PaaS shape we'd have to redo; (b) it needs no new managed-service accounts and keeps Postgres/Redis/MinIO identical to dev; (c) it is provider-agnostic — any VPS (a cheap cloud VM for staging, or the eventual UZ prod VPS itself in a `staging` compose project) satisfies it.
>
> **Alternatives the user might prefer instead:** a managed PaaS for a zero-ops throwaway staging (Fly.io / Render / Railway — fast to stand up, but a different shape than prod, and their default regions are outside UZ, acceptable for test-only data but a divergence to eventually reconcile); or splitting backend (container on a VPS) from the static web build (any static host / object storage + CDN).
>
> **What is target-agnostic and can proceed regardless (Tasks 1-5):** the Docker images, the env-var/secrets contract, the E2E test suite, and the CI E2E job are all written to be host-independent and do **not** wait on D18. Only the concrete deploy *destination* (Tasks 6-8's workflow secrets, the box, the DNS/URL) waits on the user's answer. Tasks 6-8 below are written against the recommended default and flag every place the user's chosen target changes a concrete value.

**Implementers/dispatchers: do not silently pick a provider.** If D18 is unanswered when Tasks 6-8 come up, stop and get the user's confirmation first, exactly as prior plans stopped for content/secret decisions.

## Global Constraints

- Repo root: `/home/sher/Рабочий стол/avtotest` (Cyrillic + space — always double-quote in shell, never backslash-escape). Backend: `backend/` (module `avtotest.uz/backend`). Flutter app: `app/` (package `avtotest_app`).
- **M1 = web only** (D4). Staging deploys the Flutter **web** build. No Android/iOS/desktop (M6).
- **Reuse existing infra, don't reinvent it.** `docker-compose.yml` already defines postgres/redis/minio(+minio-init bucket bootstrap). The Makefile already has `up/down/test/lint/generate/seed/seed-real/run/check`. The backend already self-migrates on boot (`db.Migrate`, embedded `migrations/*.sql`, idempotent — safe to call on every startup, confirmed in `cmd/api/main.go`). This plan **extends** those; it does not fork a parallel setup.
- **Backend config is 100% env-var driven** (`internal/config/config.go`): `ENV` (`dev|staging|prod`), `PORT`, `DATABASE_URL`, `REDIS_URL`, `MEDIA_BASE_URL`, `JWT_SECRET`, `OTP_CHANNEL`, `TELEGRAM_GATEWAY_TOKEN`, `TELEGRAM_GATEWAY_URL`. Every value has a dev default; **staging MUST override the security-sensitive ones** (`ENV=staging`, a real random `JWT_SECRET`, a real `DATABASE_URL`/`REDIS_URL`/`MEDIA_BASE_URL`) and MUST set `OTP_CHANNEL=sandbox`. No secret is ever committed to the repo — staging secrets live in the deploy target's secret store / CI secrets, and only a **template** (`*.env.example` with placeholder values) is committed.
- **Content seeding reality (do not assume real content is available in a clean checkout):** the real avtoimtihon dataset lives in the untracked `aaa/` source tree and the gitignored `backend/seed/avtoimtihon/` (49MB images) — a clean checkout / CI runner **cannot** `make seed-real`. The only **self-contained** seed is the synthetic **NAMUNA** fixture (`make seed` → `cmd/genfixture` generates it from code, no external source; it includes 2 bilets, 40 questions, 4 categories, and — critically for the signs-catalog leg of the journey — a 4-sign catalog). **E2E in CI (Tasks 3-5) and the first staging bring-up (Task 6) MUST use the NAMUNA fixture.** Loading real content into staging is a separate, manual, one-time **operator** step from a machine that has the `aaa/` source (documented in Task 8's runbook), not part of CI or the automated deploy.
- **Anti-cheat stays enforced end-to-end** (backend-owned, client respects — unchanged since Plans 01/03/07): in `exam` mode the server withholds `correct`/`correct_answer_id` per-answer until finish. The E2E exam flow (Task 3) MUST assert the journey *works* without the client ever seeing per-answer correctness mid-exam — i.e. the test verifies the anti-cheat contract holds through the real stack, it does not try to defeat it.
- **VIP/free-tier gating stays enforced** (D13/backend): free = `variant` bilet #1 + `practice` (daily-limited) + signs catalog; VIP = bilet #2+, `exam`, `mistakes`. A default OTP signup is **non-VIP**; VIP is granted only via the `grantvip` CLI (`cd backend && go run ./cmd/grantvip -phone <p> -days <n>`). Task 4's E2E exercises both a non-VIP profile hitting the gates and (optionally) a `grantvip`-elevated profile passing them.
- **Commits:** conventional (`feat:`/`chore:`/`ci:`/`docs:`/`test:`) + Claude co-author trailer, direct to `main`, no branches (established convention). Infra commits use `ci:`/`chore:` as appropriate; docs use `docs:`.
- **CI discipline:** the existing `backend` and `frontend` jobs in `.github/workflows/ci.yml` stay green and untouched in spirit; the new `e2e` job is **additive** and must not slow or destabilize the existing two. If E2E proves flaky/slow, it may be gated to run on `main` merges + `workflow_dispatch` rather than every PR (state the choice — see Task 5).
- **Scope boundaries (hard):** NO real billing/checkout/Payme/Click (M2), NO GRAND MOCK (M2), NO guest-demo mode (M2), NO public SSG/Astro site (M2), NO admin panel (M3), NO real load-testing/k6/full monitoring-alerting (M7 — a *light* Lighthouse/health smoke is the ceiling here, see Task 7). This plan verifies and deploys **what M1 already built**, nothing more.

## File Structure (new additions this plan)

```
avtotest/
  backend/
    Dockerfile                        # multi-stage: build cmd/api -> minimal runtime image
    .dockerignore
    .env.example                      # committed template (placeholders only, no secrets)
  app/
    Dockerfile                        # multi-stage: flutter build web -> static server image
    .dockerignore
    nginx.conf                        # (or equivalent) static serving + SPA fallback + runtime API base URL
    web/env.js                        # runtime-injected API base URL (so one image works dev/staging)
    integration_test/
      m1_journey_test.dart            # Task 3: full learner happy-path (signup -> exam -> results -> explanation -> saved -> streak/stats)
      m1_gating_test.dart             # Task 4: VIP-gate (bilet#2/exam/mistakes) + free-tier daily practice limit
  deploy/
    docker-compose.staging.yml        # full staging stack (infra + backend + web), env from staging.env
    staging.env.example               # committed template (placeholders only)
    smoke.sh                          # Task 7: post-deploy health + API smoke (curl-level, reads OTP debug_code from JSON)
    ROLLBACK.md                       # Task 7: rollback runbook (image pin/revert)
  .github/workflows/
    ci.yml                            # + e2e job (Task 5)
    deploy-staging.yml                # Task 6: build+push images, deploy (workflow_dispatch; see D18)
```

*(Exact filenames/tool choices — nginx vs a tiny Go/`serve` static server, compose vs plain `docker run` on the host — are the implementer's call within each task, as long as the contract each task states is met. Prefer the least-moving-parts option.)*

---

### Task 1: Backend production Docker image

**Files:** create `backend/Dockerfile`, `backend/.dockerignore`, `backend/.env.example`.

**Interfaces (produced):** a runnable image that starts the API identically to `go run ./cmd/api`, reading all config from env vars, self-migrating on boot, serving `/healthz` and `/api/v1/**`.

**Logic:**
- Multi-stage build: stage 1 `golang:1.22` (or the repo's pinned Go version — check `backend/go.mod` for the exact `go` directive, don't guess) builds a static binary from `./cmd/api` (`CGO_ENABLED=0`); stage 2 a minimal runtime (`gcr.io/distroless/static` or `alpine` — pick one, justify briefly) copies just the binary. The embedded `migrations/*.sql` and sqlc code are compiled into the binary (`//go:embed` — confirmed in `internal/db/db.go`), so **no migration files need to be copied into the image** and no separate migrate step is needed; the API applies migrations on startup.
- Expose the port via `PORT` env (default 8080 inside the container — the host/compose maps it). `EXPOSE 8080`.
- `.dockerignore` excludes `seed/`, test data, git metadata, and anything large/irrelevant to keep the build context small.
- `.env.example`: a committed template listing **every** env var from `config.go` with **placeholder** values and a one-line comment each (`JWT_SECRET=CHANGE_ME_random_32+_bytes`, `OTP_CHANNEL=sandbox`, etc.). No real secret. This doubles as living documentation of the deploy contract.
- Do **not** bake secrets or a specific `DATABASE_URL` into the image — the image is environment-neutral; env is injected at run time.

**Testing:**
- `docker build -f backend/Dockerfile -t avtotest-api:local backend/` succeeds.
- Local run against the existing compose infra: `make up` (postgres/redis/minio), then `docker run --rm --network host -e PORT=8090 -e OTP_CHANNEL=sandbox avtotest-api:local` (or a compose service) — confirm it boots, logs `listening`/`migrate` cleanly, `curl localhost:8090/healthz` returns healthy, and `curl "localhost:8090/api/v1/variants"` returns the envelope (after `make seed`). Record the actual output.
- Confirm the image is reasonably small (single static binary — note the size).

- [ ] Steps 1-3 (Dockerfile, dockerignore, env template).
- [ ] **Step 4: Build + run smoke** as above; record real output.
- [ ] **Step 5: Commit.**
  ```bash
  git add backend/Dockerfile backend/.dockerignore backend/.env.example
  git commit -m "ci: backend production Docker image (self-migrating, env-driven)"
  ```

---

### Task 2: Flutter web production image + runtime-configurable API base URL

**Files:** create `app/Dockerfile`, `app/.dockerignore`, `app/nginx.conf` (or equivalent), `app/web/env.js`; modify `app/web/index.html` and the Dart networking bootstrap in `app/lib/main.dart` **only** as needed to read a runtime API base URL (see Logic — this is the one small, justified app-code touch this plan allows, and it must be minimal).

**Interfaces (produced):** a static-serving image that serves the `flutter build web` output as an SPA, with the backend API base URL configurable **at container run time** (not baked at build time), so **one** web image works for dev/staging/prod by env alone.

**Logic:**
- **The runtime-config problem, stated plainly:** `flutter build web` bakes `--dart-define=API_BASE_URL=...` at *build* time. Baking staging's URL into the image means rebuilding per environment — avoid that. Instead: ship a tiny `web/env.js` that sets `window.API_BASE_URL = "..."`, referenced from `index.html` before the Flutter bootstrap; the container's entrypoint rewrites `env.js` from an `API_BASE_URL` env var at start (a 2-line `sed`/`envsubst`). The Dart side reads `window.API_BASE_URL` (via `dart:js_interop`/`package:web`) with a fallback to the existing `String.fromEnvironment('API_BASE_URL', defaultValue: 'http://localhost:8090/api/v1')`. Keep the Dart change surgical — it only changes *where the base URL comes from*, not the Dio/interceptor stack (Plan 06 Task 2/7) which stays byte-for-byte otherwise. Add a unit/widget test asserting the resolution order (runtime `window` value wins; falls back to the dart-define default when absent).
- Multi-stage: stage 1 `flutter build web --release`; stage 2 nginx (or `busybox httpd`/a tiny static server) serving the build output with **SPA fallback** (`try_files $uri /index.html`) so go_router deep links work, plus sane cache headers (long-cache the hashed assets, `no-cache` on `index.html`/`env.js`).
- Reminder from the Environment note: the **release** web build is not `kDebugMode`, so the OTP dev-code caption won't render in this deployed image — that is correct/expected (Task 7's smoke reads the code from the API, not this UI).
- `.dockerignore` excludes `build/`, `.dart_tool/`, test dirs, etc.

**Testing:**
- `flutter build web --release` succeeds locally (via `dart analyze` clean + `flutter test --concurrency=1` for the new base-URL-resolution test first).
- `docker build -f app/Dockerfile -t avtotest-web:local app/` succeeds.
- `docker run --rm -p 5000:80 -e API_BASE_URL=http://localhost:8090/api/v1 avtotest-web:local` — confirm `curl localhost:5000` returns the app shell, `curl localhost:5000/some/deep/route` returns `index.html` (SPA fallback), and `curl localhost:5000/env.js` shows the injected URL. Record output.
- The base-URL-resolution test passes and genuinely discriminates (runtime value overrides the default).

- [ ] Steps 1-4 (env.js + index.html wiring + minimal Dart read + resolution test, then Dockerfile/nginx).
- [ ] **Step 5: Build + run smoke**; record real output.
- [ ] **Step 6: Commit.**
  ```bash
  git add app/Dockerfile app/.dockerignore app/nginx.conf app/web/ app/lib/main.dart app/test/
  git commit -m "ci: Flutter web production image + runtime-configurable API base URL"
  ```

---

### Task 3: E2E — full M1 learner happy-path journey

**Files:** create `app/integration_test/m1_journey_test.dart`. Reuse the existing `app/test_driver/integration_test.dart` driver (from Plan 06 Task 9 — do not create a second driver).

**Interfaces (produced):** one headless, real-stack integration test covering the M1 learner happy path end-to-end, runnable via the established `chromedriver` + `flutter drive ... -d web-server --browser-name=chrome` recipe against a real backend seeded with the NAMUNA fixture.

**Logic — the journey (all against the real backend, NAMUNA seed, `OTP_CHANNEL=sandbox`):**
1. **Signup/OTP:** enter a fresh test phone on `PhoneEntryScreen` → request OTP → read the dev `debug_code` shown on `OtpVerifyScreen` (visible because `flutter drive` = debug build) → verify → land on `HomeShell` with a real (auto-created) profile. (This leg mirrors Plan 06's `auth_flow_test` — reuse its approach, don't reinvent.)
2. **Browse content:** open Variants (bilet grid renders, bilet #1 unlocked/free); open Signs catalog (renders the NAMUNA sign catalog — free-tier, no gate); open Practice setup.
3. **Full exam-mode session with anti-cheat:** start an `exam` session → answer through the questions → **assert the UI never shows per-answer correctness mid-exam** (anti-cheat holds through the real stack) → reach finish (or trigger the natural stop) → land on results.
4. **Results + explanations:** results screen shows score/status; open an explanation where available (NAMUNA fixture ships verified explanations post Plan 07's fixture-schema fix — confirm the seeded explanation renders, not a generic error).
5. **Save a question:** toggle save on a question → confirm it reflects saved state (and appears in the saved list).
6. **Streak/stats:** after answering, the Stats/Streak surface reflects the attempt (mastery/readiness present; streak incremented per the UTC-day semantics — do not assert an exact wall-clock date, assert presence/increment consistent with the backend's documented UTC-day behavior).

**Testing / running:**
- Local: start the stack (backend on `PORT=8090` + `make up` + `make seed`), start `chromedriver` on its port, run `flutter drive --driver=test_driver/integration_test.dart --target=integration_test/m1_journey_test.dart -d web-server --browser-name=chrome`. Record the real pass output.
- **Apply Plan 06/07's seam lesson at journey scale:** this test exists precisely to catch the class of bug that only appears when real screens + controllers + router + backend compose (Plan 06 found several: post-action navigation that never fired, concurrent-fetch token races). Prefer asserting **navigation actually landed** and **data actually rendered from the backend** at each leg, not just that a widget exists.
- Keep it deterministic: a fresh phone per run (or a cleanup) so signup doesn't collide with prior state; the E2E DB is ephemeral (Task 5) so a fresh CI run starts clean.

- [ ] Steps 1-3 (write the flow, run locally green, record output).
- [ ] **Step 4: Commit.**
  ```bash
  git add app/integration_test/m1_journey_test.dart
  git commit -m "test(e2e): full M1 learner happy-path journey (signup->exam->results->explanation->saved->stats)"
  ```

---

### Task 4: E2E — VIP gating + free-tier daily limit

**Files:** create `app/integration_test/m1_gating_test.dart`.

**Interfaces (produced):** a headless real-stack integration test covering the two enforcement surfaces the happy path doesn't hit: VIP gating and the free-tier daily practice limit.

**Logic (real backend, NAMUNA seed, `OTP_CHANNEL=sandbox`):**
- **VIP-gate (non-VIP profile):** sign up a fresh (non-VIP) profile → attempt a VIP-gated action (start bilet **#2**, or `exam`, or `mistakes`) → assert the app routes to the `VipRequiredScreen` (Plan 07 Task 9), **not** a generic error banner, and offers a way back (not a dead end). This confirms the `402 vip_required` path holds through the real stack and the UI reacts specifically.
- **Free-tier daily practice limit:** as the same non-VIP profile, run `practice` sessions until the backend's configured free daily limit (`daily_practice_questions`, NAMUNA/config default) is exhausted → assert the next attempt surfaces the specific `429 daily_limit_reached` "kunlik limit" message (informational per D13, not an alarming error), distinct from the VIP paywall.
- **(Optional, if it fits cleanly) VIP-granted pass:** elevate the profile via `cd backend && go run ./cmd/grantvip -phone <that phone> -days 30`, then re-attempt a previously-gated action and assert it now proceeds. This proves the gate opens for entitled users, not just that it closes for others. If the CLI step is awkward to orchestrate from the E2E harness, document it as a manual companion check rather than forcing it into the automated flow — say so explicitly.

**Testing / running:** same `chromedriver` + `flutter drive` recipe as Task 3, targeting `m1_gating_test.dart`. Record real pass output. Assert the **distinctness** of the two failure UIs (paywall vs daily-limit) — Plan 07 was careful never to collapse these; the E2E confirms that end-to-end.

- [ ] Steps 1-3 (write, run locally green, record output).
- [ ] **Step 4: Commit.**
  ```bash
  git add app/integration_test/m1_gating_test.dart
  git commit -m "test(e2e): VIP-gate + free-tier daily practice limit journeys"
  ```

---

### Task 5: CI — E2E job wiring

**Files:** modify `.github/workflows/ci.yml` (add an `e2e` job). Do not disturb the existing `backend`/`frontend` jobs.

**Interfaces (produced):** a CI job that stands up the full stack (Postgres + Redis + backend + NAMUNA seed + chromedriver + Flutter web) and runs Tasks 3-4's flows headlessly, gating merges to `main`.

**Logic:**
- New `e2e` job on `ubuntu-latest` with `postgres:16-alpine` and `redis:7-alpine` **service containers** (mirror the `backend` job's postgres service block — same creds/health-check; this is a **fresh ephemeral DB per run**, satisfying the "own DB, never the shared test DB" constraint). MinIO/media: the NAMUNA fixture references image keys but the journey flows don't strictly need images served to pass logic assertions — if an image URL is needed, point `MEDIA_BASE_URL` at a stub/placeholder rather than standing up MinIO in CI (keep the job lean; state the choice).
- Steps: checkout → set up Go (build+run the API with `ENV=staging`-like test config: `OTP_CHANNEL=sandbox`, a throwaway `JWT_SECRET`, the service `DATABASE_URL`/`REDIS_URL`) → apply the NAMUNA seed (`go run ./cmd/genfixture ...` + `go run ./cmd/importer ...`, i.e. `make seed`'s commands — real content is unavailable in CI per Global Constraints, this is required) → start the API in the background on a known port → set up Flutter (`subosito/flutter-action@v2`, stable) → `flutter pub get` → `dart run build_runner build --delete-conflicting-outputs` → start `chromedriver` → `flutter drive` each of `m1_journey_test.dart` and `m1_gating_test.dart` → fail the job on any flow failure. Note CI paths are ASCII, so `flutter analyze`/`flutter test` gotchas don't apply here (bare forms are fine in CI, as already true for the `frontend` job).
- **Trigger choice (decide and state it):** run the `e2e` job on **push to `main`** and on **`workflow_dispatch`** (manual), but **not** on every PR by default — reasoning: E2E is heavier and browser-driven; gating every PR risks flaky-failure friction on a solo/small-team direct-to-main workflow, while `main`-push coverage still catches regressions before they can reach staging, and `workflow_dispatch` lets anyone run it on demand. (If the user prefers PR-gating for maximum safety, it's a one-line `on:` change — note this in the job comment.) This is a deliberate, reversible choice, stated per the plan brief's instruction to pick and justify.
- Keep the job resilient: give the backend a readiness wait (`until curl -sf localhost:<port>/healthz; do sleep 1; done`) before driving, so the E2E doesn't race a not-yet-listening API.

**Testing:** validate the workflow YAML (`actionlint` if available, or a careful diff against the working `backend` job's service block). Since a green CI run can only be observed by pushing, the task's local evidence is: the exact same commands the job runs, executed locally end-to-end (Tasks 1-4 already established each piece works locally) — record that the local equivalent of the job passes. Note explicitly that the first real CI run is the true confirmation and should be watched after push.

- [ ] Steps 1-2 (write the job; dry-run the equivalent commands locally, record output).
- [ ] **Step 3: Commit.**
  ```bash
  git add .github/workflows/ci.yml
  git commit -m "ci: E2E job — full-stack chromedriver-driven M1 journey on main"
  ```

---

### Task 6: Staging environment definition + deploy workflow

> **BLOCKED on Open Decision D18** — confirm the deploy target with the user before executing. The task is written against the recommended default (single Docker host running the compose stack); the flagged values change if the user picks a different target.

**Files:** create `deploy/docker-compose.staging.yml`, `deploy/staging.env.example`, `.github/workflows/deploy-staging.yml`.

**Interfaces (produced):** a reproducible staging stack definition + a deploy mechanism that builds/pushes the Task 1/2 images and brings the stack up on the staging host.

**Logic:**
- `deploy/docker-compose.staging.yml`: the full staging stack in one file — `postgres` (with a **named volume** for persistence, unlike CI's ephemeral service), `redis`, `minio`+`minio-init` (reuse the exact bucket-bootstrap from the root compose), the **backend** image (Task 1) with env from `staging.env`, and the **web** image (Task 2) with its `API_BASE_URL` pointing at the staging backend's public URL. The backend self-migrates on boot, so no separate migrate step — bringing the stack up **is** the migration path (state this: the staging Postgres migration path is "app applies embedded migrations on container start", same code as dev, no drift risk).
- `deploy/staging.env.example`: committed template (placeholders only) for every backend env var: `ENV=staging`, `PORT`, `DATABASE_URL` (staging Postgres), `REDIS_URL`, `MEDIA_BASE_URL` (staging MinIO/CDN public base), `JWT_SECRET=CHANGE_ME`, `OTP_CHANNEL=sandbox`, Telegram vars empty. **The real `staging.env` is never committed** — it lives on the host (or is materialized from CI secrets at deploy time). Document where the real values come from.
- `.github/workflows/deploy-staging.yml`: triggered by **`workflow_dispatch`** (manual, deliberate — reasoning: staging is not user-facing, has no billing/real users, and a manual gate avoids surprise deploys and gives a human the decision point; auto-deploy-on-merge is easy to switch to later once staging is trusted, and this is called out in the workflow comment). Steps: build the backend + web images → push to a registry (GHCR by default — `ghcr.io/<owner>/avtotest-api|web` — provider-agnostic and free for the repo; the user's D18 choice may substitute another registry) → connect to the staging host over SSH (host/user/key from CI secrets `STAGING_HOST`/`STAGING_SSH_KEY`, materialized per D18) → `docker compose -f docker-compose.staging.yml pull && up -d` → wait for `/healthz` → done. **Every host-specific value (registry, SSH target, public URL) is a CI secret / placeholder, never hardcoded** — the workflow is committable without leaking anything, and the concrete values are filled in once D18 is confirmed.
- Explicitly gate the E2E-then-deploy relationship: staging deploys **only** the images built from `main`; the `e2e` job (Task 5) having passed on `main` is the pre-deploy quality bar (note this dependency, but keep the deploy manual per the trigger choice above).

**Testing:** YAML validity; a **local dry-run** of the staging compose stack — `docker compose -f deploy/docker-compose.staging.yml --env-file <a local throwaway staging.env> up -d` on the dev machine (treating localhost as the "host"), confirm all services come healthy, the backend migrates + serves `/healthz`, and the web container serves the app pointed at the local backend. This proves the stack definition is correct **without** needing the real remote host (which awaits D18). Record output. The remote deploy workflow itself can only be fully exercised once a host exists (Task 8) — note that.

- [ ] **Step 0: Confirm D18** is answered; if not, stop and ask the user.
- [ ] Steps 1-3 (compose, env template, deploy workflow).
- [ ] **Step 4: Local dry-run** of the staging compose; record output.
- [ ] **Step 5: Commit.**
  ```bash
  git add deploy/docker-compose.staging.yml deploy/staging.env.example .github/workflows/deploy-staging.yml
  git commit -m "ci: staging stack definition + manual deploy workflow (see D18)"
  ```

---

### Task 7: Post-deploy smoke check + rollback runbook

**Files:** create `deploy/smoke.sh`, `deploy/ROLLBACK.md`.

**Interfaces (produced):** a runnable post-deploy smoke script and a written rollback procedure — the "did the deploy actually work, and what do we do if it didn't" safety net.

**Logic:**
- `deploy/smoke.sh <base_url>`: a curl-level smoke that runs after every deploy (and can be wired as the final step of `deploy-staging.yml`). It MUST (a) `GET /healthz` → healthy; (b) `GET /api/v1/variants` → valid `{"data":...}` envelope with content present (proves DB + seed); (c) exercise the **auth API path** at the API level — `POST /api/v1/auth/otp/request {phone}` → read `debug_code` **from the JSON** (this is why staging uses `OTP_CHANNEL=sandbox`; the release web UI does NOT show it, so the smoke works at the API, not the UI) → `POST /auth/otp/verify` → get tokens → `GET /me` with the bearer → profile returns. Exit non-zero on any failure so a bad deploy is loud, not silent. Keep it dependency-light (curl + `jq`).
- Add a **light** web-reachability check (the deployed web container returns the app shell + SPA fallback works) — but **no** heavy performance/load testing here (that's M7). At most an optional single Lighthouse budget note if trivially available; do not build a load harness.
- `deploy/ROLLBACK.md`: the rollback procedure for the D18-default (image-tag) shape — deploys are immutable image tags (git SHA / semver), so rollback = re-deploy the previous known-good tag: `docker compose -f docker-compose.staging.yml pull <prev tag> && up -d`, then re-run `smoke.sh`. Cover: how tags are named, how to find the last-good tag, the DB-migration caveat (migrations are **forward-only** via golang-migrate `Up`; a rollback of the *app image* does not auto-revert a schema migration — so an image rollback across a migration boundary is safe only if the older binary tolerates the newer schema; for M1's additive migrations this is fine, but state the caveat and the escape hatch of the committed `*.down.sql` files for a manual, deliberate schema rollback if ever needed). Keep it honest and concrete, not aspirational.

**Testing:** run `smoke.sh` against the **local** staging dry-run stack from Task 6 (localhost base URL) — confirm every check passes and that it correctly **fails** if the backend is down (flip it off, confirm non-zero exit). Record both the pass and the deliberate-failure output (proves the smoke actually discriminates, per this project's "does the check genuinely fail when it should" standard). `ROLLBACK.md` is prose — verify its commands are copy-pasteable against the Task 6 compose file.

- [ ] Steps 1-2 (smoke script + runbook).
- [ ] **Step 3: Test** smoke pass + deliberate-failure locally; record output.
- [ ] **Step 4: Commit.**
  ```bash
  git add deploy/smoke.sh deploy/ROLLBACK.md
  git commit -m "ci: post-deploy smoke check + rollback runbook"
  ```

---

### Task 8: Full verification + live staging deploy + docs

> The **live remote deploy** portion is BLOCKED on Open Decision D18 (a provisioned host). If D18 is unconfirmed/unprovisioned at this point, complete every host-independent part (Steps 1-2, 4) and record the remote deploy (Step 3) as a documented **operator runbook to execute once the host exists**, with the exact commands — do not fabricate a deploy that didn't happen.

- [ ] **Step 1: Full local verification.**
  - App: `cd "/home/sher/Рабочий стол/avtotest/app" && dart analyze` (0 issues), `flutter test --concurrency=1` (all pass, incl. Task 2's base-URL-resolution test), `flutter build web --release` (succeeds).
  - Backend: `cd "/home/sher/Рабочий стол/avtotest" && make check` (lint + test green — run this when no other backend agent is touching the shared test DB, per the Environment note).
  - Images: `docker build` both images green.
  - E2E: run both `flutter drive` flows (Tasks 3-4) locally against the real seeded backend; record the real pass output.
  - Staging stack: `docker compose -f deploy/docker-compose.staging.yml up -d` local dry-run healthy; `deploy/smoke.sh` passes against it.

- [ ] **Step 2: Watch the first CI `e2e` run** after the Task 5 push actually lands on `main` (the true confirmation the job works in CI, not just locally). Record the run result. Fix forward if it's flaky (per the seam-bug discipline — a real E2E flake is often a real race, as Plan 06 repeatedly found).

- [ ] **Step 3: Live staging deploy** (requires D18-provisioned host + secrets set):
  1. Set the CI secrets (`STAGING_HOST`, `STAGING_SSH_KEY`, registry creds, real `staging.env` values incl. a real random `JWT_SECRET`, `OTP_CHANNEL=sandbox`).
  2. Trigger `deploy-staging.yml` (`workflow_dispatch`).
  3. Confirm the stack comes up on the host, `smoke.sh` passes against the **public** staging URL, and the deployed **web** URL is reachable and drives the M1 journey by hand (signup via the sandbox flow — reading `debug_code` from the API since the release UI hides it, then verifying in the UI).
  4. Seed content: for a usable staging, run the NAMUNA seed on the host (self-contained), OR — if real content is wanted and the operator has the `aaa/` source on a machine with host DB access — run the real-content import as a documented one-time operator step (`make seed-real`, per README). State which was used.
  5. Record every command + response. If the host isn't provisioned, write this as the runbook to execute later, verbatim.

- [ ] **Step 4: README.** Add a "Staging / deploy (M1 Plan 08 holati)" section covering: the two Docker images and how to build them; the env-var/secrets contract (pointing at `.env.example`/`staging.env.example`); the staging compose stack + how to bring it up; the deploy workflow + its manual `workflow_dispatch` trigger and D18 dependency; `smoke.sh` + `ROLLBACK.md`; the CI `e2e` job (what it covers, its trigger); the content-seeding reality (NAMUNA in CI/first-boot, real content as a manual operator step); and an explicit note on what staging is **not** — no billing/checkout, no GRAND MOCK, no guest-demo, no public SSG site, no admin (all M2/M3), no load-testing/alerting (M7). Cross-reference D18 as the open decision.

- [ ] **Step 5: Commit.**
  ```bash
  git add README.md
  git commit -m "docs: staging/deploy setup + E2E verification (M1 Plan 08)"
  ```

## Self-Review

1. **Spec coverage:** master spec §23 step 12 ("E2E + performance + staging deploy") is covered — E2E by Tasks 3-5 (CI-wired full-journey + gating flows, generalizing the Plan 06/07 `flutter drive` recipe), staging deploy by Tasks 6-8 (images, env/secrets contract, staging Postgres on the self-migrating path, a deploy workflow, post-deploy health/smoke, rollback). "Performance" is deliberately kept to a **light** health/reachability smoke (Task 7) — full k6 load-testing + monitoring/alerting are §21/§22/§7-scoped to **M7**, not pulled forward. §22's "muhitlar: dev (compose), staging, prod" and "migratsiya avtomatik (golang-migrate)" are honored: staging reuses the dev compose shape and the app's own embedded-migration path (no schema drift between environments).
2. **Scope boundaries (honest, not gaps):** explicitly OUT and called out as later-milestone, not silently missing — real billing/checkout/Payme/Click, GRAND MOCK, guest-demo mode, public SSG/Astro site (all **M2**); admin panel (**M3**); real load-testing/k6/full monitoring-alerting (**M7**). Staging uses the **sandbox** OTP channel and holds only test data — no real PII, consistent with billing being absent in M1. This plan verifies and deploys exactly what M1 (Plans 01-07) already built, and adds only the operational scaffolding to do so.
3. **Open decision, surfaced not assumed (D18):** the deploy target is genuinely undecided in-repo (no Dockerfile/PaaS/cloud config exists — verified). Rather than inventing a provider, this plan flags **D18** (following the D1–D17 convention), recommends the prod-shape-reusing single-Docker-host default **with reasoning**, lists real alternatives, and makes Tasks 1-5 fully target-agnostic so they proceed regardless — only Tasks 6-8's concrete destination waits on the user's confirmation. Every host-specific value is a placeholder/secret, so nothing in the committed files leaks or presumes a provider.
4. **Reuse over reinvention (matches the prior-plans discipline):** the backend already self-migrates and is fully env-driven (verified in `config.go`/`cmd/api/main.go`) — the image is thin and the migration "path" is the existing boot code, no new migrate tooling. The E2E recipe is the exact `chromedriver` + `flutter drive -d web-server --browser-name=chrome` recipe Plans 06/07 established (reused, not rediscovered), and the CI job mirrors the working `backend` job's Postgres service block. The NAMUNA-fixture constraint reflects the real, verified fact that real content is unavailable in a clean checkout/CI.
5. **Seam/integration discipline (Plan 06/07's biggest recurring lesson) applied at deploy scale:** the whole point of Tasks 3-5 is to exercise real screens + controllers + router + backend + DB composed together through the real network — precisely the boundary where Plan 06 repeatedly found bugs that isolated tests missed (unfired navigation, concurrent-fetch token races). Task 8 Step 2 explicitly treats a first-CI-run E2E flake as a probable real race to fix forward, not to retry-until-green. The smoke check (Task 7) is required to demonstrably **fail** when the backend is down, so it genuinely discriminates.
6. **Environment gotchas respected:** `dart analyze` (not `flutter analyze`), `flutter test --concurrency=1` (not bare), `PORT=8090` dev backend, and the shared-`avtotest_test`-DB contention rule (E2E stands up its own ephemeral DB, never the shared test DB; local backend verification not run concurrently with other backend agents) — all carried forward from Plans 06/07 and the progress ledger's operational notes.
7. **Commit hygiene:** conventional commits with the appropriate `ci:`/`test:`/`docs:` types + Claude co-author trailer, direct to `main`, no branches — established convention. Each task commits its own additive slice; no task leaves a half-wired seam (the one small app-code touch, Task 2's runtime base URL, is fully wired + tested within its own task).
