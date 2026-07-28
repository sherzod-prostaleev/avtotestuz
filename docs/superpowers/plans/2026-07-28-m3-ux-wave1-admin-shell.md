# M3-UX Wave 1 — Admin Control Center UX Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make all 44 existing admin pages usable and consistent on phone, tablet and desktop by replacing the scattered per-page table markup with one responsive primitive and rebuilding the shell chrome.

**Architecture:** Three layers, bottom-up. (1) A viewport hook + admin density tokens. (2) `AdminDataTable` v2 — dynamic row measurement, no forced min-width, and an automatic card-list rendering below `md` derived from the same `ColumnDef[]`, so every migrated page gets a correct mobile view without bespoke design work. (3) Shell chrome — collapsible icon sidebar, breadcrumb header, mobile bottom bar. Then the 9 hand-rolled `<table>` pages migrate onto the primitive.

**Tech Stack:** Next.js 14 App Router, TypeScript, Tailwind, TanStack Table v8 + TanStack Virtual, next-intl, lucide-react, Vitest + @testing-library/react.

## Global Constraints

- **Design tokens only.** Colors come from CSS vars already in `frontend/src/app/globals.css`: `background`, `card`, `border`, `foreground`, `muted-foreground`, `accent`, `accent-foreground`, `accent-shadow`, `success`, `danger`, `destructive`, `ring`, `gold`, `streak`. Never hard-code `hsl(220 28% 6%)` or any literal color. Source of truth: `docs/superpowers/specs/2026-07-25-driver-go-design-system-v2.md`.
- **Banned (design-system v2 §Tokens):** indigo/violet accents, purple blobs, glow logos, multi-layer glass, backdrop-blur on list surfaces.
- **Both themes must work.** `globals.css` defines a light palette; admin currently hard-codes dark. Every surface built or touched in this wave must render correctly in light and dark. SoT §7: "Dark/Light via existing `next-themes`."
- **Touch targets ≥ 44×44px** on interactive elements (design-system v2 §Mobile-first chrome rules).
- **Radius:** controls `rounded-xl`, panels `rounded-2xl` (M3-0 design system).
- **Type:** titles `font-display`, body `text-sm`, labels `text-[11px] uppercase tracking-wider` (M3-0 design system).
- **Focus:** every interactive element carries `focus-visible:ring-2 focus-visible:ring-ring`.
- **i18n:** no user-visible literal strings. Add keys to all three of `frontend/messages/uz-Latn.json`, `frontend/messages/uz-Cyrl.json`, `frontend/messages/ru.json`. Admin namespaces already exist (`AdminShell`, `AdminNav`, `AdminUsers`, …).
- **No behavior changes.** This wave is presentation only. Do not add, remove, or alter any `/admin/v1` API call, permission check, or mutation. `PermissionGate` wrappers stay exactly as they are.
- **No fake data.** Honest empty states only (M3-0: `ComingSoon` / `AdminEmptyState`).
- **Commands** run from `frontend/`: tests `npx vitest run <path>`, lint `npm run lint`, build `npm run build`.

---

## File Structure

**Create**
| File | Responsibility |
|---|---|
| `frontend/src/hooks/use-media-query.ts` | SSR-safe `matchMedia` subscription via `useSyncExternalStore`; exports `useMediaQuery` and `useIsCompact`. |
| `frontend/src/hooks/use-media-query.test.ts` | Hook tests. |
| `frontend/src/components/admin/admin-nav-config.tsx` | Nav IA data + lucide icon per group, split out of the sidebar component so both sidebar and mobile bar consume one source. |
| `frontend/src/components/admin/admin-mobile-bar.tsx` | Bottom tab bar for the 5 critical destinations on phones. |
| `frontend/src/components/admin/admin-mobile-bar.test.tsx` | Bar tests. |
| `frontend/src/components/admin/admin-breadcrumbs.tsx` | Derives breadcrumb trail from pathname + nav config. |
| `frontend/src/components/admin/admin-data-table.test.tsx` | Table v2 tests. |
| `frontend/src/components/admin/admin-sidebar.test.tsx` | Sidebar v2 tests. |

**Modify**
| File | Change |
|---|---|
| `frontend/src/app/globals.css` | Add `@layer components` admin density utilities. |
| `frontend/src/components/admin/admin-data-table.tsx` | v2: dynamic measurement, no min-width, card fallback, sticky column, row link. |
| `frontend/src/components/admin/admin-sidebar.tsx` | v2: consume `admin-nav-config`, collapsible groups, icons, rail mode, token colors. |
| `frontend/src/app/[locale]/admin/(shell)/layout.tsx` | v2 chrome: token colors, breadcrumbs, rail toggle, mobile bar, safe-area. |
| `frontend/src/app/[locale]/admin/(shell)/content/{questions,explanations,tickets,signs}/page.tsx` | Migrate raw `<table>` → `AdminDataTable`. |
| `frontend/src/app/[locale]/admin/(shell)/payments/{transactions,referral-payouts}/page.tsx` | Migrate raw `<table>` → `AdminDataTable`. |
| `frontend/src/app/[locale]/admin/(shell)/security/rbac/page.tsx` | Migrate raw `<table>` → `AdminDataTable`. |
| `frontend/src/app/[locale]/admin/(shell)/support/inbox/page.tsx` | Migrate raw `<table>` → `AdminDataTable`. |
| `frontend/src/app/[locale]/admin/(shell)/users/[id]/page.tsx` | Migrate raw `<table>` → `AdminDataTable`. |
| `frontend/src/app/[locale]/admin/(shell)/[[...stub]]/page.tsx` | Overview density: fill dead space, honest chart empty state. |
| `frontend/messages/{uz-Latn,uz-Cyrl,ru}.json` | New keys. |

---

### Task 1: Viewport hook + admin density tokens

**Files:**
- Create: `frontend/src/hooks/use-media-query.ts`
- Create: `frontend/src/hooks/use-media-query.test.ts`
- Modify: `frontend/src/app/globals.css` (append inside the existing `@layer components` block that starts at line 124)

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `useMediaQuery(query: string): boolean`
  - `useIsCompact(): boolean` — true when viewport is `max-width: 767px`. Returns `false` during SSR and first paint.
  - CSS classes: `.admin-shell-pad`, `.admin-stack`, `.admin-panel`, `.admin-label`, `.admin-scroll-x`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/hooks/use-media-query.test.ts`:

```ts
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useIsCompact, useMediaQuery } from "./use-media-query";

type Listener = () => void;

function mockMatchMedia(initialMatches: boolean) {
  const listeners = new Set<Listener>();
  const mql = {
    matches: initialMatches,
    media: "",
    addEventListener: (_: string, cb: Listener) => {
      listeners.add(cb);
    },
    removeEventListener: (_: string, cb: Listener) => {
      listeners.delete(cb);
    },
  };
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => mql),
  );
  return {
    set(next: boolean) {
      mql.matches = next;
      listeners.forEach((cb) => cb());
    },
    listenerCount: () => listeners.size,
  };
}

describe("useMediaQuery", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the current match state", () => {
    mockMatchMedia(true);
    const { result } = renderHook(() => useMediaQuery("(max-width: 767px)"));
    expect(result.current).toBe(true);
  });

  it("re-renders when the media query changes", () => {
    const ctl = mockMatchMedia(false);
    const { result } = renderHook(() => useMediaQuery("(max-width: 767px)"));
    expect(result.current).toBe(false);
    act(() => ctl.set(true));
    expect(result.current).toBe(true);
  });

  it("removes its listener on unmount", () => {
    const ctl = mockMatchMedia(false);
    const { unmount } = renderHook(() => useMediaQuery("(max-width: 767px)"));
    expect(ctl.listenerCount()).toBe(1);
    unmount();
    expect(ctl.listenerCount()).toBe(0);
  });

  it("useIsCompact asks for the md breakpoint", () => {
    mockMatchMedia(true);
    const { result } = renderHook(() => useIsCompact());
    expect(result.current).toBe(true);
    expect(matchMedia).toHaveBeenCalledWith("(max-width: 767px)");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/hooks/use-media-query.test.ts`
Expected: FAIL — `Failed to resolve import "./use-media-query"`.

- [ ] **Step 3: Write the implementation**

Create `frontend/src/hooks/use-media-query.ts`:

```ts
"use client";

import { useCallback, useSyncExternalStore } from "react";

/**
 * SSR-safe media query subscription.
 *
 * The server snapshot is always `false`, so the first client paint matches the
 * server HTML and React never reports a hydration mismatch. The real value
 * arrives on the effect tick right after hydration.
 */
export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onChange: () => void) => {
      const mql = window.matchMedia(query);
      mql.addEventListener("change", onChange);
      return () => mql.removeEventListener("change", onChange);
    },
    [query],
  );

  const getSnapshot = useCallback(() => window.matchMedia(query).matches, [query]);
  const getServerSnapshot = useCallback(() => false, []);

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

