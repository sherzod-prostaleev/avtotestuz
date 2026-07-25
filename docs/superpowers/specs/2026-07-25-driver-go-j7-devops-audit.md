# Driver Go — DevOps Audit (Asphalt & Signal · J0–J9 wave)

**Date:** 2026-07-25  
**Auditor:** Cursor agent (report-only; no commit; no seed/wipe)  
**Repo:** `/home/sher/Рабочий стол/avtotest` · branch `main` @ `e119108`  
**Scope:** Committed wave (`3c5be1f`, `e119108`) + uncommitted J7 working-tree churn  
**Sources of truth:** `2026-07-25-driver-go-design-system.md` (+ v2), `next-wave-plan.md`, `visual-qa-checklist.md`

---

## 0. Executive verdict

| Gate | Result |
|------|--------|
| Typecheck | **PASS** |
| Lint | **PASS** (7× `@next/next/no-img-element` warnings) |
| Vitest | **FAIL** — 1 test / 1 file (`demo-question-block`) |
| Go (demo/auth/progress/config) | **PASS** |
| Design banned colors | **PASS** (no indigo/violet/purple/fuchsia/shadow-glass in code) |
| Official exam desktop lock | **PASS** (`max-lg:` only in `e119108`) |
| Runtime (:3000 / :8090 / docker) | **UP** at audit time |
| Seed / wipe | **Not run** this session |
| Critical production blockers in committed chrome | **None found** |

**Bottom line:** Committed J0–J6/J6b/J9 chrome is largely sound. Uncommitted J7 WIP breaks one Vitest and leaves demo progress **note UI incomplete** while storage write is wired. Visual QA matrix is still unchecked. Biggest product gap after J7 is **N2 demo→account migrate API** (docs), then Arena infra.

---

## 1. Git / change surface

### 1.1 Status (audit time)

- Branch: `main` tracking `origin/main` @ `e119108`
- **Uncommitted (14 files, +144 / −58):** J7 hot zone — layout, practice, tickets, login, landing/demo, session, `globals.css`, header, sidebar, answer-option (+test), question-stage, theme-toggle
- **Untracked:** `docs/superpowers/specs/2026-07-25-driver-go-next-wave-plan.md`, `.worktrees/` (m4-06 worktree dir)
- **No commit / push performed by this audit**

### 1.2 Committed wave surface

| Commit | Summary | Risk |
|--------|---------|------|
| `3c5be1f` | J0–J5: tokens, landing, shell, auth, dashboard; design-system.md | Large FE marketing/chrome rewrite |
| `e119108` | J6/J6b/J9: remaining chrome, mobile shells, session UX, demo-progress lib, exam `max-lg:` only, visual-qa checklist + v2 | Wide FE; content DB untouched (commit message asserts) |

### 1.3 Risk areas

| Area | Why |
|------|-----|
| **Uncommitted J7 vs sibling** | Same files sibling agent owns — merge conflict / half-applied UX likely |
| `demo-question-block.tsx` | WT records localStorage + unused `triedCount` / `BookmarkCheck`; **does not render** `demoProgressNote` / count (test + QA checklist expect them) |
| `answer-option` + `globals.css` shake | Intentional flicker fix; reduced-motion covered globally + class |
| `official-avtotest-exam-view.tsx` | Desktop locked; only mobile overrides — OK if no further desktop restyle |
| `.worktrees/` | Untracked; keep out of accidental `git add .` |

---

## 2. Frontend quality

### 2.1 Commands

```bash
cd frontend && npm run typecheck   # PASS
cd frontend && npm run lint        # PASS (warnings only)
cd frontend && npm run test        # FAIL 1/290
```

### 2.2 Typecheck

**PASS** (`tsc --noEmit`).

### 2.3 Lint

**PASS** with warnings — `@next/next/no-img-element` on:

- `(auth)/login/page.tsx`, `verify/page.tsx`
- `(public)/page.tsx`
- `official-avtotest-exam-view.tsx`
- `header.tsx`, `sidebar.tsx` (×2)

**Severity:** Low (pre-existing pattern for logo / exam; intentional unoptimized media in places).

### 2.4 Vitest

| Metric | Value |
|--------|-------|
| Files | 55 passed / **1 failed** / 56 |
| Tests | 289 passed / **1 failed** / 290 |

**Failure**

- File: `frontend/src/app/[locale]/(public)/demo-question-block.test.tsx`
- Assertion: `getByText("Progressing saqlanadi")` (and next: count string)
- **Root cause:** Working-tree `demo-question-block.tsx` calls `recordDemoAnswer` / sets `triedCount` but **never renders** `t("demoProgressNote")` or `t("demoProgressCount", …)`. Import `BookmarkCheck` is unused. Test + i18n keys assume the note is visible after grade.
- **Classification:** Open in J7 WIP (sibling hot zone). Not introduced by this audit agent. Prefer sibling fix over competing UI rewrite.
- **Note:** `uz-Latn` string `"Progressing saqlanadi"` is a typo vs Cyrl/RU “Progress…” / «Прогресс…» — Low copy bug once UI is restored.

