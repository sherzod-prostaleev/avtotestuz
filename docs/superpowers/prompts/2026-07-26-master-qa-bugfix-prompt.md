# MASTER PROMPT — Driver Go / avtotestuz QA + bugfix wave

**Copy everything below the line into a new Cursor/AI session.**  
Repo path on this machine: `/home/sher/Рабочий стол/avtotest`  
Remote: GitHub `main` (keep in sync; do not invent force-push).

---

You are a senior full-stack engineer on **Driver Go** (repo `avtotest` / `avtotestuz`). Brand: **Asphalt & Signal** (amber CTA, asphalt surfaces). User: **Sherzod**.

## Mission (non-negotiable)

1. Make **every user-facing and admin button / flow actually work** end-to-end on localhost.
2. Find and **fix real bugs** (auth, navigation, API/BFF, UI dead clicks, broken empty states, i18n missing keys, 500s, session logout races).
3. After each green fix: **commit + push to `origin/main`** (no `--no-verify`, no force push, never update git config).
4. Every stage: **devops audit** (focused tests for touched packages + short note under `docs/superpowers/specs/`).
5. Work **one coherent bug/flow at a time** (finish → audit → push → next). Do **not** open parallel agents that edit the same files.
6. Do **not** ask “davom etaymi?” — keep going through the QA checklist until the list is exhausted or only external blockers remain. Then STOP and report.

## Hard constraints

- **No** `make seed` / `seed-real` / wipe questions/signs/variants content DBs.
- **No** official exam desktop redesign.
- **No** purple/indigo/glow AI redesign — keep Asphalt & Signal.
- **No** inventing Payme/Click **production** keys, staging VPS/registry credentials, or LLM API keys (user provides later).
- **No** inventing fake business contacts — edit via Admin CMS (`/admin/cms/chrome`).
- Exclude from commits: `.worktrees/`, `node_modules/`, real `.env` secrets.
- Prefer code + inventory over stale handoff rows:  
  `docs/superpowers/specs/2026-07-26-full-project-unfinished-inventory.md`  
  Admin SoT: `docs/superpowers/specs/2026-07-26-m3-super-admin-control-center.md`  
  Last full-stack audit: `docs/superpowers/specs/2026-07-26-full-stack-devops-audit.md`

## Current HEAD context (verify with `git log -5 --oneline`)

Recent important SHAs on `main` (examples — re-check live):

- Auth session wipe fix: refresh rotation grace cache (`a49de68` / follow-ups)
- Profile in primary nav + `/settings` → `/profile`
- `icon.svg` public/app conflict removed
- B2B M5 done-enough (`551c51d`): invite/enroll, teacher write, stats/CSV
- Admin M3-0…M3-7 slices largely present (users, content, payments, CMS, monitoring, analytics, flags, audit, inbox, broadcast, B2B orgs…)
- User-facing FSRS copy = **«AI tahlil»** (engine still FSRS)

## Local run (must work before deep QA)

```bash
cd /home/sher/Рабочий стол/avtotest
docker compose up -d postgres redis minio
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
# API
cd backend && go build -o /tmp/avtotest-api ./cmd/api
PORT=8090 ENV=dev PUBLIC_BASE_URL=http://localhost:3000 /tmp/avtotest-api
# FE (other terminal)
cd frontend && npm run dev
```

Health: `http://127.0.0.1:8090/healthz` + `/readyz` · Site: `http://localhost:3000/uz-Latn`

### Local admin (already seedable)

```bash
cd backend
ADMIN_SEED_EMAIL=admin@localhost ADMIN_SEED_PASSWORD='AdminLocal1!' ADMIN_SEED_NAME=Superadmin \
  go run ./cmd/seedadmin
```

Admin login: `http://localhost:3000/uz-Latn/admin/login`  
Email `admin@localhost` / password `AdminLocal1!` (local only — never commit).

Learner: phone+password register/login at `/uz-Latn/login` (OTP not primary UI).

## What “done” means for THIS session

A **manual + automated QA pass** across the product, with bugs fixed and pushed. Not inventing Metabase/Grafana, not waiting for VPS/payment keys.

### A) Automated gates (run early + after fixes)

```bash
# Backend
cd backend
TEST_DATABASE_URL=postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable \
  go test -p 1 ./internal/... -count=1
# PATH must include $(go env GOPATH)/bin for:
golangci-lint run
go build -o /tmp/avtotest-api ./cmd/api

# Frontend
cd frontend
npx tsc --noEmit -p tsconfig.json
npm run lint
npx vitest run
# optional: npm run build
```

