import { expect, test, type Page } from "@playwright/test";

/**
 * The two screens money passes through, pinned to real phone viewports.
 *
 * Buyers reported the "Sotib olish" button missing on some phones: the column
 * simply grew past `mobile-fit-screen`, and that box cannot scroll, so the CTA
 * sat under the fixed tab bar with no way to reach it. On a payment screen that
 * is not a layout nit, it is the whole sale. Both screens now pin their button
 * to the bottom of the viewport box and scroll everything above it, and these
 * specs hold that: button above the tab bar, page itself never scrolls, and
 * nothing in the scroller is unreachable.
 *
 * Heights are browser *viewport* heights (address bar already subtracted).
 */
const PHONES = [
  { name: "android-compact", width: 360, height: 600 },
  { name: "iphone-se", width: 375, height: 553 },
  { name: "iphone-13", width: 390, height: 664 },
  { name: "pixel-7", width: 412, height: 730 },
] as const;

/** ru is the longest of the three locales — if it fits, they all do. */
const LOCALES = ["uz-Latn", "ru"] as const;

const ME = {
  profile: {
    id: "e2e-user",
    phone: "998901234567",
    name: "E2E",
    region: "Toshkent",
    district: "Chilonzor",
    birth_date: null,
    locale_pref: "uz-Latn",
    theme_pref: "dark",
    referral_code: "E2E123",
    role: "student",
    created_at: "2026-01-01T00:00:00Z",
  },
  vip: { active: false, until: null },
};

const TARIFFS = [
  {
    code: "nexia",
    days: 7,
    price_uzs: 24900,
    old_price_uzs: 34900,
    price_per_day_uzs: 3557,
    discount_percent: 29,
    badge: null,
    name: "Nexia",
    description: "1 haftalik",
  },
  {
    code: "gentra",
    days: 30,
    price_uzs: 59900,
    old_price_uzs: 99900,
    price_per_day_uzs: 1997,
    discount_percent: 40,
    badge: "popular",
    name: "Gentra",
    description: "1 oylik",
  },
  {
    code: "malibu",
    days: 75,
    price_uzs: 109900,
    old_price_uzs: 199900,
    price_per_day_uzs: 1465,
    discount_percent: 45,
    badge: "best_value",
    name: "Malibu",
    description: "2.5 oylik",
  },
];

const PAYMENT_ID = "11111111-2222-3333-4444-555555555555";

const MANUAL = {
  payment_id: PAYMENT_ID,
  payment_status: "pending",
  amount_uzs: 59937,
  pan_full: "9860246603626754",
  pan_last4: "6754",
  holder_name: "SADULLOYEV SHERZOD",
  network: "humo",
  hold_until: new Date(Date.now() + 9 * 60_000).toISOString(),
  manual_state: "awaiting_transfer",
};

/**
 * The proxy's session gate only checks that the auth cookie exists — the value
 * is never validated here because every API call below is stubbed, so a
 * placeholder gets us onto the page without a backend.
 */