/** Phone-width layout: below Tailwind's `md` breakpoint. */
export function useIsCompact(): boolean {
  return useMediaQuery("(max-width: 767px)");
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/hooks/use-media-query.test.ts`
Expected: PASS — 4 tests.

- [ ] **Step 5: Add admin density utilities**

In `frontend/src/app/globals.css`, inside the existing `@layer components { … }` block (opens at line 124), append these rules just before that block's closing brace:

```css
  /* --- Admin Control Center density layer (M3-UX Wave 1) --- */
  .admin-shell-pad {
    @apply px-4 py-4 sm:px-6 sm:py-6;
    padding-bottom: max(1rem, env(safe-area-inset-bottom));
  }
  .admin-stack {
    @apply space-y-4 sm:space-y-5;
  }
  .admin-panel {
    @apply rounded-2xl border border-border bg-card;
  }
  .admin-label {
    @apply text-[11px] font-bold uppercase tracking-wider text-muted-foreground;
  }
  .admin-scroll-x {
    @apply overflow-x-auto;
    scrollbar-width: thin;
    -webkit-overflow-scrolling: touch;
  }
```

- [ ] **Step 6: Verify the stylesheet still compiles**

Run: `cd frontend && npm run build`
Expected: build succeeds, no Tailwind "class does not exist" error.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/hooks/use-media-query.ts frontend/src/hooks/use-media-query.test.ts frontend/src/app/globals.css
git commit -m "feat(admin): add SSR-safe viewport hook and admin density utilities"
```

---

### Task 2: AdminDataTable v2 — responsive, measured rows, card fallback

This is the keystone. The current component (`admin-data-table.tsx`) has three defects that this task fixes:
1. `min-w-[720px]` (line 57) forces horizontal overflow on every phone.
2. `estimateSize: () => estimateRowHeight` with absolutely-positioned rows and no `measureElement` — a wrapping cell overflows its fixed 44px row and its neighbours misalign.
3. No mobile representation at all.

**Files:**
- Modify: `frontend/src/components/admin/admin-data-table.tsx`
- Create: `frontend/src/components/admin/admin-data-table.test.tsx`
- Modify: `frontend/messages/uz-Latn.json`, `frontend/messages/uz-Cyrl.json`, `frontend/messages/ru.json`

**Interfaces:**
- Consumes: `useIsCompact` from Task 1; `AdminEmptyState` (existing).
- Produces:

```ts
export type AdminColumnMeta = {
  /** Hide this column in the mobile card (table view only). */
  hideOnCard?: boolean;
  /** Render as the card's title line instead of a labelled row. */
  cardTitle?: boolean;
  /** Right-align the desktop cell (numbers). */
  numeric?: boolean;
};

export type AdminDataTableProps<T> = {
  data: T[];
  columns: ColumnDef<T, unknown>[];
  emptyTitle: string;
  emptyDescription?: string;
  maxHeight?: number;          // default 560
  estimateRowHeight?: number;  // default 48
  getRowId?: (row: T) => string;
  /** Optional bespoke mobile card. When omitted, cards are derived from `columns`. */
  renderCard?: (row: T) => React.ReactNode;
  /** Force a representation; default is viewport-driven. */
  variant?: "auto" | "table" | "cards";
};

export function AdminDataTable<T>(props: AdminDataTableProps<T>): JSX.Element;
```

Set column meta with TanStack's `meta` field, e.g. `{ accessorKey: "amount", header: "Sum", meta: { numeric: true } satisfies AdminColumnMeta }`.

- [ ] **Step 1: Add the i18n keys**

Add a new top-level `"AdminTable"` namespace to each messages file.

`frontend/messages/uz-Latn.json`:
```json
  "AdminTable": {
    "rowsLabel": "Jadval qatorlari",
    "cardsLabel": "Yozuvlar ro‘yxati"
  },
```

`frontend/messages/uz-Cyrl.json`:
```json
  "AdminTable": {
    "rowsLabel": "Жадвал қаторлари",
    "cardsLabel": "Ёзувлар рўйхати"
  },
```

`frontend/messages/ru.json`:
```json
  "AdminTable": {
    "rowsLabel": "Строки таблицы",
    "cardsLabel": "Список записей"
  },
```

- [ ] **Step 2: Write the failing test**

Create `frontend/src/components/admin/admin-data-table.test.tsx`:

```tsx
import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ColumnDef } from "@tanstack/react-table";
import { AdminDataTable, type AdminColumnMeta } from "./admin-data-table";

type Row = { id: string; name: string; amount: number; secret: string };

const rows: Row[] = [
  { id: "1", name: "Musharraf Qodirova", amount: 49000, secret: "s1" },
  { id: "2", name: "Ali Valiyev", amount: 12000, secret: "s2" },
];

const columns: ColumnDef<Row, unknown>[] = [
  {
    accessorKey: "name",
    header: "Ism",
    meta: { cardTitle: true } satisfies AdminColumnMeta,
  },
  {
    accessorKey: "amount",
    header: "Summa",
    meta: { numeric: true } satisfies AdminColumnMeta,
  },
  {
    accessorKey: "secret",
    header: "Sirli",
    meta: { hideOnCard: true } satisfies AdminColumnMeta,
  },
];

const messages = {
  AdminTable: { rowsLabel: "Jadval qatorlari", cardsLabel: "Yozuvlar ro‘yxati" },
};

function renderTable(ui: React.ReactNode) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      {ui}
    </NextIntlClientProvider>,
  );
}

describe("AdminDataTable", () => {
  it("shows the empty state when there is no data", () => {
    renderTable(
      <AdminDataTable data={[]} columns={columns} emptyTitle="Bo‘sh" emptyDescription="Yo‘q" />,
    );
    expect(screen.getByText("Bo‘sh")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("renders a real table in table variant", () => {
    renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="table" />,
    );
    const table = screen.getByRole("table");
    expect(within(table).getByText("Ism")).toBeInTheDocument();
    expect(within(table).getByText("Musharraf Qodirova")).toBeInTheDocument();
  });

  it("never forces a fixed minimum width that overflows phones", () => {
    const { container } = renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="table" />,
    );
    expect(container.querySelector('[class*="min-w-[720px]"]')).toBeNull();
  });

  it("renders labelled cards in cards variant", () => {
    renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="cards" />,
    );
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    const list = screen.getByRole("list", { name: "Yozuvlar ro‘yxati" });
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    // cardTitle column renders as the heading, without its label
    expect(within(items[0]).getByText("Musharraf Qodirova")).toBeInTheDocument();
    // labelled row for a normal column
    expect(within(items[0]).getByText("Summa")).toBeInTheDocument();
    expect(within(items[0]).getByText("49000")).toBeInTheDocument();
  });

  it("omits hideOnCard columns from cards but keeps them in the table", () => {
    const cards = renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="cards" />,
    );
    expect(screen.queryByText("Sirli")).not.toBeInTheDocument();
    cards.unmount();

    renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="table" />,
    );
    expect(screen.getByText("Sirli")).toBeInTheDocument();
  });

  it("prefers a bespoke renderCard when supplied", () => {
    renderTable(
      <AdminDataTable
        data={rows}
        columns={columns}
        emptyTitle="Bo‘sh"
        variant="cards"
        renderCard={(row) => <span>custom:{row.name}</span>}
      />,
    );
    expect(screen.getByText("custom:Musharraf Qodirova")).toBeInTheDocument();
    expect(screen.queryByText("Summa")).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/components/admin/admin-data-table.test.tsx`
Expected: FAIL — `AdminColumnMeta` is not exported, and the cards/variant tests fail because the component has no card mode.

- [ ] **Step 4: Rewrite the component**

Replace the entire contents of `frontend/src/components/admin/admin-data-table.tsx`:

```tsx
"use client";

import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type Cell,
  type ColumnDef,
  type Row as TanRow,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useRef } from "react";
import { useTranslations } from "next-intl";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { useIsCompact } from "@/hooks/use-media-query";

export type AdminColumnMeta = {
  /** Hide this column in the mobile card (table view only). */
  hideOnCard?: boolean;
  /** Render as the card's title line instead of a labelled row. */
  cardTitle?: boolean;
  /** Right-align the desktop cell (numbers). */
  numeric?: boolean;
};

function metaOf<T>(cell: Cell<T, unknown>): AdminColumnMeta {
  return (cell.column.columnDef.meta ?? {}) as AdminColumnMeta;
}

export type AdminDataTableProps<T> = {
  data: T[];
  columns: ColumnDef<T, unknown>[];
  emptyTitle: string;
  emptyDescription?: string;
  maxHeight?: number;
  estimateRowHeight?: number;
  getRowId?: (row: T) => string;
  /** Optional bespoke mobile card. When omitted, cards are derived from `columns`. */
  renderCard?: (row: T) => React.ReactNode;
  /** Force a representation; default is viewport-driven. */
  variant?: "auto" | "table" | "cards";
};

/**
 * One table primitive for every admin directory.
 *
 * Desktop renders a virtualized table whose rows are *measured*, not assumed —
 * a wrapping cell grows its own row instead of overlapping the next one.
 * Below `md` the same `columns` are re-projected as a card list, so a page gets
 * a correct phone layout without writing a second markup tree.
 */
export function AdminDataTable<T>({
  data,
  columns,
  emptyTitle,
  emptyDescription,
  maxHeight = 560,
  estimateRowHeight = 48,
  getRowId,
  renderCard,
  variant = "auto",
}: AdminDataTableProps<T>) {
  const t = useTranslations("AdminTable");
  const isCompact = useIsCompact();
  const parentRef = useRef<HTMLDivElement>(null);

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getRowId: getRowId ? (row) => getRowId(row) : undefined,
  });
  const rows = table.getRowModel().rows;

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => estimateRowHeight,
    overscan: 12,
    // Measure the real rendered height so wrapping content cannot overlap.
    measureElement: (el) => el.getBoundingClientRect().height,
  });

  if (data.length === 0) {
    return <AdminEmptyState title={emptyTitle} description={emptyDescription} />;
  }

  const asCards = variant === "cards" || (variant === "auto" && isCompact);

  if (asCards) {
    return (
      <ul role="list" aria-label={t("cardsLabel")} className="space-y-2.5">
        {rows.map((row) => (
          <li key={row.id} className="admin-panel p-3.5">
            {renderCard ? renderCard(row.original) : <DerivedCard row={row} />}
          </li>
        ))}
      </ul>
    );
  }

  return (
    <div className="admin-panel overflow-hidden">
      <div
        ref={parentRef}
        className="admin-scroll-x relative w-full overflow-y-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
        style={{ maxHeight }}
        role="region"
        aria-label={t("rowsLabel")}
        tabIndex={0}
      >
        <table className="w-full border-collapse text-left text-sm">
          <thead className="sticky top-0 z-10 bg-card">
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id} className="border-b border-border">
                {hg.headers.map((h) => {
                  const meta = (h.column.columnDef.meta ?? {}) as AdminColumnMeta;
                  return (
                    <th
                      key={h.id}
                      scope="col"
                      className={`admin-label whitespace-nowrap px-3 py-2.5 ${
                        meta.numeric ? "text-right" : "text-left"
                      }`}
                    >
                      {h.isPlaceholder
                        ? null
                        : flexRender(h.column.columnDef.header, h.getContext())}
                    </th>
                  );
                })}
              </tr>
            ))}
          </thead>
          <tbody
            style={{
              height: `${virtualizer.getTotalSize()}px`,
              position: "relative",
            }}
          >
            {virtualizer.getVirtualItems().map((vRow) => {
              const row = rows[vRow.index];
              return (
                <tr
                  key={row.id}
                  data-index={vRow.index}
                  ref={virtualizer.measureElement}
                  className="absolute left-0 w-full border-b border-border/60 hover:bg-accent/[0.06]"
                  style={{ transform: `translateY(${vRow.start}px)` }}
                >
                  {row.getVisibleCells().map((cell) => {
                    const meta = metaOf(cell);
                    return (
                      <td
                        key={cell.id}
                        className={`px-3 py-2.5 align-middle ${
                          meta.numeric ? "text-right tabular-nums" : ""
                        }`}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/** Card projection of a row: title column on top, remaining columns as label/value pairs. */
function DerivedCard<T>({ row }: { row: TanRow<T> }) {
  const cells = row.getVisibleCells();
  const titleCell = cells.find((c) => metaOf(c).cardTitle);
  const bodyCells = cells.filter((c) => {
    const meta = metaOf(c);
    return !meta.cardTitle && !meta.hideOnCard;
  });

  return (
    <div className="space-y-2">
      {titleCell ? (
        <div className="font-display text-sm font-bold">
          {flexRender(titleCell.column.columnDef.cell, titleCell.getContext())}
        </div>
      ) : null}
      <dl className="grid grid-cols-[minmax(6rem,auto)_1fr] gap-x-3 gap-y-1.5">
        {bodyCells.map((cell) => (
          <div key={cell.id} className="contents">
            <dt className="admin-label self-center">
              {typeof cell.column.columnDef.header === "string"
                ? cell.column.columnDef.header
                : cell.column.id}
            </dt>
            <dd className="min-w-0 break-words text-sm">
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
```

The `<dt>` reads `columnDef.header` directly rather than through `flexRender`, because `flexRender` on a header needs a *header* context and only a *cell* context is available here. Every admin column in this codebase declares `header` as a plain translated string, so the direct read is correct; the `column.id` branch is the safety net for a future function header.

- [ ] **Step 5: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/components/admin/admin-data-table.test.tsx`
Expected: PASS — 6 tests.

- [ ] **Step 6: Verify the existing consumer still passes and typechecks**

Run: `cd frontend && npx vitest run src/components/admin && npm run lint`
Expected: all admin component tests PASS, lint clean.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/admin/admin-data-table.tsx frontend/src/components/admin/admin-data-table.test.tsx frontend/messages
git commit -m "feat(admin): responsive data table with measured rows and card fallback"
```

---

### Task 3: Nav config + sidebar v2 (collapsible groups, icons, tokens)

Today `adminNav()` lives inside `admin-sidebar.tsx` and emits 40 flat links, forcing a 2352px-tall sidebar. This task extracts the IA, adds an icon per group, collapses groups by default except the active one, and removes the hard-coded `hsl(220 28% 7%)` so light theme works.

**Files:**
- Create: `frontend/src/components/admin/admin-nav-config.tsx`
- Create: `frontend/src/components/admin/admin-sidebar.test.tsx`
- Modify: `frontend/src/components/admin/admin-sidebar.tsx`
- Modify: `frontend/messages/{uz-Latn,uz-Cyrl,ru}.json`

**Interfaces:**
- Consumes: `ADMIN_ROUTE_PERMISSIONS`, `hasPermission`, `routePermissionKey` from `@/lib/admin-permissions` (existing); `useAdminMeOptional` (existing).
- Produces:

```ts
export type AdminNavItem = { href: string; labelKey: string; stub?: boolean };
export type AdminNavGroup = {
  titleKey: string;
  icon: LucideIcon;
  items: AdminNavItem[];
};
export function adminNav(locale: string): AdminNavGroup[];
/** The 5 phone-critical destinations, in bar order. */
export function adminMobilePrimary(locale: string): { href: string; labelKey: string; icon: LucideIcon }[];
export function activeGroupTitleKey(groups: AdminNavGroup[], pathname: string): string | null;
```

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/admin/admin-sidebar.test.tsx`:

```tsx
import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { AdminSidebar } from "./admin-sidebar";
import { activeGroupTitleKey, adminNav } from "./admin-nav-config";
// Use the real catalogue: the sidebar renders every group, so a hand-written
// subset would make next-intl warn about missing keys and mask real gaps.
import messages from "../../../messages/uz-Latn.json";

function renderSidebar(activePath: string) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <AdminSidebar locale="uz-Latn" activePath={activePath} />
    </NextIntlClientProvider>,
  );
}

