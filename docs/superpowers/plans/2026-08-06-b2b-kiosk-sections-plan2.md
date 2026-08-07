# B2B Kiosk Sections — Plan 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A classroom PC's kiosk offers the seven sections a driving school actually needs — practice, tickets, exam, road signs, stats, saved questions and a read-only leaderboard — with no login and no route that bounces a student to a login form.

**Architecture:** Each section reuses the learner app's existing page component through a `kiosk` prop rather than duplicating it, mounted at its own `/station/...` URL inside the `(kiosk)` route group so the middleware exemption stays narrow and the pages never inherit the authenticated app shell. The first task widens the existing route-registration test into one that also follows each kiosk wrapper to the page it reuses and checks that page's own navigation targets — the gap that let four separate navigation breaks ship during Plan 1.

**Tech Stack:** Next.js 15 (App Router, TypeScript, next-intl), Vitest, Testing Library.

**Spec:** `docs/superpowers/specs/2026-08-06-b2b-admin-installer-design.md`, section 6A.

## Global Constraints

- Plan 2 only. Nothing in the installer, agent or admin surface changes — that was Plan 1.
- **Do not** add Arena, "mistakes" (`xatolar ustida ishlash`) or grand mock to the kiosk. Each is excluded for a recorded reason in spec section 6A.
- **Do not** add Profile, Premium, Checkout or Dashboard to the kiosk, and do not let any of them become reachable from a kiosk page. An earlier review had to close exactly that hole.
- **Do not** add `practice`, `tickets`, `session`, `signs`, `stats`, `saved` or `leaderboard` to the public list in `frontend/src/lib/protected-segments.ts`. That would unauthenticate them for every learner on the platform. The kiosk gets its own `/station/...` URLs instead.
- Offline tolerance is not being built, now or later.
- Every user-facing string must exist in all three locale files: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json` — real translations, not Latin text pasted into the Cyrillic and Russian ones.
- Frontend gate: `make fe-check`. Never commit a failing gate.
- Run every command in the foreground. Do NOT launch background jobs, monitors, or `&`-detached processes.
- Commit after every task.

## The pattern every section task follows

Plan 1 established it; read these three files before starting any section task:

- `frontend/src/app/[locale]/(kiosk)/station/practice/page.tsx` — the wrapper: a doc comment explaining why the URL is distinct, one import, one line rendering the reused page with `kiosk`.
- `frontend/src/app/[locale]/(app)/practice/page.tsx` — how a reused page takes `kiosk?: boolean`, derives `backHref` and `sessionStartBase` from it, and drops surfaces that make no sense on a shared classroom PC.
- `frontend/src/app/[locale]/(kiosk)/kiosk-path.test.tsx` — the guard.

**The accepted limitation, stated once so no task re-litigates it:** the station has one shadow profile, so stats and saved questions belong to *that PC*, not to a student. This is known and accepted; do not add per-student identity to work around it.

### The per-page "kiosk mode" assertion

Tasks 3-6 each add one of these to the reused page's own test file. It is the half the static path scan cannot do — it catches a target built at runtime, which no filesystem scan can see. Adapt the targets to the page you are working on; the shape stays the same:

```tsx
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";

