import { expect, test, type Page } from "@playwright/test";

/**
 * The "Tozalash" control on the bilet grid, on the three screens it ships to.
 *
 * It is exercised through /station/tickets, which renders the same component
 * as /tickets with kiosk=true and needs no login — so this runs in CI, where
 * there is no backend and no auth token, instead of skipping the way the
 * authenticated /tickets specs do.
 *
 * What is asserted throughout is that the control is ON SCREEN, not merely in
 * the DOM. A previous kiosk banner sat at y=816 on a classroom PC: present,
 * queryable, and invisible to every student who did not scroll.
 */

const CLEAR_WIDE = "Tozalash";
const CLEAR_PHONE = "Ishlangan biletlar natijasini tozalash";

function variant(number: number, played: boolean) {
  return {
    number,
    question_count: 20,
    unlocked: true,
    best_correct: played ? 18 : 0,
    attempts: played ? 2 : 0,
    ...(played ? { completed_at: "2026-07-20T12:00:00Z" } : {}),
  };
}

/** Four bilets, the first two carrying results — so two rows are clearable. */
const playedGrid = [variant(1, true), variant(2, true), variant(3, false), variant(4, false)];
const clearedGrid = playedGrid.map((v) => variant(v.number, false));

/**
 * Stubs the BFF. The grid answers `playedGrid` until the reset is posted and
 * `clearedGrid` afterwards, which is what the real endpoint does and what
 * makes the post-clear assertions mean anything.
 */
async function stubTickets(page: Page) {
  let cleared = false;
  await page.route("**/api/proxy/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    if (path.endsWith("/me/variants/reset")) {
      cleared = true;
      await route.fulfill({ json: { data: { cleared: 2, unlock_ceiling: 3 } } });
      return;
    }
    if (path.endsWith("/me/variants")) {
      await route.fulfill({ json: { data: cleared ? clearedGrid : playedGrid } });
      return;
    }
    if (path.endsWith("/me")) {
      await route.fulfill({ json: { data: { profile: { kind: "station", name: "Chilonzor · PC-01" } } } });
      return;
    }
    await route.fulfill({ json: { data: [] } });
  });
}

/**
 * The kiosk's floating language/theme bar is pinned to the top-right corner.
 * At classroom sizes it sits well clear of the header controls, but at phone
 * width it lands on them — and it is the reason this route is usable without a
 * login at all. The phone rows below stand in for the LEARNER phone layout,
 * which has no such bar (its top bar is `sticky`, so it is in flow and pushes
 * the page down), so it is taken out of the way rather than tested around.
 */
async function hideKioskChrome(page: Page) {
  await page.addStyleTag({ content: '[data-testid="kiosk-chrome"]{display:none !important}' });
}

// A phone, a classroom TV, and a desktop. The 1280x720 row is the one the
// station PCs actually run.
const viewports = [
  { name: "phone", width: 390, height: 844, control: CLEAR_PHONE },
  { name: "classroom TV", width: 1280, height: 720, control: CLEAR_WIDE },
  { name: "desktop", width: 1440, height: 900, control: CLEAR_WIDE },
];