describe("adminNav config", () => {
  it("gives every group an icon", () => {
    for (const group of adminNav("uz-Latn")) {
      expect(group.icon, `${group.titleKey} has no icon`).toBeTruthy();
    }
  });

  it("resolves the active group from a nested pathname", () => {
    const groups = adminNav("uz-Latn");
    expect(activeGroupTitleKey(groups, "/uz-Latn/admin/payments/transactions/abc")).toBe(
      "groupPayments",
    );
    expect(activeGroupTitleKey(groups, "/uz-Latn/admin/nowhere")).toBeNull();
  });
});

describe("AdminSidebar", () => {
  it("opens only the active group by default", () => {
    renderSidebar("/uz-Latn/admin/users");
    expect(screen.getByRole("link", { name: /Katalog/ })).toBeInTheDocument();
    // A link from a non-active group is not rendered while collapsed.
    expect(screen.queryByRole("link", { name: /Tranzaksiyalar/ })).not.toBeInTheDocument();
  });

  it("expands a collapsed group when its header is clicked", async () => {
    const user = userEvent.setup();
    renderSidebar("/uz-Latn/admin/users");
    await user.click(screen.getByRole("button", { name: /To‘lovlar/ }));
    expect(screen.getByRole("link", { name: /Tranzaksiyalar/ })).toBeInTheDocument();
  });

  it("marks the active link with aria-current", () => {
    renderSidebar("/uz-Latn/admin/users");
    expect(screen.getByRole("link", { name: /Katalog/ })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("uses design tokens, not hard-coded colors", () => {
    const { container } = renderSidebar("/uz-Latn/admin/users");
    expect(container.innerHTML).not.toMatch(/hsl\(220_28%_7%\)/);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/components/admin/admin-sidebar.test.tsx`
Expected: FAIL — `Failed to resolve import "./admin-nav-config"`.

- [ ] **Step 3: Add the i18n keys**

Add to the existing `AdminNav` namespace in all three messages files:

`uz-Latn.json`: `"collapseGroup": "Guruhni yig‘ish", "expandGroup": "Guruhni ochish", "collapseRail": "Menyuni toraytirish", "expandRail": "Menyuni kengaytirish"`

`uz-Cyrl.json`: `"collapseGroup": "Гуруҳни йиғиш", "expandGroup": "Гуруҳни очиш", "collapseRail": "Менюни торайтириш", "expandRail": "Менюни кенгайтириш"`

`ru.json`: `"collapseGroup": "Свернуть группу", "expandGroup": "Развернуть группу", "collapseRail": "Свернуть меню", "expandRail": "Развернуть меню"`

- [ ] **Step 4: Create the nav config**

Create `frontend/src/components/admin/admin-nav-config.tsx`. Copy the exact group and item lists from the current `adminNav()` in `admin-sidebar.tsx` (lines 23–116) — same `href`s, same `labelKey`s, same `stub` flags — and add an `icon` per group:

```tsx
import {
  Activity,
  BarChart3,
  Briefcase,
  Building2,
  CreditCard,
  FileText,
  LayoutDashboard,
  LifeBuoy,
  type LucideIcon,
  Settings,
  Shield,
  Users,
} from "lucide-react";

export type AdminNavItem = { href: string; labelKey: string; stub?: boolean };

export type AdminNavGroup = {
  titleKey: string;
  icon: LucideIcon;
  items: AdminNavItem[];
};

/** Sidebar IA from the M3 SoT (docs/superpowers/specs/2026-07-26-m3-super-admin-control-center.md §2). */
export function adminNav(locale: string): AdminNavGroup[] {
  const base = `/${locale}/admin`;
  return [
    {
      titleKey: "groupMain",
      icon: LayoutDashboard,
      items: [{ href: base, labelKey: "overview" }],
    },
    {
      titleKey: "groupMonitoring",
      icon: Activity,
      items: [
        { href: `${base}/monitoring/health`, labelKey: "systemHealth" },
        { href: `${base}/monitoring/perf`, labelKey: "apiDb" },
        { href: `${base}/monitoring/logs`, labelKey: "liveLogs" },
        { href: `${base}/monitoring/jobs`, labelKey: "jobs" },
        { href: `${base}/monitoring/alerts`, labelKey: "alerts" },
      ],
    },
    {
      titleKey: "groupAnalytics",
      icon: BarChart3,
      items: [
        { href: `${base}/analytics/overview`, labelKey: "overview" },
        { href: `${base}/analytics/funnels`, labelKey: "funnels", stub: true },
        { href: `${base}/analytics/exports`, labelKey: "exports", stub: true },
      ],
    },
    {
      titleKey: "groupInvestors",
      icon: Briefcase,
      items: [{ href: `${base}/investors`, labelKey: "overview" }],
    },
    {
      titleKey: "groupB2B",
      icon: Building2,
      items: [{ href: `${base}/b2b/orgs`, labelKey: "organizations" }],
    },
    {
      titleKey: "groupUsers",
      icon: Users,
      items: [{ href: `${base}/users`, labelKey: "directory" }],
    },
    {
      titleKey: "groupContent",
      icon: FileText,
      items: [
        { href: `${base}/content/questions`, labelKey: "questions" },
        { href: `${base}/content/explanations`, labelKey: "explanations" },
        { href: `${base}/content/tickets`, labelKey: "tickets" },
        { href: `${base}/content/signs`, labelKey: "signs" },
      ],
    },
    {
      titleKey: "groupPayments",
      icon: CreditCard,
      items: [
        { href: `${base}/payments/transactions`, labelKey: "transactions" },
        { href: `${base}/payments/referral-payouts`, labelKey: "referralPayouts" },
        { href: `${base}/payments/manual`, labelKey: "manualPay" },
        { href: `${base}/payments/refunds`, labelKey: "refunds" },
        { href: `${base}/payments/webhooks`, labelKey: "webhooks", stub: true },
        { href: `${base}/payments/providers`, labelKey: "providers" },
        { href: `${base}/payments/catalog`, labelKey: "catalog", stub: true },
        { href: `${base}/payments/recon`, labelKey: "recon" },
      ],
    },
    {
      titleKey: "groupCMS",
      icon: FileText,
      items: [
        { href: `${base}/cms/home`, labelKey: "homepage" },
        { href: `${base}/cms/chrome`, labelKey: "headerFooter" },
        { href: `${base}/cms/brand`, labelKey: "brand", stub: true },
        { href: `${base}/cms/surfaces`, labelKey: "surfaces", stub: true },
        { href: `${base}/cms/legal`, labelKey: "legal" },
      ],
    },
    {
      titleKey: "groupSettings",
      icon: Settings,
      items: [
        { href: `${base}/settings/flags`, labelKey: "featureFlags" },
        { href: `${base}/settings/limits`, labelKey: "limits" },
        { href: `${base}/settings/config`, labelKey: "runtimeConfig", stub: true },
      ],
    },
    {
      titleKey: "groupSecurity",
      icon: Shield,
      items: [
        { href: `${base}/security/totp`, labelKey: "totp" },
        { href: `${base}/security/rbac`, labelKey: "adminsRbac" },
        { href: `${base}/security/ip`, labelKey: "ipAllowlist", stub: true },
        { href: `${base}/security/audit`, labelKey: "auditLog" },
      ],
    },
    {
      titleKey: "groupSupport",
      icon: LifeBuoy,
      items: [
        { href: `${base}/support/inbox`, labelKey: "inbox" },
        { href: `${base}/support/broadcasts`, labelKey: "broadcasts" },
      ],
    },
  ];
}

/** Phone bottom bar: the destinations an operator needs while away from a desk. */
export function adminMobilePrimary(locale: string) {
  const base = `/${locale}/admin`;
  return [
    { href: base, labelKey: "overview", icon: LayoutDashboard },
    { href: `${base}/payments/manual`, labelKey: "manualPay", icon: CreditCard },
    { href: `${base}/users`, labelKey: "directory", icon: Users },
    { href: `${base}/monitoring/health`, labelKey: "systemHealth", icon: Activity },
    { href: `${base}/support/inbox`, labelKey: "inbox", icon: LifeBuoy },
  ];
}

function isActive(href: string, pathname: string, base: string): boolean {
  if (href === base) return pathname === base;
  return pathname === href || pathname.startsWith(href + "/");
}

/** Which group contains the current route, or null. */
export function activeGroupTitleKey(
  groups: AdminNavGroup[],
  pathname: string,
): string | null {
  const base = groups[0]?.items[0]?.href ?? "";
  for (const group of groups) {
    if (group.items.some((item) => isActive(item.href, pathname, base))) {
      return group.titleKey;
    }
  }
  return null;
}

export { isActive as isNavItemActive };
```

- [ ] **Step 5: Rewrite the sidebar**

Replace `frontend/src/components/admin/admin-sidebar.tsx`. Keep the existing permission filtering logic verbatim (`routePermissionKey` → `ADMIN_ROUTE_PERMISSIONS` → `hasPermission`); change only presentation:

```tsx
"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ChevronDown } from "lucide-react";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import {
  activeGroupTitleKey,
  adminNav,
  isNavItemActive,
  type AdminNavGroup,
} from "@/components/admin/admin-nav-config";
import {
  ADMIN_ROUTE_PERMISSIONS,
  hasPermission,
  routePermissionKey,
} from "@/lib/admin-permissions";

export { adminNav } from "@/components/admin/admin-nav-config";
export type { AdminNavGroup, AdminNavItem } from "@/components/admin/admin-nav-config";

type AdminSidebarProps = {
  locale: string;
  activePath: string;
  mobileOpen?: boolean;
  onNavigate?: () => void;
};

export function AdminSidebar({
  locale,
  activePath,
  mobileOpen,
  onNavigate,
}: AdminSidebarProps) {
  const t = useTranslations("AdminNav");
  const me = useAdminMeOptional();
  const groups = adminNav(locale);
  const activeGroup = activeGroupTitleKey(groups, activePath);
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({});
  const base = `/${locale}/admin`;

  function isOpen(group: AdminNavGroup): boolean {
    return openGroups[group.titleKey] ?? group.titleKey === activeGroup;
  }

  function toggle(titleKey: string, current: boolean) {
    setOpenGroups((prev) => ({ ...prev, [titleKey]: !current }));
  }

  return (
    <aside
      className={`fixed inset-y-0 left-0 z-40 flex w-[272px] flex-col border-r border-border bg-card transition-transform lg:static lg:translate-x-0 ${
        mobileOpen ? "translate-x-0" : "-translate-x-full"
      }`}
    >
      <div className="border-b border-border px-4 py-4">
        <p className="font-display text-xl font-black tracking-tight">Driver Go</p>
        <p className="mt-0.5 text-[10px] font-extrabold uppercase tracking-[0.2em] text-accent">
          {t("badge")}
        </p>
        {me?.email ? (
          <p className="mt-3 truncate rounded-xl bg-muted px-2 py-1 text-[11px] text-muted-foreground">
            {me.email}
          </p>
        ) : null}
      </div>

      <nav className="flex-1 overflow-y-auto px-2 py-3" aria-label={t("badge")}>
        {groups.map((group) => {
          const visibleItems = group.items.filter((item) => {
            const key = routePermissionKey(item.href, locale);
            if (!key) return true;
            const need = ADMIN_ROUTE_PERMISSIONS[key];
            if (!need) return true;
            return hasPermission(me?.permissions, need);
          });
          if (visibleItems.length === 0) return null;

          const open = isOpen(group);
          const Icon = group.icon;
          const panelId = `admin-nav-${group.titleKey}`;

          return (
            <div key={group.titleKey} className="mb-1">
              <button
                type="button"
                onClick={() => toggle(group.titleKey, open)}
                aria-expanded={open}
                aria-controls={panelId}
                aria-label={`${t(group.titleKey)} — ${open ? t("collapseGroup") : t("expandGroup")}`}
                className="flex min-h-[44px] w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-left text-[13px] font-bold text-foreground/90 transition-colors hover:bg-accent/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <Icon className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden />
                <span className="flex-1 truncate">{t(group.titleKey)}</span>
                <ChevronDown
                  aria-hidden
                  className={`h-4 w-4 shrink-0 text-muted-foreground transition-transform ${
                    open ? "rotate-180" : ""
                  }`}
                />
              </button>

              {open ? (
                <ul id={panelId} className="mb-2 mt-0.5 space-y-0.5 pl-6">
                  {visibleItems.map((item) => {
                    const active = isNavItemActive(item.href, activePath, base);
                    return (
                      <li key={item.href}>
                        <Link
                          href={item.href}
                          onClick={onNavigate}
                          aria-current={active ? "page" : undefined}
                          className={`flex min-h-[40px] items-center justify-between gap-2 rounded-xl px-2.5 py-1.5 text-[13px] font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                            active
                              ? "bg-accent text-accent-foreground"
                              : "text-foreground/80 hover:bg-accent/10 hover:text-accent"
                          }`}
                        >
                          <span className="truncate">{t(item.labelKey)}</span>
                          {item.stub ? (
                            <span
                              className={`shrink-0 text-[9px] font-bold uppercase ${
                                active ? "text-accent-foreground/70" : "text-muted-foreground"
                              }`}
                            >
                              {t("soon")}
                            </span>
                          ) : null}
                        </Link>
                      </li>
                    );
                  })}
                </ul>
              ) : null}
            </div>
          );
        })}
      </nav>
    </aside>
  );
}
```

If the previous file had trailing content after the `<nav>` (a footer block), preserve it below the `</nav>` using token colors.

- [ ] **Step 6: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/components/admin/admin-sidebar.test.tsx`
Expected: PASS — 6 tests.

- [ ] **Step 7: Verify nothing else broke**

Run: `cd frontend && npx vitest run && npm run lint`
Expected: full suite PASS, lint clean.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/admin/admin-nav-config.tsx frontend/src/components/admin/admin-sidebar.tsx frontend/src/components/admin/admin-sidebar.test.tsx frontend/messages
git commit -m "feat(admin): collapsible icon sidebar driven by shared nav config"
```

---

### Task 4: Shell v2 — breadcrumbs, theme-correct chrome, mobile bottom bar

**Files:**
- Create: `frontend/src/components/admin/admin-breadcrumbs.tsx`
- Create: `frontend/src/components/admin/admin-mobile-bar.tsx`
- Create: `frontend/src/components/admin/admin-mobile-bar.test.tsx`
- Modify: `frontend/src/app/[locale]/admin/(shell)/layout.tsx`
- Modify: `frontend/messages/{uz-Latn,uz-Cyrl,ru}.json`

**Interfaces:**
- Consumes: `adminNav`, `adminMobilePrimary`, `isNavItemActive`, `activeGroupTitleKey` from Task 3.
- Produces:
  - `<AdminBreadcrumbs locale={string} pathname={string} />`
  - `<AdminMobileBar locale={string} activePath={string} />`

- [ ] **Step 1: Add the i18n keys**

Add to the existing `AdminShell` namespace in all three files:

`uz-Latn.json`: `"breadcrumbLabel": "Joylashuv", "primaryNav": "Asosiy bo‘limlar", "home": "Boshqaruv markazi"`
`uz-Cyrl.json`: `"breadcrumbLabel": "Жойлашув", "primaryNav": "Асосий бўлимлар", "home": "Бошқарув маркази"`
`ru.json`: `"breadcrumbLabel": "Навигация", "primaryNav": "Основные разделы", "home": "Центр управления"`

- [ ] **Step 2: Write the failing test**

Create `frontend/src/components/admin/admin-mobile-bar.test.tsx`:

```tsx
import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { AdminMobileBar } from "./admin-mobile-bar";
import { AdminMeProvider } from "./admin-me-context";
import messages from "../../../messages/uz-Latn.json";

// Four of the five thumb-zone destinations are permission-gated routes
// (payments, users, monitoring, support). The bar must answer the same way
// the sidebar does — it is the phone's only nav, so an item here is a promise.
const ALL_PERMISSIONS = [
  "payments.read",
  "users.read",
  "monitoring.read",
  "support.inbox",
];

function renderBar(activePath: string, permissions: string[] = ALL_PERMISSIONS) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <AdminMeProvider
        me={{
          id: "1",
          email: "admin@example.com",
          display_name: "Admin",
          roles: ["admin"],
          permissions,
        }}
      >
        <AdminMobileBar locale="uz-Latn" activePath={activePath} />
      </AdminMeProvider>
    </NextIntlClientProvider>,
  );
}

describe("AdminMobileBar", () => {
  it("renders the five critical destinations", () => {
    renderBar("/uz-Latn/admin");
    const nav = screen.getByRole("navigation", { name: "Asosiy bo‘limlar" });
    expect(within(nav).getAllByRole("link")).toHaveLength(5);
  });

  it("marks the current destination", () => {
    renderBar("/uz-Latn/admin/payments/manual");
    expect(screen.getByRole("link", { name: /Manual Humo/ })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("does not mark overview as current on a nested route", () => {
    renderBar("/uz-Latn/admin/users");
    expect(screen.getByRole("link", { name: /Umumiy/ })).not.toHaveAttribute("aria-current");
  });

  it("hides destinations the admin has no permission for", () => {
    renderBar("/uz-Latn/admin", ["support.inbox"]);
    const nav = screen.getByRole("navigation", { name: "Asosiy bo‘limlar" });
    // Overview is ungated, support.inbox is held -> 2 links, nothing else.
    expect(within(nav).getAllByRole("link")).toHaveLength(2);
    expect(screen.queryByRole("link", { name: /Manual Humo/ })).not.toBeInTheDocument();
  });

  it("fails closed when the admin has no permissions at all", () => {
    renderBar("/uz-Latn/admin", []);
    const nav = screen.getByRole("navigation", { name: "Asosiy bo‘limlar" });
    expect(within(nav).getAllByRole("link")).toHaveLength(1);
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/components/admin/admin-mobile-bar.test.tsx`
Expected: FAIL — `Failed to resolve import "./admin-mobile-bar"`.

- [ ] **Step 4: Create the mobile bar**

Create `frontend/src/components/admin/admin-mobile-bar.tsx`:

```tsx
"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import {
  adminMobilePrimary,
  isNavItemActive,
} from "@/components/admin/admin-nav-config";
import {
  ADMIN_ROUTE_PERMISSIONS,
  hasPermission,
  routePermissionKey,
} from "@/lib/admin-permissions";

/** Thumb-zone navigation for the operations an admin runs from a phone. */
export function AdminMobileBar({
  locale,
  activePath,
}: {
  locale: string;
  activePath: string;
}) {
  const t = useTranslations("AdminNav");
  const tShell = useTranslations("AdminShell");
  const me = useAdminMeOptional();
  const base = `/${locale}/admin`;
  // Same gate as the sidebar, and for the same reason: on a phone this bar is
  // the whole navigation, so an item it shows is a capability claim.
  const items = adminMobilePrimary(locale).filter((item) => {
    const key = routePermissionKey(item.href, locale);
    if (!key) return true;
    const need = ADMIN_ROUTE_PERMISSIONS[key];
    if (!need) return true;
    return hasPermission(me?.permissions, need);
  });

  return (
    <nav
      aria-label={tShell("primaryNav")}
      className="fixed inset-x-0 bottom-0 z-30 border-t border-border bg-card lg:hidden"
      style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
    >
      <ul className="flex items-stretch justify-between">
        {items.map((item) => {
          const active = isNavItemActive(item.href, activePath, base);
          const Icon = item.icon;
          return (
            <li key={item.href} className="flex-1">
              <Link
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={`flex min-h-[56px] flex-col items-center justify-center gap-1 px-1 py-2 text-[10px] font-bold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
                  active ? "text-accent" : "text-muted-foreground"
                }`}
              >
                <Icon className="h-5 w-5" aria-hidden />
                <span className="w-full truncate text-center">{t(item.labelKey)}</span>
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/components/admin/admin-mobile-bar.test.tsx`
Expected: PASS — 3 tests.

- [ ] **Step 6: Create the breadcrumbs component**

Create `frontend/src/components/admin/admin-breadcrumbs.tsx`:

```tsx
"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ChevronRight } from "lucide-react";
import {
  activeGroupTitleKey,
  adminNav,
  isNavItemActive,
} from "@/components/admin/admin-nav-config";

/** Location trail derived from the nav IA — group, then leaf. */
export function AdminBreadcrumbs({
  locale,
  pathname,
}: {
  locale: string;
  pathname: string;
}) {
  const t = useTranslations("AdminNav");
  const tShell = useTranslations("AdminShell");
  const base = `/${locale}/admin`;
  const groups = adminNav(locale);
  const groupKey = activeGroupTitleKey(groups, pathname);
  const group = groups.find((g) => g.titleKey === groupKey);
  const leaf = group?.items.find((item) => isNavItemActive(item.href, pathname, base));

  return (
    <nav aria-label={tShell("breadcrumbLabel")} className="min-w-0">
      <ol className="flex min-w-0 items-center gap-1.5 text-xs font-semibold text-muted-foreground">
        <li className="shrink-0">
          <Link
            href={base}
            className="rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring hover:text-accent"
          >
            {tShell("home")}
          </Link>
        </li>
        {group ? (
          <>
            <ChevronRight aria-hidden className="h-3.5 w-3.5 shrink-0" />
            <li className="shrink-0">{t(group.titleKey)}</li>
          </>
        ) : null}
        {leaf ? (
          <>
            <ChevronRight aria-hidden className="h-3.5 w-3.5 shrink-0" />
            <li className="min-w-0 truncate text-foreground" aria-current="page">
              {t(leaf.labelKey)}
            </li>
          </>
        ) : null}
      </ol>
    </nav>
  );
}
```

- [ ] **Step 7: Rewrite the shell layout**

In `frontend/src/app/[locale]/admin/(shell)/layout.tsx`, keep the auth `useEffect`, `logout`, and `AdminMeProvider` exactly as they are. Change only the returned markup and the loading state:

Replace the loading return (lines 53–59) with:

```tsx
  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        {t("loading")}
      </div>
    );
  }
```

Replace the main return (lines 63–122) with:

```tsx
  return (
    <AdminMeProvider me={me}>
      <div className="flex min-h-screen bg-background text-foreground">
        {navOpen ? (
          <button
            type="button"
            className="fixed inset-0 z-30 bg-foreground/40 lg:hidden"
            aria-label={t("closeNav")}
            onClick={() => setNavOpen(false)}
          />
        ) : null}
        <AdminSidebar
          locale={locale}
          activePath={pathname}
          mobileOpen={navOpen}
          onNavigate={() => setNavOpen(false)}
        />
        <div className="flex min-w-0 flex-1 flex-col">
          <header className="sticky top-0 z-20 flex items-center justify-between gap-3 border-b border-border bg-background px-4 py-2.5 sm:px-6">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="min-h-[44px] min-w-[44px] lg:hidden"
                aria-label={t("openNav")}
                onClick={() => setNavOpen(true)}
              >
                <Menu className="h-4 w-4" />
              </Button>
              <AdminBreadcrumbs locale={locale} pathname={pathname} />
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <p className="hidden text-[11px] text-muted-foreground xl:block">
                {t("commandHint")}
              </p>
              {me.totp_setup_required ? (
                <Link
                  href={`/${locale}/admin/security/totp`}
                  className="rounded-xl border border-accent/50 bg-accent/10 px-2 py-1 text-[11px] font-bold text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {t("totpSetupBanner")}
                </Link>
              ) : null}
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="min-h-[44px]"
                onClick={() => void logout()}
              >
                {t("logout")}
              </Button>
            </div>
          </header>
          <main className="admin-shell-pad flex-1 pb-[calc(56px+env(safe-area-inset-bottom))] lg:pb-6">
            {children}
          </main>
        </div>
        <AdminMobileBar locale={locale} activePath={pathname} />
        <AdminCommandPalette locale={locale} />
      </div>
    </AdminMeProvider>
  );
```

Update the imports at the top of the file: drop `X` from the lucide import (the toggle now only ever shows `Menu`), and add:

```tsx
import { AdminBreadcrumbs } from "@/components/admin/admin-breadcrumbs";
import { AdminMobileBar } from "@/components/admin/admin-mobile-bar";
```

The user identity line that used to sit in the header (`me.display_name … me.roles`) already appears in the sidebar header; do not duplicate it.

- [ ] **Step 8: Verify build and full suite**

Run: `cd frontend && npx vitest run && npm run lint && npm run build`
Expected: all PASS, lint clean, build succeeds.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/components/admin/admin-breadcrumbs.tsx frontend/src/components/admin/admin-mobile-bar.tsx frontend/src/components/admin/admin-mobile-bar.test.tsx "frontend/src/app/[locale]/admin/(shell)/layout.tsx" frontend/messages
git commit -m "feat(admin): theme-correct shell with breadcrumbs and phone bottom bar"
```

---

### Task 5: Migrate content directory tables

Four pages hand-roll `<table>` markup, two of them with the overflow-causing `min-w-[720px]`. Move them onto `AdminDataTable`.

**Files:**
- Modify: `frontend/src/app/[locale]/admin/(shell)/content/questions/page.tsx`
- Modify: `frontend/src/app/[locale]/admin/(shell)/content/explanations/page.tsx`
- Modify: `frontend/src/app/[locale]/admin/(shell)/content/tickets/page.tsx`
- Modify: `frontend/src/app/[locale]/admin/(shell)/content/signs/page.tsx`

**Interfaces:**
- Consumes: `AdminDataTable`, `AdminColumnMeta` from Task 2.
- Produces: nothing new.

**Migration recipe — apply to each page:**

1. Add imports:
   ```tsx
   import { type ColumnDef } from "@tanstack/react-table";
   import { AdminDataTable, type AdminColumnMeta } from "@/components/admin/admin-data-table";
   ```
2. For each `<th>` in the old markup, create one entry in a `useMemo<ColumnDef<RowType>[]>`. The `header` is the same `t("colXxx")` call the `<th>` used. The `cell` is the JSX the matching `<td>` rendered, with `row.original` in place of the map variable.
3. Tag exactly one column `meta: { cardTitle: true }` — the one that identifies the record (the linked title / question text / ticket number / sign name).
4. Tag amount and count columns `meta: { numeric: true }`.
5. Tag any column that is only an action button `meta: { hideOnCard: true }` **only if** the same action is reachable from the row's detail page; otherwise leave it on the card.
6. Delete the whole `<div className="overflow-x-auto …"><table>…</table></div>` block and replace it with:
   ```tsx
   <AdminDataTable
     data={rows}
     columns={columns}
     emptyTitle={t("emptyTitle")}
     emptyDescription={t("emptyDescription")}
     getRowId={(row) => row.id}
   />
   ```
   Reuse whatever empty-state keys the page already has; if it has none, use `t("emptyTitle")` and add that key to all three messages files.
7. Delete any now-unused local empty-state branch that the old markup rendered when the list was empty — `AdminDataTable` owns that.

**Worked example — `content/questions/page.tsx`.** The existing table starts at line 169 with `<table className="w-full min-w-[720px] text-left text-sm">`. Read the current `<thead>`/`<tbody>` in that file and produce:

```tsx
  const columns = useMemo<ColumnDef<QuestionRow>[]>(
    () => [
      {
        accessorKey: "text",
        header: t("colText"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <Link
            href={`/${locale}/admin/content/questions/${row.original.id}`}
            className="font-semibold text-foreground hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {row.original.text}
          </Link>
        ),
      },
      // …one entry per remaining <th>, copying its <td> JSX verbatim
    ],
    [t, locale],
  );
```

- [ ] **Step 1: Migrate `content/questions/page.tsx`**

Apply the recipe. Run: `cd frontend && npm run lint && npx tsc --noEmit`
Expected: clean.

- [ ] **Step 2: Verify no min-width remains on this page**

Run: `cd frontend && grep -n "min-w-\[720px\]" "src/app/[locale]/admin/(shell)/content/questions/page.tsx"`
Expected: no output (exit 1).

- [ ] **Step 3: Migrate `content/explanations/page.tsx`**

Apply the recipe. Run: `cd frontend && npm run lint && npx tsc --noEmit`
Expected: clean.

- [ ] **Step 4: Migrate `content/tickets/page.tsx`**

Apply the recipe. Run: `cd frontend && npm run lint && npx tsc --noEmit`
Expected: clean.

- [ ] **Step 5: Migrate `content/signs/page.tsx`**

Apply the recipe. Run: `cd frontend && npm run lint && npx tsc --noEmit`
Expected: clean.

- [ ] **Step 6: Verify no raw tables remain in content pages**

Run: `cd frontend && grep -rn "<table" "src/app/[locale]/admin/(shell)/content/"`
Expected: no output (exit 1).

- [ ] **Step 7: Run the suite and build**

Run: `cd frontend && npx vitest run && npm run build`
Expected: PASS, build succeeds.

- [ ] **Step 8: Commit**

```bash
git add "frontend/src/app/[locale]/admin/(shell)/content" frontend/messages
git commit -m "refactor(admin): move content directories onto the responsive table"
```

---

### Task 6: Migrate payments tables

**Files:**
- Modify: `frontend/src/app/[locale]/admin/(shell)/payments/transactions/page.tsx`
- Modify: `frontend/src/app/[locale]/admin/(shell)/payments/referral-payouts/page.tsx`

**Interfaces:**
- Consumes: `AdminDataTable`, `AdminColumnMeta` from Task 2.
- Produces: nothing new.

Same recipe as Task 5. Two page-specific rules:

- **`transactions`**: the phone column links to the user; make it `meta: { cardTitle: true }`. `amount_uzs` gets `meta: { numeric: true }`. The delete-action column is rendered conditionally on `canDelete` — keep that condition by building the columns array with a conditional spread so an operator without `payments.delete` never sees the column:
  ```tsx
  const columns = useMemo<ColumnDef<PaymentRow>[]>(
    () => [
      /* …base columns… */
      ...(canDelete
        ? ([
            {
              id: "actions",
              header: t("colActions"),
              cell: ({ row }) => (/* the existing delete button JSX, using row.original */),
            },
          ] as ColumnDef<PaymentRow>[])
        : []),
    ],
    [t, locale, canDelete],
  );
  ```
  Do **not** move the permission decision anywhere else — it must stay exactly the same condition the page uses today.
- **`referral-payouts`**: the payout amount gets `meta: { numeric: true }`; the user identifier gets `meta: { cardTitle: true }`. Approve/reject buttons stay visible on cards (an operator approves payouts from a phone) — do not tag them `hideOnCard`.

- [ ] **Step 1: Migrate `payments/transactions/page.tsx`**

Run: `cd frontend && npm run lint && npx tsc --noEmit`
Expected: clean.

- [ ] **Step 2: Confirm the delete column is still permission-gated**

Run: `cd frontend && grep -n "canDelete" "src/app/[locale]/admin/(shell)/payments/transactions/page.tsx"`
Expected: `canDelete` still appears, guarding the actions column.

- [ ] **Step 3: Migrate `payments/referral-payouts/page.tsx`**

Run: `cd frontend && npm run lint && npx tsc --noEmit`
Expected: clean.

- [ ] **Step 4: Verify no raw tables or min-widths remain**

Run: `cd frontend && grep -rn "<table\|min-w-\[720px\]" "src/app/[locale]/admin/(shell)/payments/"`
Expected: no output (exit 1).

- [ ] **Step 5: Run the suite and build**

Run: `cd frontend && npx vitest run && npm run build`
Expected: PASS, build succeeds.

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/[locale]/admin/(shell)/payments" frontend/messages
git commit -m "refactor(admin): move payment directories onto the responsive table"
```

---

### Task 7: Migrate security, support and user-detail tables

**Files:**
- Modify: `frontend/src/app/[locale]/admin/(shell)/security/rbac/page.tsx`
- Modify: `frontend/src/app/[locale]/admin/(shell)/support/inbox/page.tsx`
- Modify: `frontend/src/app/[locale]/admin/(shell)/users/[id]/page.tsx`

**Interfaces:**
- Consumes: `AdminDataTable`, `AdminColumnMeta` from Task 2.
- Produces: nothing new.

Same recipe as Task 5. Page-specific rules:

- **`security/rbac`**: this page renders the role→permission matrix. A matrix is *not* a directory — a card projection of it is meaningless. Convert its wrapper to `admin-scroll-x` and keep the `<table>`, but delete any `min-w-[720px]` and instead set `min-w-max` so the matrix scrolls horizontally inside its own container without pushing the page. Add `role="region"`, `tabIndex={0}` and an `aria-label` on the scroller so keyboard users can reach it. **Do not** move this page to `AdminDataTable`.
- **`support/inbox`**: full migration. Subject is `cardTitle`; status badge stays; the row links to `support/inbox/[id]`.
- **`users/[id]`**: this file (777 lines) contains one or more small tables inside tabs (sessions, payments, referral ledger). Migrate each to `AdminDataTable` with `maxHeight={320}`. The sessions table's "revoke" button must stay on the card — session revocation is an incident-response action an operator performs from a phone.

- [ ] **Step 1: Fix `security/rbac` matrix scrolling**

Run: `cd frontend && grep -n "min-w-\[720px\]\|admin-scroll-x" "src/app/[locale]/admin/(shell)/security/rbac/page.tsx"`
Expected: `admin-scroll-x` present, `min-w-[720px]` absent.

- [ ] **Step 2: Migrate `support/inbox/page.tsx`**

Run: `cd frontend && npm run lint && npx tsc --noEmit`
Expected: clean.

- [ ] **Step 3: Migrate the tables in `users/[id]/page.tsx`**

Run: `cd frontend && npm run lint && npx tsc --noEmit`
Expected: clean.

- [ ] **Step 4: Verify only the RBAC matrix keeps a raw table**

Run: `cd frontend && grep -rln "<table" "src/app/[locale]/admin/"`
Expected: exactly one path — `src/app/[locale]/admin/(shell)/security/rbac/page.tsx`.

- [ ] **Step 5: Verify no forced min-width survives anywhere in admin**

Run: `cd frontend && grep -rn "min-w-\[720px\]" src/app/\[locale\]/admin src/components/admin`
Expected: no output (exit 1).

- [ ] **Step 6: Run the suite and build**

Run: `cd frontend && npx vitest run && npm run build`
Expected: PASS, build succeeds.

- [ ] **Step 7: Commit**

```bash
git add "frontend/src/app/[locale]/admin/(shell)/security" "frontend/src/app/[locale]/admin/(shell)/support" "frontend/src/app/[locale]/admin/(shell)/users" frontend/messages
git commit -m "refactor(admin): responsive support inbox, user detail tables, scrollable RBAC matrix"
```

---

### Task 8: Overview density — remove dead space, honest charts

The overview (`[[...stub]]/page.tsx`) renders 6 metric tiles then two 14-day bar charts. Screenshots showed the charts rendering a single bar with a large empty region, because a fresh production database has one day of data. SoT §2 density rule: "Overview = 1 screen, ≤6 KPI tiles + 3 charts + alert strip. No card forest."

**Files:**
- Modify: `frontend/src/app/[locale]/admin/(shell)/[[...stub]]/page.tsx`
- Modify: `frontend/src/components/admin/admin-bar-chart.tsx`
- Modify: `frontend/messages/{uz-Latn,uz-Cyrl,ru}.json`

**Interfaces:**
- Consumes: `AdminBarChart` (existing — its props are `points: AdminChartPoint[]`, `emptyLabel: string` (already required), `valueFormatter?`, `height?`; `AdminChartPoint` is `{ label: string; value: number }`), `MetricTile` (existing), `AdminEmptyState` (existing).
- Produces: `AdminBarChart` keeps the same prop signature. Its existing `points.length === 0` guard widens to also cover "fewer than 2 non-zero points", which is the real production case: the API returns 14 days, 13 of them zero, and the chart renders one bar beside 13 ghost bars.

- [ ] **Step 1: Correct the existing `chartEmpty` wording**

The widened guard makes `chartEmpty` also cover the "one populated day" case, so its current text becomes a lie — there *is* data. Reword the existing key rather than adding a new one; this leaves all 6 `AdminBarChart` call sites untouched.

Existing value, in `AdminOverview`, `AdminAnalytics` **and** `AdminInvestors` in every file:

| File | From | To |
|---|---|---|
| `uz-Latn.json` | `Hali ma’lumot yo‘q` | `Grafik uchun ma’lumot hali yetarli emas` |
| `uz-Cyrl.json` | `Ҳали маълумот йўқ` | `График учун маълумот ҳали етарли эмас` |
| `ru.json` | `Пока нет данных` | `Пока недостаточно данных для графика` |

That is 9 edits total (3 namespaces × 3 locales).

- [ ] **Step 2: Write the failing test**

Add these cases inside the existing `describe` block in `frontend/src/components/admin/admin-bar-chart.test.tsx` (keep that file's existing imports and helpers):

```tsx
  it("shows the empty label when only one day carries data", () => {
    const points = Array.from({ length: 14 }, (_, i) => ({
      label: `2026-07-${String(i + 15).padStart(2, "0")}`,
      value: i === 13 ? 3 : 0,
    }));
    render(<AdminBarChart points={points} emptyLabel="Ma’lumot yetarli emas" />);
    expect(screen.getByText("Ma’lumot yetarli emas")).toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("renders the chart once two or more days carry data", () => {
    const points = [
      { label: "2026-07-27", value: 3 },
      { label: "2026-07-28", value: 5 },
    ];
    render(<AdminBarChart points={points} emptyLabel="Ma’lumot yetarli emas" />);
    expect(screen.queryByText("Ma’lumot yetarli emas")).not.toBeInTheDocument();
    expect(screen.getByRole("img")).toBeInTheDocument();
  });
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/components/admin/admin-bar-chart.test.tsx`
Expected: the first new test FAILS — the component currently renders the `<svg role="img">` with 13 ghost bars instead of the empty label.

- [ ] **Step 4: Widen the guard**

In `frontend/src/components/admin/admin-bar-chart.tsx`, replace the existing early return (lines 22–28):

```tsx
  if (!points.length) {
    return (
      <p className="flex h-40 items-center justify-center text-sm text-muted-foreground">
        {emptyLabel}
      </p>
    );
  }
```

with:

```tsx
  // A 14-day series where 13 days are zero is not a chart — it is one bar and
  // a field of ghosts. Say so instead of drawing it.
  const populatedDays = points.filter((p) => p.value > 0).length;
  if (populatedDays < 2) {
    return (
      <p className="flex h-40 items-center justify-center rounded-xl border border-dashed border-border px-4 text-center text-sm text-muted-foreground">
        {emptyLabel}
      </p>
    );
  }
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd frontend && npx vitest run src/components/admin/admin-bar-chart.test.tsx`
Expected: PASS.

- [ ] **Step 6: Tighten the overview layout**

In `[[...stub]]/page.tsx` (presentation only — do not touch the `fetch`, the `PermissionGate`, or the stub-route branch):
- Wrap the tile grid in `grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6` so 6 tiles form one row on desktop and a 2-up grid on phones instead of a tall stack.
- Put the two chart `<section>`s side by side: `grid gap-4 lg:grid-cols-2`.
- Replace the two sections' `rounded-2xl border border-border/80 bg-card/70 p-4` with `admin-panel p-4` so they follow the shared panel token.
- Wrap the page content in `<div className="admin-stack">` and drop any `max-w-*` that leaves the right half of a 1440px screen empty; use `max-w-[1400px]`.
- Leave the `emptyLabel={t("chartEmpty")}` props exactly as they are — Step 1 fixed the wording at the source.

- [ ] **Step 7: Verify**

Run: `cd frontend && npx vitest run && npm run lint && npm run build`
Expected: PASS, clean, build succeeds.

- [ ] **Step 8: Commit**

```bash
git add "frontend/src/app/[locale]/admin/(shell)/[[...stub]]/page.tsx" frontend/src/components/admin/admin-bar-chart.tsx frontend/src/components/admin/admin-bar-chart.test.tsx frontend/messages
git commit -m "feat(admin): denser overview grid and honest chart empty state"
```

---

### Task 9: Visual QA gate — prove no horizontal overflow at 390px

A regression here is invisible to unit tests. This task adds a Playwright check that fails if any admin page scrolls sideways on a phone.

**Files:**
- Create: `frontend/e2e/admin-responsive.spec.ts`
- Modify: `frontend/package.json` (add an `e2e:admin` script if no equivalent exists)

**Interfaces:**
- Consumes: a running dev stack (`ENV=dev OTP_CHANNEL=off` API on `:8090`, web on `:3000`) and a seeded admin (`admin@local` / `localadmin123`).
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Write the spec**

Create `frontend/e2e/admin-responsive.spec.ts`:

```ts
import { expect, test } from "@playwright/test";

const LOCALE = "uz-Latn";
const ROUTES = [
  "",
  "/users",
  "/content/questions",
  "/content/tickets",
  "/content/signs",
  "/content/explanations",
  "/payments/transactions",
  "/payments/manual",
  "/payments/referral-payouts",
  "/support/inbox",
  "/security/audit",
  "/security/rbac",
  "/monitoring/health",
  "/analytics/overview",
];

test.beforeEach(async ({ page }) => {
  await page.goto(`/${LOCALE}/admin/login`);
  await page.getByLabel(/email/i).fill("admin@local");
  await page.getByLabel(/parol|password|пароль/i).fill("localadmin123");
  await page.getByRole("button", { name: /kirish|login|войти/i }).click();
  await page.waitForURL(`**/${LOCALE}/admin**`);
});

for (const route of ROUTES) {
  test(`no horizontal overflow at 390px: ${route || "/overview"}`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/${LOCALE}/admin${route}`);
    await page.waitForLoadState("networkidle");

    const overflow = await page.evaluate(() => {
      const doc = document.documentElement;
      return doc.scrollWidth - doc.clientWidth;
    });
    expect(overflow, `page scrolls ${overflow}px sideways`).toBeLessThanOrEqual(1);
  });

  test(`bottom bar reachable at 390px: ${route || "/overview"}`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/${LOCALE}/admin${route}`);
    await expect(page.getByRole("navigation", { name: /asosiy/i })).toBeVisible();
  });
}
```

- [ ] **Step 2: Run it and watch it fail on any page still overflowing**

Run: `cd frontend && npx playwright test e2e/admin-responsive.spec.ts`
Expected: all tests PASS. If a page fails, fix that page — a failure here means Tasks 5–7 missed a table or a page has its own fixed-width element.

- [ ] **Step 3: Capture the before/after evidence**

Run:
```bash
cd frontend && npx playwright test e2e/admin-responsive.spec.ts --reporter=list
```
Expected: green list. Record the pass count in the commit message.

- [ ] **Step 4: Commit**

```bash
git add frontend/e2e/admin-responsive.spec.ts frontend/package.json
git commit -m "test(admin): gate every admin page against phone horizontal overflow"
```

---

## Wave 1 Done Criteria

- [ ] `grep -rn "min-w-\[720px\]" frontend/src/app/\[locale\]/admin frontend/src/components/admin` returns nothing.
- [ ] `grep -rln "<table" frontend/src/app/\[locale\]/admin` returns only `security/rbac/page.tsx`.
- [ ] No admin file contains a hard-coded `hsl(220 28% …)` colour.
- [ ] `npx vitest run` green; `npm run lint` clean; `npm run build` succeeds.
- [ ] `npx playwright test e2e/admin-responsive.spec.ts` green — 28 tests.
- [ ] Every group in the sidebar collapses; only the active group is open on load.
- [ ] The phone bottom bar reaches Overview, Manual Humo, Users, Health, Inbox.

## Out of scope for Wave 1 (Wave 2 backlog, from the SoT gap analysis)

Refund initiate + entitlement revoke (§4.4) · Webhook inbox and replay (§4.4) · Tariff/promo/limit catalog write (§4.4) · RBAC write — create admin, assign role (§4.8) · Live log tail (§4.1) · IP allowlist (§4.8) · Runtime config, maintenance mode, cache purge (§4.7) · Funnels and async export jobs (§4.9) · Signs/tickets CRUD, media library, import/export, taxonomy (§4.3) · Learning & sessions browser (§2) · Growth section — leaderboard ops, arena, telegram (§2) · Investor entities/documents/contact log (§4.5) · Job pause/resume/requeue and alert ack/snooze (§4.1)
