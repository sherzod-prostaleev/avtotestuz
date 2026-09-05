import { expect, test, type Page } from "@playwright/test";

// Real QR for this exact test referral, generated locally with qrencode.
const qr =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAIQAAACEAQMAAABrihHkAAAABlBMVEUAAAD///+l2Z/dAAAAAnRSTlP//8i138cAAAAJcEhZcwAACxIAAAsSAdLdfvwAAADCSURBVEiJ7dRBEsMgDANA/UD//6V+oI5M03S4FR0bX0I2M2DADrwHHvkK2ABISsygENoBz6ARkKQ5g1Iw3ItFuRU7CTKvjWBOntzvgj9KQnc3HAuM7BSWrrWOhKRgAobciDKjIIuNaPbHTI9GqHV9wmenR4J1amK+FWIq6WkVQiGY4jZzi4UAc1a8Zz6TRPJj+qQQrBk9LVcI8zLPK8NDQUBZ4aqNYzHTb+4Fmr+/G3FQsFKj5wKAeFcTC9kCO/if5QX+BMN0A+wVFQAAAABJRU5ErkJggg==";

const BANNER_NAME = "Telefoningizda ham o‘rganishni davom ettiring";
const enabled = { enabled: true, url: "https://drivergo.uz/r/REF-C62LC2", qr_data_url: qr };

async function stubStation(page: Page, promo: Record<string, unknown> | null) {
  await page.route("**/api/proxy/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const data = path.endsWith("/me/mobile-promo")
      ? (promo ?? { enabled: false, url: "" })
      : path.endsWith("/me")
        ? { profile: { kind: "station", name: "Chilonzor · PC-01" } }
        : [];
    await route.fulfill({ json: { data } });
  });
}

/**
 * A QR big enough for a phone camera, entirely inside the first screen, with
 * no scrolling. This is the requirement in one sentence, and it does not care
 * which of the two copies satisfies it.
 */
async function scannableQROnFirstScreen(page: Page): Promise<boolean> {
  return page.evaluate(() => {
    const viewport = window.innerHeight;
    return Array.from(document.querySelectorAll("img"))
      .filter((img) => img.src.startsWith("data:image/png;base64,"))
      .some((img) => {
        const box = img.getBoundingClientRect();
        return box.top >= 0 && box.bottom <= viewport && box.width >= 64;
      });
  });
}

// The three shapes a classroom PC actually runs. They are why the pinned strip
// exists: the station home screen is ~1200px tall, so the rich banner alone
// began at y=816 and no student who did not scroll ever saw the advert.
for (const viewport of [
  { width: 1366, height: 768 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
]) {
  test(`school mobile QR stays on the classroom screen at ${viewport.width}x${viewport.height}`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize(viewport);
    await stubStation(page, enabled);
    await page.goto("/uz-Latn/station");

    // The regression this test exists for: with no scrolling at all, the QR is
    // on screen, because "permanently at the bottom, like an ad" is the
    // requirement and a student sitting an exam never scrolls the home page.
    const strip = page.getByTestId("mobile-promo-strip");
    await expect(strip).toBeVisible();
    await expect(strip).toHaveCSS("opacity", "1");
    expect(await page.evaluate(() => window.scrollY)).toBe(0);
    expect(await scannableQROnFirstScreen(page)).toBe(true);

    const stripBox = await strip.boundingBox();
    expect(stripBox).not.toBeNull();
    expect(stripBox!.y).toBeGreaterThanOrEqual(0);
    expect(stripBox!.y + stripBox!.height).toBeLessThanOrEqual(viewport.height + 1);
    // A strip taller than a third of the screen would be covering the lesson,
    // not advertising alongside it.
    expect(stripBox!.height).toBeLessThan(viewport.height / 3);
    await expect(strip.locator("img")).toHaveJSProperty("naturalWidth", 132);
    await page.screenshot({ path: testInfo.outputPath(`kiosk-${viewport.width}-first-screen.png`) });

    // The rich banner is still there for anyone who does scroll.
    const banner = page.getByRole("complementary", { name: BANNER_NAME });
    await banner.scrollIntoViewIfNeeded();
    await expect(banner).toBeVisible();
    await expect(banner.getByRole("img")).toHaveJSProperty("naturalWidth", 132);
    // Scrolled onto the banner, the pinned copy steps aside instead of sitting
    // on top of the thing it was advertising.
    await expect(strip).toHaveCSS("opacity", "0");
    // Reserved space: at the very end of the page the banner still clears the
    // strip's row, so the pinned copy can never hide what it advertises.
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    const bannerBox = await banner.boundingBox();
    const stripAtEnd = await strip.boundingBox();
    expect(bannerBox!.y + bannerBox!.height).toBeLessThanOrEqual(stripAtEnd!.y + 1);

    // Back at the top the pinned copy returns; it is not a one-shot banner.
    await page.evaluate(() => window.scrollTo(0, 0));
    await expect(strip).toHaveCSS("opacity", "1");

    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);
  });
}

// No phone or tablet viewport is exercised here on purpose. /station is only
// ever opened by the station agent, in Chromium --app on a classroom PC; the
// same URL on a phone gets kind != "station" from /me and the refusal screen,
// so the advert is unreachable there and a phone-sized assertion would be
// testing a state that cannot happen. The banner's own small-screen styles are
// covered where they do matter -- the admin preview, which operators open on
// their phones (see admin-responsive.spec.ts).

// Switched off, the kiosk has to be exactly the screen it was before this
// feature existed -- no strip, no banner, and no reserved space at the end.
test("school with the advert switched off sees no trace of it", async ({ page }) => {
  await page.setViewportSize({ width: 1366, height: 768 });
  await stubStation(page, null);
  await page.goto("/uz-Latn/station");
  await expect(page.getByRole("heading", { name: "Sinfxona" })).toBeVisible();
  await expect(page.getByTestId("mobile-promo-strip")).toHaveCount(0);
  await expect(page.getByRole("complementary")).toHaveCount(0);
  const offHeight = await page.evaluate(() => document.documentElement.scrollHeight);

  await page.unrouteAll({ behavior: "ignoreErrors" });
  await stubStation(page, enabled);
  await page.reload();
  await expect(page.getByTestId("mobile-promo-strip")).toBeVisible();
  const onHeight = await page.evaluate(() => document.documentElement.scrollHeight);
  expect(onHeight).toBeGreaterThan(offHeight);
});
