# Driver Go — Next-Wave Plan (post-J7)
**2026-07-25 · Asphalt & Signal**

> Status lock: **J0–J6, J6b, J7, J9 ✅**. **J8** optional (later). **J10** after Arena infra.
> Official exam desktop UI locked. Never wipe/seed questions/signs/variants.
> Sources: `2026-07-25-driver-go-design-system.md`, `-v2.md`, `visual-qa-checklist.md`, M4-03 Arena docs.

---

## 1. Goal of this wave

After J7 sign-off: close **demo → account investment continuity**, optionally lock **Figma SoT (J8)**, then sequence **Arena backend (M4-03/04)** before **Arena UI on Asphalt tokens (J10 / M4-05)**. Prefer product continuity and infra seams over more chrome churn.

---

## 2. Prioritized backlog (after J7)

| # | Item | Priority | Est. order | Depends on |
|---|------|----------|------------|------------|
| **N1** | **J7 close-out** — visual QA matrix sign-off + residual chrome/flicker fixes | P0 | 0 (sibling) | — |
| **N2** | **Demo investment continuity** — localStorage → real account progress (beyond bookmarks) | P0 | 1 | J7 done (or parallel if BE-only) |
| **N3** | **J8 Figma SoT** (optional) — tokens + key chrome screens only | P2 | 2 | N1 preferred |
| **N4** | **Tech debt** from chrome wave (footer placeholders, leftover raw colors, a11y leftovers) | P2 | 2–3 | N1 |
| **N5** | **M4-03 Arena infra** — expand TDD plan → T1–T4 code (no UI) | P1 | 3 | Spec locked ✅ |
| **N6** | **M4-04** rating / medals / match history API | P1 | 4 | N5 |
| **N7** | **J10 / M4-05 Arena UI** — matchmaking, live duel, result, friend invite on Asphalt tokens | P1 | 5 | N5 (+ N6 for ratings chrome) |

North-star after chrome: **anon demo → signed-in habit loop unbroken** (Hooked investment), then **Arena retention loop** on the same visual system.

---

## 3. N2 — Demo investment continuity (inventory + gaps)

### 3.1 What already exists

| Piece | Path | Behavior |
|-------|------|----------|
| Guest store | `frontend/src/lib/demo-progress-storage.ts` | Key `drivergo:demo-progress` v1; upsert by `questionId` |
| Unit tests | `frontend/src/lib/demo-progress-storage.test.ts` | record/upsert/corrupt/migrate/retry/clear |
| Landing write | `frontend/src/app/[locale]/(public)/demo-question-block.tsx` | `recordDemoAnswer` after grade |
| Login migrate | `frontend/src/app/[locale]/(auth)/login/verify/page.tsx` | `await migrateDemoProgressOnLogin()` after OTP |
| Retry capture | `frontend/src/components/demo/demo-progress-capture.tsx` in `(app)/layout.tsx` | Re-run migrate on pathname change |
| Backend demo | `backend/internal/demo/` | `GET /demo/question`, `POST /demo/answer` only — **no migrate ingest** |
| Saved API | `backend/internal/progress` `POST /me/saved` | Current migration target for **incorrect** only |

### 3.2 Gaps (product)

1. **No dedicated migrate API** — comments in `demo-progress-storage.ts` state this explicitly; correct answers have no server home.
2. **Incorrect → bookmarks, not mistakes/FSRS** — investment is weak: Saved list ≠ mistake bank / `learning.Again`.
3. **Correct-only progress is wiped** on successful migrate with zero POSTs — user loses “I already tried” signal.
4. **Dashboard / readiness / streak** do not reflect demo answers after login.
5. **No idempotent server ack** beyond “clear localStorage” — partial multi-question failure mid-loop can leave inconsistent bookmark set (sequential POSTs).
6. **Cross-device** — localStorage-only; second device never sees guest demo.

### 3.3 Proposed direction (do not implement in J7 collision window)

**Preferred (small, honest):**