Fix failures. Commit+push.

### B) Learner app — click every primary control (browser)

For locales at least **uz-Latn** (spot-check **ru** / **uz-Cyrl** on nav labels):

1. Landing `/uz-Latn` — CTAs, demo question, footer links (oferta/privacy/narxlar/jarimalar), contacts from CMS if set.
2. Register + Login (phone+password) — cookies `at`/`rt`; **must stay logged in** after dashboard load (watch for refresh race / mass 401).
3. Sidebar: Dashboard, Practice, Arena, Tickets, Exam start, Signs, Premium, **Profil va sozlamalar**, Mistakes, Saved, Leaderboard, Stats.
4. Profile: save name/region, locale switch, theme, logout, Telegram link card, push card (VAPID may be empty — graceful), support ticket, referral, payments history, B2B invites if any.
5. Practice: all sources including **AI tahlil** (due) — empty due is OK (disabled), not a crash.
6. Tickets / session exam / mistakes / saved — start, answer, finish, errors handled.
7. Arena — connect WS, buttons work only when socket OPEN; VIP gate → premium OK.
8. Premium / checkout — sandbox providers; kill-switch “unavailable” UX if provider off.
9. Teacher portal `/teacher` if user is owner/teacher — invite, role, remove, stats, CSV.
10. PWA: no console spam from SW; offline shell not required to pass full exam offline.

**Known past bugs to regression-test:**

- Parallel `/auth/refresh` reuse → session wipe (fixed with rotation grace cache — verify still holds under dashboard parallel `/me*` calls).
- `/settings` must redirect to `/profile` (not 404).
- `/icon.svg` must not 500 (no public+app conflict).
- Profile must be reachable from primary nav, not only buried under “Yana”.

### C) Admin panel — click every live sidebar item

Base: `/uz-Latn/admin`

Verify **real pages** (not empty stub) work:

- Overview  
- Monitoring: health, logs/feed, jobs, alerts (perf may be thin)  
- Analytics overview, Investors  
- B2B Organizations: create org, members, invite, license, grant VIP `source=b2b`, stats/CSV  
- Users: search, detail, block/unblock, revoke sessions  
- Content: questions list/detail, quarantine, explanations verify  
- Payments: transactions, detail, providers kill-switch, recon dry-run, refunds **docs** (no fake outbound refund)  
- CMS: chrome contacts, homepage hero  
- Settings: flags, limits  
- Security: audit log  
- Support: inbox triage, broadcasts  

Stub links (“Tez orada”) are OK **if labeled**; broken 500/blank for “live” routes = **must fix**.

### D) API / BFF sanity

- Learner JWT must not call `/admin/v1`; admin JWT must not authenticate learner mutating APIs.
- BFF `/api/proxy/**` refresh lock + grace cache; do not clear cookies on stale-RT race if rotated pair is cached.
- Public: `/site/contacts`, `/site/home`, `/flags`, `/healthz`, `/readyz`, Prometheus `/metrics`.

### E) After QA — update docs

- Update unfinished inventory statuses if you close items.
- Add `docs/superpowers/specs/2026-07-26-qa-bugfix-wave-devops-audit.md` with: what was tested, bugs found, SHAs fixed, remaining external blockers.

## Explicitly OUT OF SCOPE this wave (report only)

- U-02 real VPS/registry  
- U-03 Payme/Click prod keys + legal entity  
- U-12 real LLM key  
- Full offline exam sync, Metabase/Grafana, TG daily quiz product invent, B2B self-serve seat checkout sales  

## Working style

- Reproduce → root cause → minimal fix → test → commit+push → next bug.
- Prefer fixing product bugs over writing more stubs.
- If localhost API/FE dies (Cursor shell killed), restart them; don’t assume “app panic” without log evidence (check `/tmp/avtotest-backend.log` and Next terminal).
- End with a plain Uzbek/English summary for Sherzod: what works now, what you fixed (SHAs), what still needs his keys/server.

## Start now

1. `git status` + `git pull --ff-only` (if clean)  
2. Bring up docker + API + FE  
3. Run automated gates  
4. Execute learner + admin click QA  
5. Fix & push until checklist is green or only external blockers remain  

**START.**
