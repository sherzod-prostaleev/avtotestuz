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

    await page.goto("/uz-Latn");
    await page.waitForTimeout(600);

    const supported = await page.evaluate(() => typeof (document as any).startViewTransition === "function");
    test.skip(!supported, "browser has no View Transitions API");

    await page.evaluate(() => {
      const link = [...document.querySelectorAll("a")].find((a) =>
        (a.getAttribute("href") || "").includes("/narxlar"),
      ) as HTMLAnchorElement;
      link.click();
    });
    await page.waitForTimeout(1500);

    const result = await page.evaluate(() => ({
      calls: (window as any).__vt.calls,
      snapshot: (window as any).__vt.snapshot,
      path: location.pathname,
      // The enter-only fallback must stay out of the way when the API drives.
      fallbackAnimation: getComputedStyle(document.querySelector(".page-fade")!).animationName,
    }));

    expect(result.path).toContain("/narxlar");
    expect(result.calls).toBe(1);
    expect(result.snapshot).toBe(true);
    expect(result.fallbackAnimation).toBe("none");
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
});