describe("kiosk mode", () => {
  it("keeps every navigation target inside the kiosk", async () => {
    render(<SignsPage kiosk />);

    // Every link the page renders, not a hand-picked subset: a target added
    // later without a matching assertion is exactly how a student ends up
    // on the login form.
    const links = await screen.findAllByRole("link");
    for (const link of links) {
      const href = link.getAttribute("href") ?? "";
      const p = href.replace(/^\/(uz-Latn|uz-Cyrl|ru)(?=\/|$)/, "").split("?")[0];
      expect(matchesAny(p || "/", PROTECTED_SEGMENTS)).toBe(false);
    }
  });

  it("offers no account or upsell surface", async () => {
    render(<SignsPage kiosk />);
    await screen.findAllByRole("link");

    for (const dead of ["dashboard", "premium", "checkout", "profile"]) {
      expect(
        screen.queryByRole("link", { name: new RegExp(dead, "i") }),
      ).not.toBeInTheDocument();
    }
  });
});
```

`findAllByRole("link")` rather than a list of expected hrefs is deliberate: it holds for targets nobody thought to enumerate.

**Note on the section tasks' Step 1 and Step 2.** Each says "list every navigation target" and "assert every listed target" rather than giving literal code for that page. That is because the four pages total 972 lines and their navigation differs; pre-listing it here would be guessing. The block above is the literal shape to write, and the widened path test from Task 1 is what catches a literal target the list misses. A case list is not permission to write fewer tests.

---

### Task 1: Make the path test follow wrappers into the pages they reuse

Do this first. Every later task depends on it to prove nothing was missed.

**Files:**
- Modify: `frontend/src/app/[locale]/(kiosk)/kiosk-path.test.tsx`

**Interfaces:**
- Consumes: `PROTECTED_SEGMENTS`, `matchesAny` from `@/lib/protected-segments`.
- Produces: a test that fails when any kiosk page, or any `(app)` page a kiosk wrapper reuses, contains a literal navigation target under a cookie-gated segment.

- [ ] **Step 1: Understand what the current test does and does not catch**

Read the file. It walks the filesystem for `page.tsx` under `(kiosk)` and asserts each *route's own path* is outside `PROTECTED_SEGMENTS`, plus a boundary check for `not-found.tsx` / `error.tsx`.

That would have caught **none** of the four navigation breaks found during Plan 1 — `/station` linking to `/practice`, practice pushing to `/session/start`, session-start replacing to `/session/[id]`, and the session screen exiting to `/dashboard`. In every case the page's own path was fine; the target it navigated to was not.

- [ ] **Step 2: Write the failing test**

Add to `kiosk-path.test.tsx`. It resolves each kiosk wrapper's re-exported page and scans both files for literal navigation targets:

```tsx
/**
 * A kiosk wrapper is three lines; the navigation that can strand a student
 * lives in the `(app)` page it reuses. This resolves that import so the
 * scan below covers both files.
 *
 * Only literal paths are checked. A target built at runtime — from an API
 * response, or a variable this scan cannot see — is invisible here, and no
 * static check can fix that. Those are covered by the per-page "kiosk mode"
 * assertions in each reused page's own test file.
 */
function reusedPageFiles(kioskPageFile: string): string[] {
  const src = fs.readFileSync(kioskPageFile, "utf8");
  const out: string[] = [];
  for (const m of src.matchAll(/from\s+"@\/app\/([^"]+)"/g)) {
    const candidate = path.join(kioskDir, "..", "..", "..", "app", m[1] + ".tsx");
    if (fs.existsSync(candidate)) out.push(candidate);
  }
  return out;
}

