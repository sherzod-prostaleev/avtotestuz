import { test, expect } from "@playwright/test";

// The navigation fade has broken silently twice: once because a Suspense
// fallback made the template mount around the spinner instead of the content,
// and once because it was measured in a hidden tab where the document timeline
// is frozen. Asserting the DOM shape is not enough — this watches the opacity
// actually move.
test.describe("Page fade", () => {
  test("animates the incoming page from transparent to opaque", async ({ page }) => {
    await page.goto("/uz-Latn");
    await page.waitForSelector(".page-fade");

    const samples = await page.evaluate(async () => {
      const out: Array<{ opacity: number; node: number }> = [];
      const ids = new WeakMap<Element, number>();
      let next = 0;
      const idOf = (el: Element) => {
        if (!ids.has(el)) ids.set(el, next++);
        return ids.get(el)!;
      };
      idOf(document.querySelector(".page-fade")!);

      const link = [...document.querySelectorAll("a")].find((a) =>
        (a.getAttribute("href") || "").includes("/narxlar"),
      );
      (link as HTMLAnchorElement).click();

      for (let i = 0; i < 60; i++) {
        await new Promise((r) => requestAnimationFrame(r));
        const el = document.querySelector(".page-fade");
        if (el) out.push({ opacity: +getComputedStyle(el).opacity, node: idOf(el) });
      }
      return out;
    });

    // A fresh wrapper for the incoming page, not the one we started on.
    const incoming = Math.max(...samples.map((s) => s.node));
    expect(incoming).toBeGreaterThan(0);

    const onIncoming = samples.filter((s) => s.node === incoming);
    const midFade = onIncoming.filter((s) => s.opacity > 0 && s.opacity < 1);

    // Several frames caught mid-fade: the animation ran, it was not a jump.
    expect(midFade.length).toBeGreaterThan(3);
    // And it settles fully opaque — a page must never be left transparent.
    expect(onIncoming.at(-1)!.opacity).toBe(1);
  });

  test("leaves the page readable when the animation cannot run", async ({ page }) => {
    // No fill-mode, so the resting opacity is 1. Guards against the variant
    // that holds the 0% keyframe and blanks the page on a frozen timeline.
    await page.goto("/uz-Latn");
    const fillMode = await page.evaluate(
      () => getComputedStyle(document.querySelector(".page-fade")!).animationFillMode,
    );
    expect(fillMode).toBe("none");
  });
});
