import { expect, test, type Page } from "@playwright/test";

/**
 * The learner's Telegram photo in the avatar spots (phone menu header, phone
 * profile header, wide sidebar footer), and the initial letter it falls back
 * to. No backend: /me is stubbed, the photo is served by a route.
 *
 * Set AVATAR_SCREENSHOT_DIR to keep a screenshot of every state for review.
 */

const PHOTO_PATH = "/media/images/avatars/abcdefghijklmnopqrstuvwxyz.jpg";

// Stands in for the 256px JPEG: something face-like that is obviously not a letter.
const PHOTO_SVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256">
  <defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="#5eead4"/><stop offset="1" stop-color="#2563eb"/></linearGradient></defs>
  <rect width="256" height="256" fill="url(#g)"/>
  <circle cx="128" cy="104" r="52" fill="#fde7c8"/>
  <rect x="52" y="168" width="152" height="120" rx="60" fill="#fde7c8"/>
</svg>`;

function me(avatar: boolean) {
  return {
    profile: {
      id: "e2e-user",
      phone: "+998901234567",
      name: "Zarina",
      region: "Toshkent",
      district: "Chilonzor",
      birth_date: null,
      locale_pref: "uz-Latn",
      theme_pref: "dark",
      referral_code: "E2E123",
      role: "user",
      kind: "user",
      created_at: "2026-01-01T00:00:00Z",
      ...(avatar ? { avatar_url: `http://localhost:${process.env.PORT || 3000}${PHOTO_PATH}` } : {}),
    },
    vip: { active: false, until: null },
  };
}

function json(body: unknown) {
  return { status: 200, contentType: "application/json", body: JSON.stringify({ data: body }) };
}

async function setup(page: Page, opts: { avatar: boolean; photoStatus?: number; theme: "light" | "dark" }) {
  await page.context().addCookies([
    { name: "at", value: "e2e-stub-not-a-real-token", url: test.info().project.use.baseURL ?? "http://localhost:3000", httpOnly: true, sameSite: "Lax" },
  ]);
  await page.addInitScript((theme) => {
    try {
      localStorage.setItem("theme", theme);
    } catch {
      /* storage blocked: the default theme is fine */
    }
  }, opts.theme);
  await page.route("**/api/proxy/**", (route) => {
    const path = new URL(route.request().url()).pathname.replace("/api/proxy/", "");
    if (path === "me") return route.fulfill(json(me(opts.avatar)));
    if (path === "me/entitlement") return route.fulfill(json({ active: false, until: null }));
    if (path === "me/telegram") return route.fulfill(json({ linked: true, phone_verified: true, tg_user_id: 1 }));
    // The sidebar shows the learner only once its stats query has landed.
    if (path === "me/streak") {
      return route.fulfill(json({ current: 3, best: 5, today_done: 4, daily_goal: 20, last_active_date: null }));
    }
    if (path === "me/stats") return route.fulfill(json({ categories: [], readiness_pct: 0, due_count: 0 }));
    if (path.startsWith("categories")) return route.fulfill(json([]));
    return route.fulfill(json(null));
  });
  await page.route(`**${PHOTO_PATH}`, (route) =>
    opts.photoStatus
      ? route.fulfill({ status: opts.photoStatus, body: "" })
      : route.fulfill({ status: 200, contentType: "image/svg+xml", body: PHOTO_SVG }),
  );
}

/** The phone menu slides in over 300 ms; wait until it is fully open. */
async function menuOpen(page: Page) {
  await page.getByRole("navigation", { name: "Driver Go" }).getByRole("button", { name: "Menyuni ochish" }).click();
  await expect
    .poll(() => page.getByRole("dialog").evaluate((el) => Math.round(el.getBoundingClientRect().left)))
    .toBe(0);
}

async function shot(page: Page, name: string) {
  const dir = process.env.AVATAR_SCREENSHOT_DIR;
  if (dir) await page.screenshot({ path: `${dir}/${name}.png` });
}

/** The photo in scope that is actually on screen (the drawer holds a hidden wide copy and vice versa). */
function photoIn(page: Page, scope: string) {
  return page.locator(`${scope} img[src$="${PHOTO_PATH}"]`).filter({ visible: true });
}

async function expectLoaded(page: Page, scope: string) {
  const img = photoIn(page, scope).first();
  await expect(img).toBeVisible();
  await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth > 0)).toBe(true);
}

/**
 * The avatar box (the img's parent) must be exactly the size the letter
 * bubble always was — h-11/w-11 on phones, h-10/w-10 in the wide footer —
 * measured against the root font size so the check holds at any rem.
 */
async function expectBoxRem(page: Page, scope: string, rem: number) {
  const box = await photoIn(page, scope).first().locator("xpath=..").boundingBox();
  const px = await page.evaluate((r) => r * parseFloat(getComputedStyle(document.documentElement).fontSize), rem);
  expect(box, "avatar box").not.toBeNull();
  expect(Math.abs(box!.width - px)).toBeLessThan(0.5);
  expect(Math.abs(box!.height - px)).toBeLessThan(0.5);
}

for (const theme of ["light", "dark"] as const) {
  test.describe(`learner avatar (${theme})`, () => {
    test.use({ viewport: { width: 390, height: 844 } });

    test("phone menu and profile show the Telegram photo", async ({ page }) => {
      await setup(page, { avatar: true, theme });
      await page.goto("/uz-Latn/profile");
      await expectLoaded(page, "main");
      await expectBoxRem(page, "main", 2.75);
      await shot(page, `profile-mobile-photo-${theme}`);

      await menuOpen(page);
      await expectLoaded(page, "aside");
      await expectBoxRem(page, "aside", 2.75);
      await shot(page, `drawer-photo-${theme}`);
    });

    test("without a photo the initial stays", async ({ page }) => {
      await setup(page, { avatar: false, theme });
      await page.goto("/uz-Latn/profile");
      await expect(page.locator("main").getByText("Zarina").first()).toBeVisible();
      await expect(page.locator(`img[src*="/media/images/avatars/"]`)).toHaveCount(0);
      await expect(page.locator("main").getByText("Z", { exact: true })).toBeVisible();
      await shot(page, `profile-mobile-initial-${theme}`);
      await menuOpen(page);
      await expect(page.locator("aside").getByText("Z", { exact: true }).first()).toBeVisible();
      await shot(page, `drawer-initial-${theme}`);
    });
  });
}

test("a photo that fails to load falls back to the initial", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await setup(page, { avatar: true, photoStatus: 404, theme: "light" });
  await page.goto("/uz-Latn/profile");
  await expect(page.locator("main").getByText("Z", { exact: true })).toBeVisible();
  await expect(photoIn(page, "main")).toHaveCount(0);
});

test("wide layout: sidebar footer shows the photo in the same box", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  for (const theme of ["light", "dark"] as const) {
    await setup(page, { avatar: true, theme });
    await page.goto("/uz-Latn/profile");
    await expectLoaded(page, "aside");
    await expectBoxRem(page, "aside", 2.5);
    await shot(page, `desktop-profile-photo-${theme}`);
  }
  await page.unrouteAll({ behavior: "ignoreErrors" });
  await setup(page, { avatar: false, theme: "light" });
  await page.goto("/uz-Latn/profile");
  await expect(page.locator("aside").getByText("Z", { exact: true }).filter({ visible: true })).toBeVisible();
  await expect(page.locator(`aside img[src*="/media/images/avatars/"]`)).toHaveCount(0);
  await shot(page, "desktop-profile-initial-light");
});
