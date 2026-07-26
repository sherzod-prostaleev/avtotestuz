# 2026-07-26 QA + bugfix wave — devops audit

**Repo:** avtotest / Driver Go  
**Date:** 2026-07-26 (session continuation)  
**Focus:** Full learner + admin click QA; admin block must kill learner auth

---

## What was tested

### Automated gates
| Gate | Result |
|------|--------|
| `go test -p 1 ./internal/... -count=1` (TEST_DATABASE_URL) | **PASS** (early wave) |
| `go test ./internal/auth ./internal/server` (this fix) | **PASS** |
| `golangci-lint` (auth/server + full earlier) | **PASS** (0 issues) |
| `go build ./cmd/api` | **PASS** |
| `npx tsc --noEmit` | **PASS** |
| `npm run lint` | **PASS** |
| `npx vitest run` (350 tests early; proxy+i18n after fix) | **PASS** |

### Local stack
- `docker compose` postgres/redis/minio: up
- API `:8090` `/healthz` + `/readyz`: ok
- FE `:3000`: ok
- Admin seed `admin@localhost` ready

### Public / API sanity
- `/api/v1/site/contacts`, `/site/home`, `/flags`, `/metrics`, `/healthz`, `/readyz` → 200
- `/icon.svg` → 200
- `/uz-Latn/settings` → 307 → `/profile`
- Learner JWT ≠ admin (`/admin/v1` → 401); admin JWT ≠ learner mutate (`/api/v1/me` → 401)
- Parallel `/api/proxy/me` ×8 after register → all 200; cookies held (refresh race OK)

### Learner (browser + API)
- Landing CTAs + footer (oferta/privacy/narxlar/jarimalar) → 200
- Dashboard sidebar: Tickets (62), Practice, Arena, Exam→`session/start?mode=exam`, Signs, Premium, **Profil va sozlamalar** in primary nav, Ko‘proq extras
- Practice: AI tahlil due empty → disabled (OK); category start → session Q&A works
- Arena: VIP surfaces + matchmaking controls when connected
- Premium: tariffs + Payme/Click picker; provider kill-switch toggles via admin
- Profile: save name/region; Telegram / web-push graceful when VAPID off (`configured:false`); support ticket → admin inbox; referral + payments history
- Leaderboard / mistakes / saved / stats / teacher (empty orgs) → load
- PWA `sw.js` → 200; no SW spam required for pass

### Admin (browser + BFF)
- Login `admin@localhost` → overview (uz-Latn)
- Live routes smoke 200: monitoring (health/feed/jobs/alerts), analytics, investors, B2B orgs, users, content questions/explanations, payments (tx/providers/recon/refunds), CMS chrome/home, flags/limits, audit, support inbox/broadcast
- Stub links labeled «Tez orada» → stub page 200 (not 500)
- Users catalog + block/unblock; CMS contacts PUT → public `/site/contacts`
- Support inbox shows learner ticket from profile form

---

## Bugs found & fixed

| Bug | Root cause | Fix |
|-----|------------|-----|
| Admin «block» did not stop learner login; live AT still called `/me` | Login/Refresh/OTP never checked `profile.status`; JWT middleware ignored `banned` | `ErrAccountBlocked` on login/refresh/OTP/issueSession; `RejectBanned` after `auth.Required` on all learner routes; BFF clears cookies on `403 account_blocked`; login i18n `errorAccountBlocked` |
| Admin overview copy still said Inbox is next | Stale AdminOverview body | Update uz-Latn/uz-Cyrl/ru: inbox works; RBAC still next |

### Prior wave SHAs (already on main)
- `a1b5df6` — wrong-answer flicker + locale FOUC + Uzbek admin nav
- `556675c` — PWA SW hydration in development

---

## SHAs (this session)

- `e5f8afe` — fix(auth): enforce admin block on learner login and live sessions

### Prior wave SHAs (already on main)
- `a1b5df6` — wrong-answer flicker + locale FOUC + Uzbek admin nav
- `556675c` — PWA SW hydration in development

---

## Remaining external blockers (out of scope)

- **U-02** real VPS / registry credentials  
- **U-03** Payme/Click production merchant keys + legal entity  
- **U-12** real LLM API key  
- Full offline exam sync, Metabase/Grafana, TG daily quiz product invent, B2B self-serve seat checkout  
- Admin stubs: content tickets/signs studios, legal CMS, runtime config, admins/RBAC UI  
- Web push delivery needs `VAPID_*` ops keys  

---

## Notes

- Public site paths are under `/api/v1/...` (not bare `/site/...`).
- Browser may keep an old PWA SW; unregister once if admin labels look stale.
- After API rebuild, keep a long-lived process (`exec /tmp/avtotest-api`); short-lived shell `&` can die with the parent.
