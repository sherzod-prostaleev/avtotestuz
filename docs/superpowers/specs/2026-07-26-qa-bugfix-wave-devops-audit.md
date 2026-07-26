# 2026-07-26 QA + bugfix wave — devops audit

**Repo:** avtotest / Driver Go  
**Date:** 2026-07-26  
**Focus:** Learner session UX (wrong-answer flicker + locale FOUC) + Admin panel uz-Latn i18n

---

## What was tested

### Automated gates
| Gate | Result |
|------|--------|
| `go test -p 1 ./internal/... -count=1` (TEST_DATABASE_URL) | **PASS** |
| `go build ./cmd/api` | **PASS** |
| `npx tsc --noEmit` | **PASS** |
| `npm run lint` | **PASS** |
| `npx vitest run` (350 tests) | **PASS** |

### Local stack
- `docker compose` postgres/redis/minio: up
- API `:8090` `/healthz` + `/readyz`: ok
- FE `:3000`: ok
- Admin seed `admin@localhost` ready

### Manual / browser
- Register phone+password → dashboard (session cookies hold)
- Tickets → start bilet 1 → select wrong answer → feedback paints without full-window shake
- Dark theme locale switch during active session (`uz-Latn` → `ru`): **0 bright flashes**, no loading wipe, soft reload keeps session painted
- `/settings` → 307 (redirect to profile)
- Public: landing, login, narxlar, oferta, privacy, jarimalar → 200
- Admin shell: sidebar + overview fully localized in uz-Latn after SW cache clear

---

## Bugs found & fixed

| Bug | Root cause | Fix |
|-----|------------|-----|
| Wrong answer “pir-pir” full-screen flicker | `answer-wrong-shake` used `transform` shake on full-width answer → sibling/image composite flash | Replace with `answer-wrong-pulse` (box-shadow only) + `contain: paint` |
| Dark theme language switch white flash | Locale remounts `<html>`; ThemeProvider not yet hydrated → `:root` light colors; session `loadSession` wiped to `null` → loading spinner | FOUC blocking script + `html` dark default paint; soft `loadSession` when same session id; `startTransition` + `router.replace` for locale switches |
| Admin sidebar English on uz-Latn | Hardcoded English labels in `admin-sidebar.tsx` | `AdminNav` / `AdminOverview` i18n namespaces; translate page titles in uz-Latn Admin* keys |

---

## SHAs (this wave)

- `a1b5df6` — fix(fe): stop wrong-answer flicker and locale white flash; Uzbek admin nav

---

## Remaining external blockers (out of scope)

- **U-02** real VPS / registry credentials  
- **U-03** Payme/Click production merchant keys + legal entity  
- **U-12** real LLM API key  
- Full offline exam sync, Metabase/Grafana, TG daily quiz product invent, B2B self-serve seat checkout  

---

## Notes

- Browser may keep an old PWA SW; after FE deploy, unregister SW once if admin labels look stale.
- Admin technical terms (B2B, API, DB, dry-run) kept where they are product identifiers; UI chrome is Uzbek on `uz-Latn`.
