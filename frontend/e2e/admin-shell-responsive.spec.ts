import { expect, test, type Page } from "@playwright/test";
import { ADMIN_ROW_FIXTURES } from "./fixtures/admin-rows";

/**
 * Stack-free layout gate for the admin panel.
 *
 * `admin-responsive.spec.ts` logs in for real, but it needs a live backend and
 * a seeded admin, so it only runs when someone opts in — a poor regression net
 * for something that breaks silently on every route.
 *
 * This spec fulfils the admin API at the network boundary instead, so it runs
 * anywhere the frontend does, with no database and no credentials. It feeds
 * the directories deliberately hostile rows (see fixtures/admin-rows.ts) and
 * asserts one of those rows is on screen before measuring, so a wrong-shaped
 * fixture cannot quietly reduce the test to measuring an empty table.
 *
 * Two viewports, because the failure differs:
 *   390px  — nothing may push the document sideways.
 *   1280px — the directories are virtualized tables here, so every header must
 *            still sit above the column it names.
 */

const LOCALE = "uz-Latn";
const IPHONE_12 = { width: 390, height: 844 };

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

/**
 * Routes fed hostile rows, and a string from those rows that must be on
 * screen before the measurement is worth anything.
 */
const POPULATED: Record<string, string> = {
  "/users": "Musharrafxon",
  "/content/questions": "AVTOTEST-2026-BILET",
  "/content/tickets": "20",
  "/content/signs": "axborot",
  "/content/explanations": "AVTOTEST-2026-BILET",
  "/payments/transactions": "PAYME-TXN",
  "/payments/referral-payouts": "Musharrafxon",
  "/support/inbox": "premium ochilmadi",
};

/** A superadmin, so no nav group is hidden and the chrome is at its widest. */
const ME = {
  id: "00000000-0000-0000-0000-000000000001",
  email: "e2e-admin@example.com",
  display_name: "E2E Superadmin",
  roles: ["superadmin"],
  permissions: [
    "monitoring.read",
    "analytics.read",
    "investors.read",
    "users.read",
    "content.questions.read",
    "payments.read",
    "payments.delete",
    "referral.read",
    "referral.payouts.manage",
    "cms.read",
    "settings.flags",
    "settings.config",
    "security.audit.read",
    "security.rbac",
    "support.inbox",
    "support.broadcast",
  ],
  totp_enabled: true,
  totp_setup_required: false,
};

/**
 * One permissive envelope. Admin pages read `data`, and defensively coalesce
 * the collection shapes (`items ?? []`), so a single object satisfies the list
 * pages, the detail pages and the metric pages alike. Pages render their empty
 * state, which is exactly the state whose chrome we want measured.
 */
const ENVELOPE = {
  data: {
    items: [],
    roles: [],
    permissions: [],
    assignments: [],
    // Series and breakdowns are read directly by the analytics and overview
    // pages; the real API always emits [] for these, so the stub must too.
    top_event_names_7d: [],
    signups_by_day_14d: [],
    revenue_by_day_14d: [],
    checks: [],
    alerts: [],
    jobs: [],
    generated_at: "2026-07-29T00:00:00Z",
    total: 0,
    page: 1,
    limit: 20,
  },
};

/**
 * Middleware gates `/admin/*` on the presence of the admin cookie alone and
 * redirects to the login page without it, so the stub has to get past that
 * before any of the page's own fetches happen. The value is never validated
 * here — the API is stubbed — so a placeholder is enough.
 */
async function seedAdminCookie(page: Page, baseURL: string | undefined): Promise<void> {
  await page.context().addCookies([
    {
      name: "aat",
      value: "e2e-stub-not-a-real-token",
      url: baseURL ?? "http://localhost:3000",
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);
}

async function stubAdminApi(page: Page): Promise<void> {
  await page.route("**/api/admin/me", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: ME }) }),
  );
  await page.route("**/api/admin/**", (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === "/api/admin/me") return route.fallback();

    // A populated table is the only one that can overflow, so the directories
    // get hostile rows; everything else falls back to the empty envelope.
    const rows = ADMIN_ROW_FIXTURES[url.pathname];
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(rows ? { data: rows } : ENVELOPE),
    });
  });
}

