import { expect, test, type Page } from "@playwright/test";

/**
 * Leaving a session must land the learner back on the hub they opened it from.
 *
 * Picking bilet 12 out of the ticket list and pressing Chiqish used to drop
 * you on the home screen, losing your place in the list — the same on a
 * classroom kiosk, where it was worse: a walk-up student had no history to
 * find their way back with. The origin is recorded by a tracker mounted in
 * the app and kiosk shells, so only a run through the real shells proves it;
 * a component test renders the session screen with no shell around it.
 */

const SESSION_ID = "exit-origin-session";
const QUESTION_ID = "exit-origin-question";

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
  vip: { active: true, until: "2027-01-01T00:00:00Z" },
};

const QUESTION = {
  id: QUESTION_ID,
  category_code: "signs",
  text: "Qaysi belgi to'xtashni taqiqlaydi?",
  image_url: null,
  answers: [
    { id: "a-1", position: 1, text: "3.27 belgisi", image_url: null },
    { id: "a-2", position: 2, text: "3.28 belgisi", image_url: null },
  ],
  signs: [],
  explanation: null,
  position: 1,
  answered: false,
  user_answer_id: null,
};

function json(body: unknown) {
  return { status: 200, contentType: "application/json", body: JSON.stringify(body) };
}

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

async function stubApi(page: Page): Promise<void> {
  await page.route("**/api/proxy/**", (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/proxy/", "");

    if (path === "me") return route.fulfill(json({ data: ME }));
    if (path === "me/variants") {
      return route.fulfill(
        json({
          data: Array.from({ length: 12 }, (_, i) => ({
            number: i + 1,
            question_count: 20,
            unlocked: true,
            best_correct: 0,
            attempts: 0,
          })),
        }),
      );
    }
    if (path === "variants") {
      return route.fulfill(json({ data: Array.from({ length: 12 }, (_, i) => ({ number: i + 1, question_count: 20 })) }));
    }
    if (path === "categories") {
      return route.fulfill(
        json({ data: [{ code: "signs", name: "Yo'l belgilari", sort_order: 1, question_count: 10 }] }),
      );
    }
    if (path === "sessions" && route.request().method() === "POST") {
      return route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            id: SESSION_ID,
            mode: "variant",
            question_ids: [QUESTION_ID],
            time_limit_sec: null,
            errors_allowed: null,
            total: 1,
            started_at: "2026-09-09T00:00:00Z",
          },
        }),
      });
    }
    if (path === `sessions/${SESSION_ID}`) {
      return route.fulfill(
        json({
          data: {
            id: SESSION_ID,
            mode: "variant",
            total: 1,
            status: "in_progress",
            stopped_reason: "",
            time_limit_sec: null,
            started_at: "2026-09-09T00:00:00Z",
            answers: [{ question_id: QUESTION_ID, position: 1, answered: false }],
          },
        }),
      );
    }
    if (path === `sessions/${SESSION_ID}/questions`) return route.fulfill(json({ data: [QUESTION] }));
    if (path.startsWith(`sessions/${SESSION_ID}/questions/`)) return route.fulfill(json({ data: QUESTION }));
    if (path.endsWith("/memorize")) {
      return route.fulfill(json({ data: [{ ...QUESTION, answered: true, correct_answer_id: "a-2" }] }));
    }

    return route.fulfill(json({ data: null }));
  });
}

test.describe("session exit returns to its origin", () => {
  test("kiosk: bilet opened from /station/tickets exits back to /station/tickets", async ({ page }) => {
    await stubApi(page);

    await page.goto("/uz-Latn/station/tickets");
    await page.getByRole("button", { name: "12-biletni ochish" }).click();

    await expect(page).toHaveURL(/\/station\/session\/[^/]+$/);
    await expect(page.getByTestId("question-stage")).toBeVisible();

    await page.getByRole("button", { name: "Chiqish" }).click();
    await expect(page).toHaveURL(/\/uz-Latn\/station\/tickets$/);
  });

  test("learner: bilet opened from /tickets exits back to /tickets", async ({ page, baseURL }) => {
    await seedSession(page, baseURL);
    await stubApi(page);

    await page.goto("/uz-Latn/tickets");
    await page.getByRole("button", { name: "12-biletni ochish" }).click();

    await expect(page).toHaveURL(/\/uz-Latn\/session\/[^/]+$/);
    await expect(page.getByTestId("question-stage")).toBeVisible();

    await page.getByRole("button", { name: "Chiqish" }).click();
    await expect(page).toHaveURL(/\/uz-Latn\/tickets$/);
  });

  test("learner: Yodlash opened from /practice exits back to /practice", async ({ page, baseURL }) => {
    await seedSession(page, baseURL);
    await stubApi(page);

    await page.goto("/uz-Latn/practice");
    await page.getByRole("button", { name: "Yodlash", exact: true }).first().click();

    await expect(page).toHaveURL(/\/practice\/memorize\//);
    await expect(page.getByTestId("question-stage")).toBeVisible();

    await page.getByRole("button", { name: "Chiqish" }).click();
    await expect(page).toHaveURL(/\/uz-Latn\/practice$/);
  });

  // A reload lands straight on a session URL with no hub behind it; exit has
  // to stay useful rather than dead-end.
  test("kiosk: a session opened cold still exits to the station home", async ({ page }) => {
    await stubApi(page);

    await page.goto(`/uz-Latn/station/session/${SESSION_ID}`);
    await expect(page.getByTestId("question-stage")).toBeVisible();

    await page.getByRole("button", { name: "Chiqish" }).click();
    await expect(page).toHaveURL(/\/uz-Latn\/station$/);
  });
});