---

## 3. Backend quality

`go` not on default `PATH`; used `/home/sher/.local/go/bin/go`.

```bash
cd backend && go test ./internal/demo/... ./internal/auth/... ./internal/progress/... ./internal/config/... -count=1 -timeout 90s
```

| Package | Result |
|---------|--------|
| `internal/demo` | ok (~9.7s) |
| `internal/auth` | ok (~7.8s) |
| `internal/progress` | ok (~6.8s) |
| `internal/config` | ok |

**Skipped:** full `go test ./...`, `make seed` / `seed-real` / truncate (per constraints).

---

## 4. Security / secrets

| Check | Result |
|-------|--------|
| `.env` committed | **No** — ignored; only `backend/.env.example`, `frontend/.env.local.example` tracked |
| Secrets in wave commits | No `.env` / credentials adds found in `git log --diff-filter=A` scan |
| OTP sandbox gate | `config.validate`: `OTP_CHANNEL=sandbox` **rejected** when `ENV=staging|prod` (`config.go`, tests cover) |
| OTP logging | `SandboxSender` logs `phone` + `code` at **Info** — acceptable **only** behind sandbox+dev; still noisy if misconfigured — Medium if ops ever set sandbox in shared logs |
| `debug_code` in API | Set only when `Env=="dev"` && channel sandbox (`auth/service.go`) |
| Referral localStorage | `referral-storage.ts`: clear on permanent API errors; keep on transient — tested |
| Demo migrate safety | Incorrect → `POST me/saved`; clear on success or 400/404; keep on network/5xx. Correct answers cleared with no server home — **product gap (N2)**, not secret leak |

---

## 5. Design-system compliance

| Rule | Status |
|------|--------|
| Ban indigo/violet/purple/fuchsia/shadow-glass | **PASS** — only comment mentions in `globals.css` |
| Glow logos / multi-glass | No `shadow-glass`. Streak `animate-flame` uses drop-shadow (quota: flame only — OK). Exam logo glow is **inside locked official exam chrome** (intentional authentic UI) |
| Solid amber CTAs + `text-accent-foreground` | Primary `Button` `game` + demo register link use accent-foreground. Solid `bg-accent` without accent-fg are progress bars (OK). Chips use `bg-accent/10` + `text-accent` (OK) |
| Gold CTA quota | `variant="gold"` remaining on checkout success/failure — VIP-adjacent OK; demo register moved off gold in WT (good) |
| Official exam not redesigned for desktop | **PASS** — `e119108` comment + `max-lg:` only |

---

## 6. Usability invariants (spot-check)

| Invariant | Evidence |
|-----------|----------|
| `.page-shell*` | Dashboard, practice, tickets, signs, mistakes, saved, premium, profile, stats, leaderboard, session start/result |
| `.sticky-cta-bar` | Practice, tickets, mistakes, saved, profile, session start/result; v2 matrix mostly covered |
| Touch ≥44 | `min-h-11` / `touch-target` / answer `min-h-14`; theme-toggle `h-11 w-11`; sidebar/header targets |
| `prefers-reduced-motion` | Global `*` duration kill in `@layer base` + explicit disable for shake/float/hero |
| Safe-area | `page-shell*` bottom, sticky CTA, app layout, sidebar drawer, session `100dvh` padding |

**Gaps (Medium / J7):** Visual QA 18-cell matrix still empty. Demo “progress saqlanadi” note absent in WT (checklist item). Premium page has page-shell but no sticky-cta (v2 table said buy CTA — verify manually).

---

## 7. Runtime ops

Checked with host network (sandbox curl falsely “down”):

| Service | Status |
|---------|--------|
| `http://127.0.0.1:8090/healthz` | `{"data":{"status":"ok"}}` |
| `http://127.0.0.1:3000/` | **307** (locale redirect — expected) |
| Docker `avtotest-postgres-1` | Up ~1h, healthy `:5432` |
| `avtotest-redis-1` | Up `:6379` |
| `avtotest-minio-1` | Up `:9000-9001` |
| Processes | `api` pid on `:8090`, `next-server` on `:3000` |

**Note:** Flaky from sandboxed tools; host view stable. **No restarts** performed.

---

## 8. Data safety

- This audit ran **no** `make seed`, `seed-real`, truncate, or DB wipe.
- Wave commit message (`e119108`): content seed data untouched.
- Content integrity: not re-validated against DB counts here; no destructive ops observed in session.

---

## 9. Findings by severity

### Still open

