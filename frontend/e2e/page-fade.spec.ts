import { test, expect } from "@playwright/test";

// This transition has broken silently more than once — a Suspense fallback made
// the wrapper mount around the spinner instead of the content, and a fill-mode
// left the page able to rest at opacity 0. Asserting DOM shape missed both, so
// these watch what the browser actually does.
test.describe("Page transition", () => {
  test("cross-fades: the outgoing page is held while the new one arrives", async ({ page }) => {
    // view-transitions.tsx binds document.startViewTransition when it mounts,
    // so the probe has to be installed before any app code runs.
    await page.addInitScript(() => {
      const w = window as any;
      w.__vt = { calls: 0, snapshot: false };
      const doc = document as any;
      const original = doc.startViewTransition;
      if (!original) return;
      doc.startViewTransition = function (cb: any) {
        w.__vt.calls++;
        const transition = original.call(this, cb);
        // `ready` settles only once the browser has captured the outgoing page
        // and built the ::view-transition tree — a real cross-fade rather than
        // a swap. It rejects if the transition is skipped.
        transition.ready.then(
          () => {
            w.__vt.snapshot = true;
          },
          () => {},
        );
        return transition;
      };
    });

    // `next dev` disables prefetching, so a first click would always outrun
    // the freeze limit and be skipped by design. Visit the route once through
    // the client router instead: staleTimes keeps it in the router cache, so
    // the click being measured commits from memory the way a prefetched route
    // does in production.
    await page.goto("/uz-Latn");
    const link = page.locator('a[href*="/narxlar"]').first();
    // Prime the router cache through a real navigation — `next dev` disables
    // prefetch — then hover long enough that the click counts the route warm.
    await link.click();
    await expect(page).toHaveURL(/narxlar/);
    await page.goBack();
    await expect(page).toHaveURL(/\/uz-Latn$/);
    await page.waitForTimeout(400);
    await link.hover();
    await page.waitForTimeout(600);

    const supported = await page.evaluate(
      () => typeof (document as any).startViewTransition === "function",
    );
    test.skip(!supported, "browser has no View Transitions API");

    await page.evaluate(() => {
      (window as any).__vt.calls = 0;
      (window as any).__vt.snapshot = false;
    });

    await link.click();
    await page.waitForTimeout(1500);

    const result = await page.evaluate(() => ({
      calls: (window as any).__vt.calls,
      snapshot: (window as any).__vt.snapshot,
      path: location.pathname,
      // While a real cross-fade runs the document is stamped, which stands the
      // enter-fade down so the two cannot stack.
      stamped: document.documentElement.hasAttribute("data-view-transition"),
    }));

    expect(result.path).toContain("/narxlar");
    expect(result.calls).toBe(1);
    expect(result.snapshot).toBe(true);
    expect(result.stamped).toBe(false); // cleared once the transition finished
  });

  test("still navigates when the API is missing, and never leaves a blank page", async ({ page }) => {
    await page.addInitScript(() => {
      delete (Document.prototype as any).startViewTransition;
    });
    await page.goto("/uz-Latn");

    // No fill-mode: a page whose animation never runs still rests at opacity 1.
    const fillMode = await page.evaluate(
      () => getComputedStyle(document.querySelector(".page-fade")!).animationFillMode,
    );
    expect(fillMode).toBe("none");

    await page.getByRole("link", { name: /narxlar|тариф/i }).first().click();
    await expect(page).toHaveURL(/narxlar/);
    await expect(page.locator("h1").first()).toBeVisible();
  });

  test("drops the snapshot instead of freezing when a route is slow", async ({ page }) => {
    // Hold the RSC payload back so the route cannot commit inside the freeze
    // limit. The transition must give up rather than leave the page locked.
    await page.route("**/*_rsc=*", async (route) => {
      await new Promise((r) => setTimeout(r, 800));
      await route.continue();
    });

    await page.addInitScript(() => {
      const w = window as any;
      w.__vt = { skipped: false };
      const doc = document as any;
      const original = doc.startViewTransition;
      if (!original) return;
      doc.startViewTransition = function (cb: any) {
        const t = original.call(this, cb);
        // `ready` rejects when a transition is skipped before it can animate.
        t.ready.then(
          () => {},
          () => {
            w.__vt.skipped = true;
          },
        );
        return t;
      };
    });

    await page.goto("/uz-Latn");
    const link = page.locator('a[href*="/narxlar"]').first();
    await link.hover();
    await page.waitForTimeout(600);
    await link.click();

    // The page still gets there, and it was never held hostage by the snapshot.
    await expect(page).toHaveURL(/narxlar/, { timeout: 10_000 });
    await expect(page.locator("h1").first()).toBeVisible();
    expect(await page.evaluate(() => (window as any).__vt.skipped)).toBe(true);

    // A skipped transition must still leave the incoming page with its own
    // fade — otherwise a slow route means no animation at all.
    const fade = await page.evaluate(() => {
      const el = document.querySelector(".page-fade")!;
      return {
        name: getComputedStyle(el).animationName,
        stamped: document.documentElement.hasAttribute("data-view-transition"),
      };
    });
    expect(fade.stamped).toBe(false);
    expect(fade.name).toBe("page-fade-in");
  });
});