for (const vp of viewports) {
  test(`clear control is on the first screen at ${vp.width}x${vp.height} (${vp.name})`, async ({ page }) => {
    await page.setViewportSize({ width: vp.width, height: vp.height });
    await stubTickets(page);
    await page.goto("/uz-Latn/station/tickets");
    await hideKioskChrome(page);

    const button = page.getByRole("button", { name: vp.control, exact: true });

    await expect(button).toBeVisible();
    // Retried, so it also waits for the grid to arrive: the control is
    // deliberately dead until there is a count it can quote.
    await expect(button).toBeEnabled();
    // Visible is not enough: it has to be reachable without scrolling, which
    // on a kiosk nobody does.
    expect(await page.evaluate(() => window.scrollY)).toBe(0);
    const box = await button.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.y).toBeGreaterThanOrEqual(0);
    expect(box!.y + box!.height).toBeLessThanOrEqual(vp.height);
    // Tap target: the phone twin is a 44px box and must not be shrunk.
    expect(box!.height).toBeGreaterThanOrEqual(40);

    // The page must never scroll sideways because of the extra control.
    const overflows = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1
    );
    expect(overflows).toBe(false);
  });

  test(`confirm dialog fits the screen at ${vp.width}x${vp.height} (${vp.name})`, async ({ page }) => {
    await page.setViewportSize({ width: vp.width, height: vp.height });
    await stubTickets(page);
    await page.goto("/uz-Latn/station/tickets");
    await hideKioskChrome(page);

    const opener = page.getByRole("button", { name: vp.control, exact: true });
    await expect(opener).toBeEnabled();
    await opener.click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText("Natijalarni tozalash")).toBeVisible();
    // Two of the four bilets carry results, and the dialog must say so rather
    // than quote the grid size.
    await expect(dialog.getByText(/^2 ta biletning natijasi o'chiriladi/)).toBeVisible();
    await expect(dialog.getByText(/Ochiq biletlar ochiqligicha/)).toBeVisible();

    // Every part of the dialog is on screen, and both choices are reachable
    // without scrolling it. Asserted with toBeInViewport rather than a single
    // getBoundingClientRect read, because the panel animates in from 16px
    // below its resting place: measured once, on the wrong frame, a bottom
    // sheet that ends up perfectly placed still reads as hanging off-screen.
    await expect(dialog).toBeInViewport({ ratio: 1 });
    await expect(dialog.getByRole("button", { name: "Ha, tozalash" })).toBeInViewport({ ratio: 1 });
    await expect(dialog.getByRole("button", { name: "Bekor qilish" })).toBeInViewport({ ratio: 1 });

    // The way out is what the keyboard and a TV remote land on first.
    await expect(dialog.getByRole("button", { name: "Bekor qilish" })).toBeFocused();
  });
}

// Each tile renders its score twice — a compact body for phones and a richer
// one from md up — and CSS shows exactly one of them. Filtering to the visible
// copy is what makes "the score is on the grid" a claim about the screen.
function visibleScores(page: Page) {
  return page.getByText("18/20").filter({ visible: true });
}

const TV = { width: 1280, height: 720 };

test("escape cancels and leaves every result in place", async ({ page }) => {
  await page.setViewportSize(TV);
  await stubTickets(page);
  await page.goto("/uz-Latn/station/tickets");
  await hideKioskChrome(page);

  const opener = page.getByRole("button", { name: CLEAR_WIDE, exact: true });
  await expect(opener).toBeEnabled();
  await opener.click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();

  // Both scores are still on the grid: nothing was posted.
  await expect(visibleScores(page)).toHaveCount(2);
  await expect(opener).toBeEnabled();
});

test("confirming clears the grid and reports what went", async ({ page }) => {
  await page.setViewportSize(TV);
  await stubTickets(page);
  await page.goto("/uz-Latn/station/tickets");
  await hideKioskChrome(page);

  const opener = page.getByRole("button", { name: CLEAR_WIDE, exact: true });
  await expect(opener).toBeEnabled();
  await expect(visibleScores(page)).toHaveCount(2);

  await opener.click();
  await page.getByRole("dialog").getByRole("button", { name: "Ha, tozalash" }).click();

  await expect(page.getByRole("dialog")).toBeHidden();
  const outcome = page.getByRole("status");
  await expect(outcome).toHaveText(/2 ta biletning natijasi tozalandi/);
  // On the first screen, not merely in the document. A classroom TV is 720px
  // tall and nobody scrolls it; a confirmation below the fold is no
  // confirmation at all.
  await expect(outcome).toBeInViewport({ ratio: 1 });
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
  // The scores are gone from the tiles, and every bilet stayed open.
  await expect(visibleScores(page)).toHaveCount(0);
  await expect(page.getByRole("button", { name: "1-biletni ochish" })).toBeVisible();
  await expect(page.getByRole("button", { name: "4-biletni ochish" })).toBeVisible();

  // Nothing left to clear, so the control retires until there is.
  await expect(opener).toBeDisabled();
});