1. Spec `POST /me/demo-progress/migrate` (or extend progress package): body = `{ answers: [{ question_id, answer_id, correct, answered_at }] }`.
2. Server rules (pure + tested):
   - Auth required; rate-limit; validate UUIDs belong to real Q/A pairs.
   - Incorrect → upsert mistakes / FSRS `Again` (align with product: saved **or** mistakes — pick one in implement plan; prefer **mistakes** for Hooked investment).
   - Correct → optional lightweight “seen” / do **not** inflate `CountStudiedQuestions` / Grand Mock gate without explicit product OK (Arena/Grand Mock gate is sensitive).
   - Idempotent: same question twice → last-write or first-write wins; return `{ migrated, skipped }`.
3. Client: call migrate API; clear local only on `2xx` with full ack; keep `DemoProgressCapture` retry.
4. UX copy: keep “progress saqlanadi” honest — after migrate, surface “X ta demo javob hisobga olindi” once on dashboard (chrome toast/banner — **after** J7).

**Out of scope for N2:** redesigning landing demo UI; changing official exam; wiping content DBs.

### 3.4 Acceptance criteria (N2)

- [ ] Guest answers survive reload (already true) and survive OTP → dashboard without silent total loss of incorrect items.
- [ ] Incorrect demo Qs appear in **mistakes** (or documented Saved decision) after login; retry-safe under network blip.
- [ ] Correct-only guest progress does not claim mastery/readiness inflation unless product explicitly allows it.
- [ ] Vitest + Go handler tests for migrate idempotency and bad IDs.
- [ ] Visual QA checklist item for demo note still passes (no flicker regression).

---

## 4. N3 — J8 Figma (optional scope)

**Do only if** design handoff or multi-dev sync needs a shared file. Not a blocker for N2 or Arena infra.

| In scope | Out of scope |
|----------|--------------|
| Variables = CSS tokens (C.1 / v2 HEX table) | Full Arena screens (wait for M4-05) |
| Components from master §F (Button, Input, OTP, Nav, Stat tile, Next-action) | Session play redesign (J9 already done) |
| Screens: Landing hero, Login, Dashboard, Practice start, one shell+More | Marketing illustrations / new brand exploration |
| Light + dark frames; mobile + desktop | Automating all 18 QA cells in CI |

**Acceptance:** Figma variables named to match CSS; link from design-system.md appendix; no mid-Arena token rename after publish.

---

## 5. N5–N7 — Arena path → J10 UI

### 5.1 Inventory (code vs docs)

| Artifact | Status |
|----------|--------|
| Design spec | ✅ `docs/superpowers/specs/2026-07-25-m4-03-battle-arena-design.md` |
| Impl plan | 🟡 skeleton `docs/superpowers/plans/2026-07-25-m4-03-battle-arena.md` — expand to full TDD before coding |
| Roadmap rows | M4-03 infra → M4-04 rating → M4-05 UI (`2026-07-24-roadmap-m2-to-admin.md`) |
| `backend/internal/arena` | ❌ **does not exist** |
| FE Arena routes/components | ❌ **none** |
| Migration | Next = **`0021_battle_arena`** (Telegram took `0020`; handoff confirms) |
| Redis prefix | `arena:` (never `lb:`) |
| Wire protocol | Spec §2.4 — M4-05 / J10 consumes this |

### 5.2 J10 UI constraints (when its turn comes)

- Build **only** on Asphalt & Signal tokens (`bg-accent`, `text-accent-foreground`, success/danger).
- Screens: queue / searching, live 1v1 (10×15s), reveal, result, friend-invite redeem — VIP gate `402` → Premium upsell chrome.
- Usability First (v2): ≤2 taps from nav to “Find match”; touch ≥44; reduced-motion respected.
- **Do not** couple Arena scores to M4-01 leaderboard (spec locked).
- **Do not** redesign official exam desktop to “match” Arena.
- Bot opponent = overt practice bot only if product opens it (Q10 deferred to M4-05).

### 5.3 Acceptance criteria (J10 — later)

- [ ] Matchmaking + live duel + result playable against real M4-03 WS.
- [ ] VIP-only entry; free users see locked/upsell, never enter queue.
- [ ] Tokens only — no indigo/glow; light/dark + mobile/desktop spot-check.
- [ ] Friend invite redeem uses T2 transport; no second matchmaking rewrite.

---