async function seedSession(page: Page, baseURL: string | undefined): Promise<void> {
  await page.context().addCookies([
    {
      name: "at",
      value: "e2e-stub-not-a-real-token",
      url: baseURL ?? "http://localhost:3000",
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);
}

function json(body: unknown) {
  return { status: 200, contentType: "application/json", body: JSON.stringify({ data: body }) };
}

async function stubApi(page: Page, opts: { vipActive?: boolean } = {}): Promise<void> {
  await page.route("**/api/proxy/**", (route) => {
    const path = new URL(route.request().url()).pathname.replace("/api/proxy/", "");
    if (path === "me") return route.fulfill(json(ME));
    if (path === "tariffs") return route.fulfill(json(TARIFFS));
    if (path === "me/entitlement") {
      return route.fulfill(
        json(opts.vipActive ? { active: true, until: "2027-01-01T00:00:00Z" } : { active: false, until: null }),
      );
    }
    if (path === "billing/providers") {
      return route.fulfill(
        json([
          { provider: "manual", enabled: true },
          { provider: "payme", enabled: false },
          { provider: "click", enabled: false },
        ]),
      );
    }
    if (path === "me/checkout") {
      return route.fulfill(json({ payment_id: PAYMENT_ID, checkout_url: "", manual: MANUAL }));
    }
    if (path === `me/payments/${PAYMENT_ID}/manual`) return route.fulfill(json(MANUAL));
    return route.fulfill(json(null));
  });
}

/** Names whatever sticks out below the fold, so a failure points at a culprit. */
async function pageOverflow(page: Page) {
  return page.evaluate(() => {
    const doc = document.documentElement;
    const limit = doc.clientHeight;
    const tallest = Array.from(document.querySelectorAll<HTMLElement>("main *"))
      .map((el) => ({
        tag: el.tagName.toLowerCase(),
        cls: el.className?.toString().slice(0, 60) ?? "",
        bottom: Math.round(el.getBoundingClientRect().bottom),
      }))
      .filter((e) => e.bottom > limit + 1)
      .sort((a, b) => b.bottom - a.bottom)
      .slice(0, 4);
    return { overflow: doc.scrollHeight - doc.clientHeight, tallest };
  });
}

/**
 * The CTA has to clear the fixed bottom tab bar, which owns the last 4.25rem —
 * "inside the viewport" is not the same as "tappable".
 */
async function expectCtaAboveTabBar(page: Page, testId: string, viewportHeight: number) {
  const cta = page.getByTestId(testId);
  await expect(cta).toBeVisible();
  const box = await cta.boundingBox();
  expect(box, `${testId} has no box`).not.toBeNull();
  expect(box!.y, `${testId} starts above the viewport`).toBeGreaterThanOrEqual(0);
  expect(
    Math.round(box!.y + box!.height),
    `${testId} runs under the tab bar`,
  ).toBeLessThanOrEqual(viewportHeight - 68);
}

/**
 * Nothing inside the screen crops its own content.
 *
 * Fitting a screen must never turn into quietly cutting text off: a block that
 * hides its overflow has an automatic minimum size of zero, so a flex column
 * squashes it and the browser silently shaves the bottom off — which is how the
 * card first lost the holder's name and the sum under it. The CTA and the
 * scroll checks both pass right through that, so this is its own assertion.
 */
async function expectNothingClipped(page: Page, scrollerTestId: string) {
  const clipped = await page.evaluate((id) => {
    const root = document.querySelector<HTMLElement>(`[data-testid="${id}"]`);
    if (!root) return [{ cls: "missing scroller", text: id, over: -1 }];
    const out: { cls: string; text: string; over: number }[] = [];
    for (const el of Array.from(root.querySelectorAll<HTMLElement>("*"))) {
      const ownText = Array.from(el.childNodes).some(
        (n) => n.nodeType === Node.TEXT_NODE && (n.textContent ?? "").trim().length > 0,
      );
      if (!ownText) continue;
      // Nearest ancestor that hides vertical overflow. A scroller is not one:
      // what it holds is reachable, and expectScrollerReachable covers it.
      let clip: HTMLElement | null = el.parentElement;
      while (clip && clip !== root) {
        const overflowY = getComputedStyle(clip).overflowY;
        if (overflowY === "hidden" || overflowY === "clip") break;
        clip = clip.parentElement;
      }
      if (!clip || clip === root) continue;
      const text = el.getBoundingClientRect();
      const box = clip.getBoundingClientRect();
      const over = Math.round(Math.max(text.bottom - box.bottom, box.top - text.top));
      if (over > 1) {
        out.push({
          cls: el.className?.toString().slice(0, 50) ?? "",
          text: (el.textContent ?? "").trim().slice(0, 40),
          over,
        });
      }
    }
    return out;
  }, scrollerTestId);
  expect(clipped, `text cropped: ${JSON.stringify(clipped)}`).toEqual([]);
}

/** Everything inside the scroller can be brought into view. */
async function expectScrollerReachable(page: Page, testId: string) {
  const reach = await page.getByTestId(testId).evaluate((el) => {
    el.scrollTop = el.scrollHeight;
    const last = el.lastElementChild as HTMLElement | null;
    return {
      hidden: el.scrollHeight - el.clientHeight - el.scrollTop,
      lastBottom: last ? Math.round(last.getBoundingClientRect().bottom) : 0,
      boxBottom: Math.round(el.getBoundingClientRect().bottom),
    };
  });
  expect(reach.hidden, "scroller cannot reach its own end").toBeLessThanOrEqual(1);
  expect(reach.lastBottom, "last block stays clipped at the end of the scroll").toBeLessThanOrEqual(
    reach.boxBottom + 1,
  );
}

for (const locale of LOCALES) {
  for (const phone of PHONES) {
    test.describe(`premium at ${phone.width}x${phone.height} (${phone.name}, ${locale})`, () => {
      test.beforeEach(async ({ page, baseURL }) => {
        await page.setViewportSize({ width: phone.width, height: phone.height });
        await seedSession(page, baseURL);
        await stubApi(page);
        await page.goto(`/${locale}/premium`);
        await expect(page.getByTestId("premium-buy")).toBeVisible();
      });

      test("keeps the buy button on screen and the page unscrolled", async ({ page }) => {
        await expectCtaAboveTabBar(page, "premium-buy", phone.height);
        const { overflow, tallest } = await pageOverflow(page);
        expect(overflow, `page scrolls by ${overflow}px; culprits: ${JSON.stringify(tallest)}`).toBeLessThanOrEqual(1);
      });

      test("lets the plan list scroll to its end, cropping nothing", async ({ page }) => {
        await expectNothingClipped(page, "premium-scroll");
        await expectScrollerReachable(page, "premium-scroll");
        // The button is pinned, so it is still there after the scroll.
        await expectCtaAboveTabBar(page, "premium-buy", phone.height);
      });
    });
  }
}

test.describe("premium, VIP already active", () => {
  // The banner used to sit above `mobile-fit-screen` and push the CTA out of
  // reach on the shortest phone — the one case a buyer could not scroll past.
  test("keeps the buy button reachable under the VIP banner", async ({ page, baseURL }) => {
    await page.setViewportSize({ width: 375, height: 553 });
    await seedSession(page, baseURL);
    await stubApi(page, { vipActive: true });
    await page.goto("/uz-Latn/premium");
    await expect(page.getByTestId("premium-buy")).toBeVisible();
    await expectCtaAboveTabBar(page, "premium-buy", 553);
    const { overflow } = await pageOverflow(page);
    expect(overflow).toBeLessThanOrEqual(1);
  });
});

test.describe("premium checkout flow", () => {
  test("goes straight from the plan to the card, asking for no promo code", async ({ page, baseURL }) => {
    await page.setViewportSize({ width: 375, height: 553 });
    await seedSession(page, baseURL);
    await stubApi(page);
    await page.goto("/uz-Latn/premium");

    await expect(page.getByText("Matiz")).toHaveCount(0);
    await expect(page.getByPlaceholder(/PROMO/i)).toHaveCount(0);

    await page.getByTestId("premium-buy").click();
    await page.waitForURL(`**/uz-Latn/checkout/manual?payment_id=${PAYMENT_ID}`);
    await expect(page.getByTestId("manual-claim")).toBeVisible();
  });
});

for (const locale of LOCALES) {
  for (const phone of PHONES) {
    test.describe(`card transfer at ${phone.width}x${phone.height} (${phone.name}, ${locale})`, () => {
      test.beforeEach(async ({ page, baseURL }) => {
        await page.setViewportSize({ width: phone.width, height: phone.height });
        await seedSession(page, baseURL);
        await stubApi(page);
        await page.goto(`/${locale}/checkout/manual?payment_id=${PAYMENT_ID}`);
        await expect(page.getByTestId("manual-claim")).toBeVisible();
      });

      test("keeps the claim button on screen and the page unscrolled", async ({ page }) => {
        await expectCtaAboveTabBar(page, "manual-claim", phone.height);
        const { overflow, tallest } = await pageOverflow(page);
        expect(overflow, `page scrolls by ${overflow}px; culprits: ${JSON.stringify(tallest)}`).toBeLessThanOrEqual(1);
      });

      test("shows the card number and both sums without clipping", async ({ page }) => {
        const scroller = page.getByTestId("manual-scroll");
        // The PAN and the exact sum are what a payer copies; they head the
        // scroller, so they are on screen before anything is scrolled.
        await expect(scroller.getByText("9860 2466 0362 6754")).toBeVisible();
        await expect(scroller.getByText(/59 937/).first()).toBeVisible();
        // 59 937 carries a tail, so the sum to warn against is the base price
        // it stepped around — another payer is owed exactly that on this card.
        await expect(scroller.getByText(/59 900/)).toBeVisible();
        await expectNothingClipped(page, "manual-scroll");
        await expectScrollerReachable(page, "manual-scroll");
      });
    });
  }
}