/** Literal navigation targets: href="/..." , router.push("/...") , router.replace("/...") */
function navigationTargets(file: string): string[] {
  const src = fs.readFileSync(file, "utf8");
  const out: string[] = [];
  for (const m of src.matchAll(/(?:href=|router\.(?:push|replace)\()\s*[`"']([^`"']*)[`"']/g)) {
    out.push(m[1]);
  }
  for (const m of src.matchAll(/(?:href=|router\.(?:push|replace)\()\s*\{?\s*`([^`]*)`/g)) {
    out.push(m[1]);
  }
  return out;
}

/**
 * `/${locale}/station/practice` and `/uz-Latn/practice` both need to reduce
 * to the path the middleware sees. Strips a leading template locale
 * placeholder or a literal locale segment, and any query string.
 */
function toMiddlewarePath(target: string): string | null {
  if (!target.startsWith("/")) return null;
  const noQuery = target.split("?")[0];
  const stripped = noQuery
    .replace(/^\/\$\{locale\}/, "")
    .replace(/^\/(uz-Latn|uz-Cyrl|ru)(?=\/|$)/, "");
  return stripped === "" ? "/" : stripped;
}

describe("kiosk navigation targets", () => {
  it("never navigates to a cookie-gated route", () => {
    const pageFiles = findPageFiles(kioskDir);
    expect(pageFiles.length).toBeGreaterThan(0);

    const offenders: string[] = [];
    for (const pageFile of pageFiles) {
      const files = [pageFile, ...reusedPageFiles(pageFile)];
      for (const file of files) {
        for (const target of navigationTargets(file)) {
          const p = toMiddlewarePath(target);
          if (p && matchesAny(p, PROTECTED_SEGMENTS)) {
            offenders.push(`${path.relative(kioskDir, file)} -> ${target}`);
          }
        }
      }
    }
    expect(offenders).toEqual([]);
  });
});
```

`findPageRoutes` returns URL paths, not file paths, and the existing route test depends on that. Add this alongside it rather than changing it:

```tsx
/** Absolute paths to every page.tsx under (kiosk). */
function findPageFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const entryPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...findPageFiles(entryPath));
    } else if (entry.name === "page.tsx") {
      files.push(entryPath);
    }
  }
  return files;
}
```

- [ ] **Step 3: Run it**

Run: `cd frontend && npm run test -- kiosk-path`
Expected: PASS. Plan 1 already fixed every navigation this scan can see, so a green result here is correct — it means the guard agrees with the fixes rather than that it does nothing. Step 4 proves it is not vacuous.

- [ ] **Step 4: Prove the guard has teeth**

Temporarily add a link to a gated route inside the reused practice page — for example change the kiosk `backHref` in `frontend/src/app/[locale]/(app)/practice/page.tsx` from `/${locale}/station` to `/${locale}/dashboard`. Run the test: it must FAIL naming that target. Restore the file and confirm it passes again.

Record both results in your report. A guard that passes but cannot fail is worse than none, because it licenses everything after it.

- [ ] **Step 5: Commit**

```bash
git add "frontend/src/app/[locale]/(kiosk)/kiosk-path.test.tsx"
git commit -m "test(kiosk): follow wrappers into the pages they reuse

The route-registration check would have caught none of the four navigation
breaks fixed during Plan 1 -- in every one the page's own path was fine and
the target it pushed to was gated. This scans literal targets in both the
wrapper and the page it re-exports."
```

---

### Task 2: Exam on the kiosk

The route already exists; this is the entry point to it.

**Files:**
- Modify: `frontend/src/app/[locale]/(kiosk)/station/page.tsx`
- Modify: `frontend/src/app/[locale]/(kiosk)/station/page.test.tsx`
- Modify: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`

**Interfaces:**
- Consumes: `(kiosk)/station/session/start/page.tsx`, which reads a `mode` query parameter (built in Plan 1).
- Produces: a `Station.exam` message key and an exam card on the kiosk home.

- [ ] **Step 1: Write the failing test**

Add to `frontend/src/app/[locale]/(kiosk)/station/page.test.tsx`, following the mocking already in that file:

```tsx
it("offers the exam simulation", async () => {
  apiGet.mockResolvedValue({ kind: "station", name: "PC-1" });
  render(<StationPage />);

  const link = await screen.findByRole("link", { name: "exam" });
  expect(link).toHaveAttribute("href", "/uz-Latn/station/session/start?mode=exam");
});
```

The file's existing tests mock `useTranslations` to return the key, which is why the accessible name is `"exam"`. Match whatever that file already does rather than introducing a different mock.

- [ ] **Step 2: Run to verify it fails**

Run: `cd frontend && npm run test -- "station/page"`
Expected: FAIL — no link named `exam`.

- [ ] **Step 3: Add the card**

In `frontend/src/app/[locale]/(kiosk)/station/page.tsx`, add a third card beside practice and tickets:

```tsx
<Link
  href={`/${locale}/station/session/start?mode=exam`}
  className="rounded-lg border p-6 text-center text-xl"
>
  {t("exam")}
</Link>
```

Real exam mode is `mode=exam`. Do **not** use `mode=grand_mock` — that is the certificate-issuing final exam, excluded from the kiosk because its unlock thresholds are met by thirty students' combined study and its certificate would read "PC-1 passed".

Add `Station.exam` to all three locale files: `"Imtihon"` (uz-Latn), `"Имтиҳон"` (uz-Cyrl), `"Экзамен"` (ru).

- [ ] **Step 4: Run the tests**

```bash
cd frontend && npm run test -- "station/page"
cd frontend && npm run test -- kiosk-path
```

Expected: both PASS. The path test must stay green — the new target is under `/station/`, which is not gated.

- [ ] **Step 5: Commit**

```bash
git add "frontend/src/app/[locale]/(kiosk)/station" frontend/messages
git commit -m "feat(kiosk): add the exam simulation entry point

The exam is what a driving school classroom is for, and its only entry
point lived on the dashboard -- a page the kiosk deliberately excludes."
```

---

### Task 3: Road signs on the kiosk

**Files:**
- Create: `frontend/src/app/[locale]/(kiosk)/station/signs/page.tsx`
- Modify: `frontend/src/app/[locale]/(app)/signs/page.tsx`
- Modify: `frontend/src/app/[locale]/(app)/signs/page.test.tsx` (create if absent)
- Modify: `frontend/src/app/[locale]/(kiosk)/station/page.tsx`, `page.test.tsx`
- Modify: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`

**Interfaces:**
- Consumes: the `kiosk` prop pattern from `(app)/practice/page.tsx`.
- Produces: route `/[locale]/station/signs`; `SignsPage({ kiosk }: { kiosk?: boolean })`; message key `Station.signs`.

- [ ] **Step 1: Find every navigation in the page**

Read `frontend/src/app/[locale]/(app)/signs/page.tsx` (291 lines) and list every `href=`, `router.push` and `router.replace` it contains, plus any it renders through a shared component. Note it already starts a practice session with `mode: "practice"` — that push target must become the kiosk one.

Write that list into your report before changing anything. The widened path test from Task 1 will catch a literal target you miss, but it cannot catch one built at runtime, so the list is what makes the runtime ones visible.

- [ ] **Step 2: Write the failing tests**

In the signs page's own test file, add a `kiosk mode` block asserting that with `kiosk` set, every navigation target you listed points under `/station/`. Assert the targets, not just that the page renders. If the file does not exist, create it modelled on `frontend/src/app/[locale]/(kiosk)/station/page.test.tsx`.

Add to the kiosk home test:

```tsx
it("offers road signs", async () => {
  apiGet.mockResolvedValue({ kind: "station", name: "PC-1" });
  render(<StationPage />);

  const link = await screen.findByRole("link", { name: "signs" });
  expect(link).toHaveAttribute("href", "/uz-Latn/station/signs");
});
```

- [ ] **Step 3: Run to verify they fail**

Run: `cd frontend && npm run test -- signs`
Expected: FAIL — the prop and the route do not exist.

- [ ] **Step 4: Thread the prop and create the wrapper**

Give `(app)/signs/page.tsx` a `kiosk?: boolean` prop exactly as `(app)/practice/page.tsx` does — same prop shape, same doc comment style explaining what it suppresses and why. Derive every navigation target from it, including the practice-session push.

Create `frontend/src/app/[locale]/(kiosk)/station/signs/page.tsx`:

```tsx
// Kiosk road-signs entry point: /[locale]/station/signs.
//
// A distinct URL from the learner app's /[locale]/signs so the middleware
// exemption in src/proxy.ts stays narrow — only "station" and everything
// under it is login-free — instead of unauthenticating /signs for every
// learner on the platform.
import SignsPage from "@/app/[locale]/(app)/signs/page";

export default function KioskSignsPage() {
  return <SignsPage kiosk />;
}
```

Add the card to the kiosk home, and `Station.signs` to all three locales: `"Yo'l belgilari"` (uz-Latn), `"Йўл белгилари"` (uz-Cyrl), `"Дорожные знаки"` (ru).

- [ ] **Step 5: Run the tests**

```bash
cd frontend && npm run test -- signs
cd frontend && npm run test -- "station/page"
cd frontend && npm run test -- kiosk-path
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/[locale]" frontend/messages
git commit -m "feat(kiosk): add road signs

285 signs, pure reference content with no personal state -- the section
that most obviously belonged on a classroom PC from the start."
```

---

### Task 4: Stats on the kiosk

**Files:**
- Create: `frontend/src/app/[locale]/(kiosk)/station/stats/page.tsx`
- Modify: `frontend/src/app/[locale]/(app)/stats/page.tsx`
- Modify: `frontend/src/app/[locale]/(app)/stats/page.test.tsx` (create if absent)
- Modify: `frontend/src/app/[locale]/(kiosk)/station/page.tsx`, `page.test.tsx`
- Modify: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`

**Interfaces:**
- Consumes: the `kiosk` prop pattern.
- Produces: route `/[locale]/station/stats`; `StatsPage({ kiosk }: { kiosk?: boolean })`; message key `Station.stats`.

- [ ] **Step 1: Find every navigation in the page**

Read `frontend/src/app/[locale]/(app)/stats/page.tsx` (212 lines) and list every navigation target, including any that lead to premium, checkout or the dashboard — those must be suppressed under `kiosk`, not merely redirected.

Write the list into your report.

- [ ] **Step 2: Write the failing tests**

Add a `kiosk mode` block to the stats page's test file asserting every listed target points under `/station/`, and that no premium, checkout, profile or dashboard link renders when `kiosk` is set. Assert absence with `queryBy...` and `not.toBeInTheDocument()`, not by checking the happy path only.

Add to the kiosk home test:

```tsx
it("offers stats", async () => {
  apiGet.mockResolvedValue({ kind: "station", name: "PC-1" });
  render(<StationPage />);

  const link = await screen.findByRole("link", { name: "stats" });
  expect(link).toHaveAttribute("href", "/uz-Latn/station/stats");
});
```

- [ ] **Step 3: Run to verify they fail**

Run: `cd frontend && npm run test -- stats`
Expected: FAIL.

- [ ] **Step 4: Thread the prop and create the wrapper**

Give `(app)/stats/page.tsx` the `kiosk?: boolean` prop in the same shape as `(app)/practice/page.tsx`, suppressing every upsell and account surface under it.

Create `frontend/src/app/[locale]/(kiosk)/station/stats/page.tsx`:

```tsx
// Kiosk stats entry point: /[locale]/station/stats.
//
// A distinct URL from the learner app's /[locale]/stats so the middleware
// exemption in src/proxy.ts stays narrow — only "station" and everything
// under it is login-free.
//
// The numbers here belong to the PC, not to a student: a station has one
// shadow profile that every student who sits down shares. That is a known
// and accepted consequence of a login-free classroom, not a bug.
import StatsPage from "@/app/[locale]/(app)/stats/page";

export default function KioskStatsPage() {
  return <StatsPage kiosk />;
}
```

Add the card and `Station.stats` to all three locales: `"Statistika"` (uz-Latn), `"Статистика"` (uz-Cyrl), `"Статистика"` (ru).

- [ ] **Step 5: Run the tests**

```bash
cd frontend && npm run test -- stats
cd frontend && npm run test -- "station/page"
cd frontend && npm run test -- kiosk-path
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/[locale]" frontend/messages
git commit -m "feat(kiosk): add stats

Scoped to the PC rather than a student, which is what a shared shadow
profile can honestly report."
```

---

### Task 5: Saved questions on the kiosk

**Files:**
- Create: `frontend/src/app/[locale]/(kiosk)/station/saved/page.tsx`
- Modify: `frontend/src/app/[locale]/(app)/saved/page.tsx`
- Modify: `frontend/src/app/[locale]/(app)/saved/page.test.tsx` (create if absent)
- Modify: `frontend/src/app/[locale]/(kiosk)/station/page.tsx`, `page.test.tsx`
- Modify: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`

**Interfaces:**
- Consumes: the `kiosk` prop pattern.
- Produces: route `/[locale]/station/saved`; `SavedPage({ kiosk }: { kiosk?: boolean })`; message key `Station.saved`.

- [ ] **Step 1: Find every navigation in the page**

Read `frontend/src/app/[locale]/(app)/saved/page.tsx` (249 lines) and list every navigation target. Saved questions typically open a practice or review session — that push is the one most likely to strand a student.

Write the list into your report.

- [ ] **Step 2: Write the failing tests**

Add a `kiosk mode` block to the saved page's test file asserting every listed target points under `/station/`. Add to the kiosk home test:

```tsx
it("offers saved questions", async () => {
  apiGet.mockResolvedValue({ kind: "station", name: "PC-1" });
  render(<StationPage />);

  const link = await screen.findByRole("link", { name: "saved" });
  expect(link).toHaveAttribute("href", "/uz-Latn/station/saved");
});
```

- [ ] **Step 3: Run to verify they fail**

Run: `cd frontend && npm run test -- saved`
Expected: FAIL.

- [ ] **Step 4: Thread the prop and create the wrapper**

Give `(app)/saved/page.tsx` the `kiosk?: boolean` prop in the same shape as `(app)/practice/page.tsx`.

Create `frontend/src/app/[locale]/(kiosk)/station/saved/page.tsx`:

```tsx
// Kiosk saved-questions entry point: /[locale]/station/saved.
//
// A distinct URL from the learner app's /[locale]/saved so the middleware
// exemption in src/proxy.ts stays narrow — only "station" and everything
// under it is login-free.
//
// The list belongs to the PC, not to a student: a station has one shadow
// profile that every student who sits down shares. Known and accepted.
import SavedPage from "@/app/[locale]/(app)/saved/page";

export default function KioskSavedPage() {
  return <SavedPage kiosk />;
}
```

Add the card and `Station.saved` to all three locales: `"Saqlanganlar"` (uz-Latn), `"Сақланганлар"` (uz-Cyrl), `"Сохранённые"` (ru).

- [ ] **Step 5: Run the tests**

```bash
cd frontend && npm run test -- saved
cd frontend && npm run test -- "station/page"
cd frontend && npm run test -- kiosk-path
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/[locale]" frontend/messages
git commit -m "feat(kiosk): add saved questions"
```

---

### Task 6: Read-only leaderboard on the kiosk

**Files:**
- Create: `frontend/src/app/[locale]/(kiosk)/station/leaderboard/page.tsx`
- Modify: `frontend/src/app/[locale]/(app)/leaderboard/page.tsx`
- Modify: `frontend/src/app/[locale]/(app)/leaderboard/page.test.tsx` (create if absent)
- Modify: `frontend/src/app/[locale]/(kiosk)/station/page.tsx`, `page.test.tsx`
- Modify: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`

**Interfaces:**
- Consumes: the `kiosk` prop pattern.
- Produces: route `/[locale]/station/leaderboard`; `LeaderboardPage({ kiosk }: { kiosk?: boolean })`; message key `Station.leaderboard`.

- [ ] **Step 1: Find every navigation, and check the no-rank path**

Read `frontend/src/app/[locale]/(app)/leaderboard/page.tsx` (220 lines) and list every navigation target.

Then check something specific to this page: a station is **deliberately excluded from the rankings** — `leaderboard.Service.RecordPoint` returns early for a `kind='station'` profile, so a classroom PC never accumulates points and never appears. Find what this page renders for "your position" when the caller has no rank, and confirm it degrades cleanly rather than erroring or showing a misleading zero.

Write both findings into your report. If the no-rank path is broken, fix it under `kiosk` rather than changing behaviour for learners.

- [ ] **Step 2: Write the failing tests**

Add a `kiosk mode` block to the leaderboard page's test file asserting every listed target points under `/station/`, and a test that the page renders without error when the caller has no rank. Add to the kiosk home test:

```tsx
it("offers the leaderboard", async () => {
  apiGet.mockResolvedValue({ kind: "station", name: "PC-1" });
  render(<StationPage />);

  const link = await screen.findByRole("link", { name: "leaderboard" });
  expect(link).toHaveAttribute("href", "/uz-Latn/station/leaderboard");
});
```

- [ ] **Step 3: Run to verify they fail**

Run: `cd frontend && npm run test -- leaderboard`
Expected: FAIL.

- [ ] **Step 4: Thread the prop and create the wrapper**

Give `(app)/leaderboard/page.tsx` the `kiosk?: boolean` prop in the same shape as `(app)/practice/page.tsx`.

Create `frontend/src/app/[locale]/(kiosk)/station/leaderboard/page.tsx`:

```tsx
// Kiosk leaderboard entry point: /[locale]/station/leaderboard.
//
// A distinct URL from the learner app's /[locale]/leaderboard so the
// middleware exemption in src/proxy.ts stays narrow — only "station" and
// everything under it is login-free.
//
// Read-only by nature: stations are excluded from the rankings on purpose
// (leaderboard.Service.RecordPoint returns early for a kind='station'
// profile), so a classroom PC can see the board but never joins it.
import LeaderboardPage from "@/app/[locale]/(app)/leaderboard/page";

export default function KioskLeaderboardPage() {
  return <LeaderboardPage kiosk />;
}
```

Add the card and `Station.leaderboard` to all three locales: `"Reyting"` (uz-Latn), `"Рейтинг"` (uz-Cyrl), `"Рейтинг"` (ru).

- [ ] **Step 5: Run the full frontend gate**

```bash
cd frontend && npm run test -- leaderboard
make fe-check
```

Expected: both PASS. This is the last section, so `make fe-check` here is the gate for the whole plan: it typechecks every new route, builds them, and fails on any message key that is referenced but missing from a locale file.

- [ ] **Step 6: Verify the kiosk home reads as a whole**

Read `frontend/src/app/[locale]/(kiosk)/station/page.tsx` back. Seven cards now sit on a screen designed for two. Confirm the layout still works at a classroom monitor's size — the grid was `sm:grid-cols-2`. If seven cards need a different grid, change it now and say what you changed; do not leave a screen a student cannot use.

Confirm no card links to dashboard, premium, checkout, profile, arena, mistakes or grand mock, and say how you verified it.

- [ ] **Step 7: Commit**

```bash
git add "frontend/src/app/[locale]" frontend/messages
git commit -m "feat(kiosk): add the read-only leaderboard

Stations are excluded from the rankings by design, so the board is
visible but never joined -- students see where the bar is."
```

---

## Definition of Done

- `make fe-check` passes.
- The kiosk home offers exactly seven sections: practice, tickets, exam, signs, stats, saved, leaderboard.
- `kiosk-path.test.tsx` follows every kiosk wrapper into the page it reuses, and fails when a literal navigation target lands on a gated segment — proven by the Task 1 experiment.
- No kiosk page links to dashboard, premium, checkout, profile, arena, mistakes or grand mock.
- `frontend/src/lib/protected-segments.ts` is unchanged.

## Not in this plan

The installer, the agent and the admin surface — Plan 1, already complete. Offline tolerance, Arena, mistakes and grand mock are excluded by decision, with reasons recorded in spec section 6A.