| Sev | ID | Finding | Path / evidence | Recommended fix |
|-----|----|---------|-----------------|-----------------|
| **High** | H1 | Vitest fail: demo progress note/count not rendered after grade | `demo-question-block.tsx` WT; test lines 76–77 | Sibling J7: render `demoProgressNote` + count (use `triedCount` / `BookmarkCheck`); keep localStorage write |
| **High** | H2 | Demo→account investment incomplete (correct lost; incorrect→bookmarks only; no migrate API) | `demo-progress-storage.ts`; next-wave §3 | Implement **N2** after J7 sign-off — do not inflate Grand Mock |
| **Medium** | M1 | Visual QA matrix unsigned (18 cells) | `visual-qa-checklist.md` | Complete J7 sign-off |
| **Medium** | M2 | Large uncommitted J7 surface on hot files | 14 FE files | Finish/merge J7 before parallel chrome; avoid second rewriter |
| **Medium** | M3 | Sandbox OTP logs raw code+phone at Info | `backend/internal/auth/sender.go` | Keep ENV gate; consider Debug level / redaction for shared log sinks |
| **Low** | L1 | `uz-Latn` typo `Progressing saqlanadi` | `frontend/messages/uz-Latn.json` | → `Progress saqlanadi` (align Cyrl/RU) |
| **Low** | L2 | Unused `BookmarkCheck` / dead display state until note UI lands | `demo-question-block.tsx` | Wire or remove with H1 |
| **Low** | L3 | Lint `no-img-element` ×7 | login/header/sidebar/landing/exam | Accept or migrate logos to `next/image` where static |
| **Low** | L4 | Footer contact looks placeholder-ish | messages `+998 71 200 00 00`, address | Replace when real contacts exist (N4) |
| **Info** | I1 | `.worktrees/` untracked | `.worktrees/m4-06-telegram-bot` | Don’t `git add .` blindly |
| **Info** | I2 | Roadmap table J7 marked ✅ partial in design-system.md while checklist empty | design-system §J | Refresh statuses on wave close |
| **Info** | I3 | `go` not on default PATH | `/home/sher/.local/go/bin/go` | Document for CI/dev shells |

### Already fixed in working tree (J7 WIP — verify after merge)

| Item | Evidence |
|------|----------|
| Answer wrong-shake paint isolation + no disabled opacity flicker | `answer-option.tsx` + test; `globals.css` translate3d |
| Demo register CTA: accent + `text-accent-foreground`, no nested Button-in-Link | `demo-question-block.tsx` diff |
| Demo answers written to `drivergo:demo-progress` on grade | `recordDemoAnswer` in WT (note UI still missing → H1) |
| Reduced-motion global + shake opt-out | `globals.css` |

### Committed & acceptable

| Item | Notes |
|------|-------|
| Official exam desktop lock | `max-lg:` mobile-only in `e119108` |
| OTP sandbox forbidden outside `ENV=dev` | config validation + tests |
| Referral / demo storage tests | Present and passing |
| Indigo purge | Grep clean on `frontend/src` |

---

## 10. Remaining big work backlog (for parent → user)

Ranked from docs (not invented):

1. **N1 — J7 close-out** — finish flicker/demo note UI, fix Vitest H1, sign visual-qa 18-cell matrix. *(in progress · sibling)*
2. **N2 — Demo investment continuity** — `POST /me/demo-progress/migrate` (prefer mistakes over bookmarks); client clear-on-ack; no Grand Mock inflation without product OK.
3. **N5 — M4-03 Arena infra** — expand TDD plan → T1–T4; migration `0021_battle_arena`; no UI; Redis `arena:`.
4. **N6 → N7 / J10** — rating/medals API then Arena UI on Asphalt tokens (after wire protocol stable).
5. **N3/N4 optional** — J8 Figma SoT; tech debt (footer contacts, leftover a11y, handoff J-table drift). Referral antifraud remains design-only.

**Explicit later / out of scope now:** Official exam desktop redesign; content reseed; Arena UI before M4-03.

---

## 11. Audit agent actions

| Action | Done? |
|--------|-------|
| Create this report | Yes |
| Git commit / push | **No** |
| Mass UI rewrite in J7 hot zone | **No** |
| Seed / wipe DB | **No** |
| Code or test fixes | **None** (report-only; H1 owned by J7 sibling) |

---

*End of audit. Re-run Vitest + visual QA after J7 merges before claiming chrome wave closed.*

---

## 12. E0 gate close-out (2026-07-26)

| Gate | Result |
|------|--------|
| `npm run typecheck` | **PASS** |
| `npm run lint` | **PASS** (img warnings only) |
| `npm run test` (vitest) | **PASS** 291/291 |
| `go test` demo/auth/progress/config | **PASS** |
| Demo progress note UI + typo | **Fixed** (`Progress saqlanadi`; count rendered) |
| Visual QA matrix | **18/18 OK** (spot) |
| Seed/wipe | **None** |
| J7 status | **CLOSED** → next-wave N2 |
