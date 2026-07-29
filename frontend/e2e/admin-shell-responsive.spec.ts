import { expect, test, type Page } from "@playwright/test";

/**
 * Stack-free phone gate for the admin *chrome*.
 *
 * `admin-responsive.spec.ts` is the authoritative gate, but it needs a live
 * backend and a seeded admin, so it only runs when someone opts in. That
 * makes it a poor regression net for the shell itself — the sidebar drawer,
 * the header, the breadcrumb trail and the thumb-zone bar, which are on every
 * admin route and are where a 390px layout most easily breaks.
 *
 * This spec fulfils the admin API at the network boundary instead, so it runs
 * anywhere the frontend runs, with no database and no credentials. It proves
 * the chrome fits a phone on every route. It deliberately does NOT prove that
 * a table full of production rows fits — that is the other spec's job.
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
    if (route.request().url().includes("/api/admin/me")) return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(ENVELOPE),
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

      const { overflow, widest } = await measureOverflow(page);
      expect(
        overflow,
        `${name} scrolls ${overflow}px sideways. Widest offenders: ${JSON.stringify(widest)}`,
      ).toBeLessThanOrEqual(1);
    });
  }

  test("the nav drawer covers the thumb bar while it is open", async ({ page }) => {
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