## 6. N4 — Tech debt (chrome leftovers)

Triage **after** J7 sign-off (sibling owns checklist/chrome now):

- Landing footer contact placeholders (`SESSION-HANDOFF` — real phone/TG/IG when available).
- Any remaining non-semantic color classes found in J7 QA notes.
- Sticky-CTA / 44px gaps filed during visual QA.
- Handoff doc J-table drift (still shows J6–J9 ⬜ in places) — refresh when wave closes.
- Referral anti-fraud still design-only (roadmap leftover; not design-system).

---

## 7. Estimated order (single timeline)

```
[now]  Sibling: J7 motion/a11y/flicker + visual-qa-checklist.md
   │
   ├─ parallel SAFE: this plan + inventories only (no chrome mass-edits)
   │
   ▼
N1  J7 sign-off (18-cell matrix as checklist, not full CI)
   │
   ▼
N2  Demo migrate API + client (investment continuity)     ← highest post-chrome product ROI
   │
   ├─ N3 J8 Figma (optional, can slip)
   └─ N4 tech debt triage (from QA notes)
   │
   ▼
N5  M4-03: expand plan → T1 transport → T2 matchmaking → T3 match+DB → T4 harden
   │
   ▼
N6  M4-04 rating/medals/history
   │
   ▼
N7  J10 / M4-05 Arena UI on Asphalt tokens
```

Parallelism rule: FE chrome ↔ Arena BE can parallelize **after** tokens locked (already). Demo migrate BE can start once J7 is not thrashing auth/layout — prefer waiting for J7 merge on `(auth)/login` and `(app)/layout` if those stay hot.

---

## 8. Risk / conflicts with J7 agent

| Hot zone (J7 sibling) | This wave must NOT touch until merge |
|------------------------|--------------------------------------|
| Landing / demo-question-block chrome | Avoid UI edits; inventory only |
| Sidebar / theme-toggle / app layout | Avoid — `DemoProgressCapture` mount lives here |
| Practice / tickets / session flicker | Avoid |
| `visual-qa-checklist.md` | Sibling-owned during J7 |
| globals / motion / reduced-motion | Avoid |

**Safe now:** docs under `docs/superpowers/specs/`; isolated lib notes; expanding M4-03 **plan** text (not code) if needed.

**Conflict if premature:** implementing migrate UI copy on landing; editing verify page migrate call site while flicker work lands; Arena FE stubs in sidebar (“Arena” nav) before M4-03.

---

## 9. What NOT to do

- Do **not** mass-edit landing / sidebar / session / practice / tickets / globals while J7 runs.
- Do **not** start Arena UI (J10) before M4-03 wire protocol is implemented and stable.
- Do **not** redesign official exam desktop UI.
- Do **not** wipe or reseed questions / signs / variants.
- Do **not** write Arena scores into leaderboard tables.
- Do **not** claim demo correct answers toward Grand Mock unlock without an explicit product decision + test.
- Do **not** treat J8 Figma as a gate for N2 or M4-03.
- Do **not** git-commit from prep-only sessions unless explicitly asked.

---

## 10. Prep done this session (safe)

- [x] Next-wave plan written (this file).
- [x] Demo-progress migration inventory + gaps documented (§3).
- [x] Arena docs/code path inventory for J10 sequencing (§5).
- [ ] Code changes deferred (no competing UI; no Arena package yet).

---

## 11. First commands for the next implementing agent

**After J7 merges — Demo continuity (N2):**
```text
Implement demo → account investment continuity per
docs/superpowers/specs/2026-07-25-driver-go-next-wave-plan.md §3.
Add POST migrate (progress or demo package), wire client clear-on-ack,
prefer mistakes over bookmarks for incorrect answers, tests required.
Do not inflate Grand Mock studied count without explicit OK.
Do not touch official exam interior.
```

**When ready for Arena infra (N5):**
```text
Expand docs/superpowers/plans/2026-07-25-m4-03-battle-arena.md to full TDD plan,
then implement T1 per spec. Migration number = 0021. No UI. No leaderboard coupling.
```

---

*Status: actionable backlog for post-J7. Chrome QA remains with J7 agent until sign-off.*
