import { expect, test, type Page } from "@playwright/test";

/**
 * Phone-viewport gate for the admin panel.
 *
 * Unit tests cannot see a layout that overflows sideways — jsdom has no
 * layout. This spec renders each admin route in a real 390px browser and
 * fails if the document scrolls horizontally, which is the failure mode the
 * whole responsive wave exists to prevent.
 *
 * It needs a live backend and a seeded admin, so it is opt-in. Set
 * E2E_ADMIN_EMAIL and E2E_ADMIN_PASSWORD to run it; without them the suite
 * skips, matching the convention in e2e/helpers/auth.ts.
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

const EMAIL = process.env.E2E_ADMIN_EMAIL;
const PASSWORD = process.env.E2E_ADMIN_PASSWORD;

/**
 * This spec types real superadmin credentials into a login form. Pointing it
 * at a deployed environment would put those credentials on the wire against
 * production, so the target host is checked rather than trusted.
 */
function assertLocalTarget(baseURL: string | undefined): void {
  const host = new URL(baseURL ?? "http://localhost:3000").hostname;
  if (host !== "localhost" && host !== "127.0.0.1") {
    throw new Error(
      `admin-responsive.spec.ts refuses to run against ${host}: it submits admin credentials and is for local stacks only.`,
    );
  }
}

async function loginAsAdmin(page: Page): Promise<void> {
  await page.goto(`/${LOCALE}/admin/login`);
  await page.getByLabel("Login", { exact: true }).fill(EMAIL!);
  await page.getByLabel("Parol", { exact: true }).fill(PASSWORD!);
  await page.getByRole("button", { name: "Kirish" }).click();

  // A TOTP-enrolled admin cannot be driven from a spec; fail loudly rather
  // than silently measuring the login page instead of the panel.
  // Promise.race abandons the loser, and the loser here is a 15s timeout that
  // would reject with nothing attached — one unhandled rejection per test,
  // surfaced against whichever test happens to be running when it fires.
  // Both branches swallow their own timeout so only the winner speaks.
  const totp = page.getByPlaceholder("000000");
  const landed = page
    .waitForURL(new RegExp(`/${LOCALE}/admin(?!/login)`), { timeout: 15_000 })
    .then(() => "in" as const)
    .catch(() => "pending" as const);
  const challenged = totp
    .waitFor({ state: "visible", timeout: 15_000 })
    .then(() => "totp" as const)
    .catch(() => "pending" as const);

  const outcome = await Promise.race([landed, challenged]);
  if (outcome === "totp") {
    throw new Error(
      "the seeded admin has TOTP enrolled; this spec needs an admin without TOTP (see `make seed-admin`)",
    );
  }
  if (outcome === "pending") {
    throw new Error("admin login neither landed nor presented a TOTP challenge within 15s");
  }
}

test.describe("admin panel at 390px", () => {
  test.skip(
    !EMAIL || !PASSWORD,
    "set E2E_ADMIN_EMAIL and E2E_ADMIN_PASSWORD to run the admin responsive gate",
  );

  test.beforeEach(async ({ page, baseURL }) => {
    assertLocalTarget(baseURL);
    await page.setViewportSize(IPHONE_12);
    await loginAsAdmin(page);
  });

  for (const route of ROUTES) {
    const name = route || "/overview";

    test(`does not scroll sideways: ${name}`, async ({ page }) => {
      await page.goto(`/${LOCALE}/admin${route}`);
      await page.waitForLoadState("networkidle");

      const overflow = await page.evaluate(() => {
        const doc = document.documentElement;
        return doc.scrollWidth - doc.clientWidth;
      });

      // Report the culprit, not just the number — a bare pixel count sends the
      // next person hunting through the whole page.
      if (overflow > 1) {
        const widest = await page.evaluate(() => {
          const limit = document.documentElement.clientWidth;
          return Array.from(document.querySelectorAll<HTMLElement>("body *"))
            .map((el) => ({
              tag: el.tagName.toLowerCase(),
              cls: el.className?.toString().slice(0, 80) ?? "",
              right: Math.round(el.getBoundingClientRect().right),
            }))
            .filter((e) => e.right > limit + 1)
            .sort((a, b) => b.right - a.right)
            .slice(0, 5);
        });
        expect(
          overflow,
          `${name} scrolls ${overflow}px sideways. Widest offenders: ${JSON.stringify(widest)}`,
        ).toBeLessThanOrEqual(1);
      }

      expect(overflow).toBeLessThanOrEqual(1);
    });

    test(`keeps the thumb-zone nav reachable: ${name}`, async ({ page }) => {
      await page.goto(`/${LOCALE}/admin${route}`);
      await expect(page.getByRole("navigation", { name: "Asosiy bo‘limlar" })).toBeVisible();
    });
  }
});