/** Names the elements sticking out, so a failure points at a culprit. */
async function measureOverflow(page: Page): Promise<{ overflow: number; widest: unknown }> {
  return page.evaluate(() => {
    const doc = document.documentElement;
    const overflow = doc.scrollWidth - doc.clientWidth;
    const limit = doc.clientWidth;
    const widest = Array.from(document.querySelectorAll<HTMLElement>("body *"))
      .map((el) => ({
        tag: el.tagName.toLowerCase(),
        cls: el.className?.toString().slice(0, 80) ?? "",
        right: Math.round(el.getBoundingClientRect().right),
      }))
      .filter((e) => e.right > limit + 1)
      .sort((a, b) => b.right - a.right)
      .slice(0, 5);
    return { overflow, widest };
  });
}

test.describe("admin chrome at 390px", () => {
  test.beforeEach(async ({ page, baseURL }) => {
    await page.setViewportSize(IPHONE_12);
    await seedAdminCookie(page, baseURL);
    await stubAdminApi(page);
  });

  for (const route of ROUTES) {
    const name = route || "/overview";

    test(`does not scroll sideways: ${name}`, async ({ page }) => {
      await page.goto(`/${LOCALE}/admin${route}`);
      await expect(page.getByRole("navigation", { name: "Asosiy bo‘limlar" })).toBeVisible();

      // Prove the fixture actually landed. A wrong-shaped fixture renders the
      // empty state, and an empty table cannot overflow — the test would pass
      // while measuring nothing.
      const marker = POPULATED[route];
      if (marker) {
        await expect(page.getByText(marker, { exact: false }).first()).toBeVisible();
      }

      const { overflow, widest } = await measureOverflow(page);
      expect(
        overflow,
        `${name} scrolls ${overflow}px sideways. Widest offenders: ${JSON.stringify(widest)}`,
      ).toBeLessThanOrEqual(1);
    });
  }

  test("the nav drawer covers the thumb bar while it is open", async ({ page }) => {
    await page.setViewportSize(IPHONE_12);
    await page.goto(`/${LOCALE}/admin`);
    const bar = page.getByRole("navigation", { name: "Asosiy bo‘limlar" });
    await expect(bar).toBeVisible();

    await page.getByRole("button", { name: "Menyuni ochish" }).click();

    // The scrim must sit above the bar: at an equal z-index the bar wins on
    // document order and stays tappable behind the "closed" drawer.
    const barIsOnTop = await page.evaluate(() => {
      const nav = document.querySelector('nav[aria-label="Asosiy bo‘limlar"]');
      if (!nav) return null;
      const r = nav.getBoundingClientRect();
      const hit = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
      return nav.contains(hit);
    });
    expect(barIsOnTop, "the bottom bar is still hit-testable while the drawer is open").toBe(false);
  });
});

/**
 * Desktop counterpart. The directories are virtualized tables here, not cards,
 * so this is where the column-alignment fix has to hold: rows must stay in
 * normal table flow, or each row sizes its own columns and the sticky header
 * stops sitting above the values it names.
 */
test.describe("admin directories at 1280px", () => {
  test.beforeEach(async ({ page, baseURL }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await seedAdminCookie(page, baseURL);
    await stubAdminApi(page);
  });

  for (const route of Object.keys(POPULATED)) {
    test(`header cells sit above their column: ${route}`, async ({ page }) => {
      await page.goto(`/${LOCALE}/admin${route}`);
      await expect(page.getByText(POPULATED[route], { exact: false }).first()).toBeVisible();

      const drift = await page.evaluate(() => {
        const table = document.querySelector("table");
        if (!table) return null;
        const heads = Array.from(table.querySelectorAll("thead th"));
        const firstRow = table.querySelector("tbody tr[data-index]");
        if (!firstRow || heads.length === 0) return null;
        const cells = Array.from(firstRow.querySelectorAll("td"));
        if (cells.length !== heads.length) return { mismatch: [cells.length, heads.length] };
        return heads.map((h, i) =>
          Math.round(Math.abs(h.getBoundingClientRect().left - cells[i].getBoundingClientRect().left)),
        );
      });

      // Pages without a virtualized table (none today) simply skip the check.
      if (drift === null) return;
      expect(Array.isArray(drift), `column count differs from header count: ${JSON.stringify(drift)}`).toBe(true);
      for (const d of drift as number[]) {
        expect(d, `a header is ${d}px away from its column: ${JSON.stringify(drift)}`).toBeLessThanOrEqual(1);
      }
    });
  }
});
