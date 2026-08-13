import { expect, test, type Page } from "@playwright/test";

const SESSION_ID = "layout-session";
const QUESTION_ID = "layout-question";

const VIEWPORTS = [
  { name: "full-hd", width: 1920, height: 1080 },
  { name: "classroom-laptop", width: 1366, height: 768 },
  { name: "short-desktop", width: 1280, height: 720 },
  { name: "legacy-classroom", width: 1024, height: 768 },
] as const;

interface LayoutScenario {
  name: string;
  question: string;
  imageUrl: string | null;
  answers: string[];
}

const SCENARIOS: LayoutScenario[] = [
  {
    name: "two-options",
    question: "Ushbu vaziyatda haydovchi qaysi yo'nalishda harakatlanishi mumkin?",
    imageUrl: null,
    answers: ["Faqat to'g'ri yo'nalishda", "To'g'ri yoki o'ng tomonga"],
  },
  {
    name: "five-long-options",
    question:
      "Yo'l harakati xavfsizligini saqlash uchun ushbu murakkab chorrahaga yaqinlashayotgan haydovchi qanday yo'l tutishi kerak?",
    imageUrl: null,
    answers: [
      "Tezlikni oldindan kamaytirib, yo'l belgilari va svetofor ishoralariga qat'iy rioya qilishi kerak",
      "Faqat qarama-qarshi yo'nalishdagi transport vositalari to'liq o'tib ketganidan keyin harakatlanishi mumkin",
      "Piyodalar o'tish joyiga yaqinlashganda vaziyatni baholab, zarur bo'lsa ularga yo'l berishi kerak",
      "Qo'shni bo'lakdagi transport vositalarining harakatiga xalaqit bermasdan xavfsiz masofani saqlashi kerak",
      "Barcha ko'rsatilgan xavfsizlik choralarini vaziyatga mos ravishda ketma-ket bajarishi kerak",
    ],
  },
  {
    name: "question-image",
    question: "Rasmda ko'rsatilgan yo'l vaziyatida qaysi transport vositasi birinchi o'tadi?",
    imageUrl: "/exam/placeholder-driver-go-cars.png",
    answers: ["Yengil avtomobil", "Avtobus", "Mototsikl"],
  },
];

async function mockSession(page: Page, scenario: LayoutScenario) {
  await page.route("**/api/proxy/me/saved", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: [] }) })
  );
  await page.route(`**/api/proxy/sessions/${SESSION_ID}**`, (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith(`/sessions/${SESSION_ID}`)) {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            id: SESSION_ID,
            mode: "practice",
            total: 1,
            status: "in_progress",
            stopped_reason: "",
            time_limit_sec: null,
            started_at: "2026-08-08T00:00:00Z",
            answers: [{ question_id: QUESTION_ID, position: 1, answered: false }],
          },
        }),
      });
    }

    if (url.pathname.endsWith(`/sessions/${SESSION_ID}/questions`) || url.pathname.endsWith(`/sessions/${SESSION_ID}/questions/${QUESTION_ID}`)) {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: url.pathname.endsWith(`/sessions/${SESSION_ID}/questions`)
            ? [
                {
                  id: QUESTION_ID,
                  category_code: "layout",
                  text: scenario.question,
                  image_url: scenario.imageUrl,
                  answers: scenario.answers.map((text, index) => ({
                    id: `answer-${index + 1}`,
                    position: index + 1,
                    text,
                    image_url: null,
                  })),
                  signs: [],
                  explanation: null,
                  position: 1,
                  answered: false,
                  user_answer_id: null,
                },
              ]
            : {
                id: QUESTION_ID,
                category_code: "layout",
                text: scenario.question,
                image_url: scenario.imageUrl,
                answers: scenario.answers.map((text, index) => ({
                  id: `answer-${index + 1}`,
                  position: index + 1,
                  text,
                  image_url: null,
                })),
                signs: [],
                explanation: null,
                position: 1,
                answered: false,
                user_answer_id: null,
              },
        }),
      });
    }

    return route.abort("failed");
  });
}

async function layoutMetrics(page: Page) {
  return page.evaluate(() => {
    const shell = document.querySelector<HTMLElement>(".session-shell");
    const stage = document.querySelector<HTMLElement>("[data-testid='question-stage']");
    const stem = stage?.querySelector<HTMLElement>("h1");
    const list = document.querySelector<HTMLElement>("[data-testid='answer-list']");
    const footer = document.querySelector<HTMLElement>(".session-actions");
    const options = Array.from(list?.querySelectorAll<HTMLElement>("[data-answer-option]") ?? []);
    if (!shell || !stage || !stem || !list || !footer) throw new Error("session layout is incomplete");

    const shellRect = shell.getBoundingClientRect();
    const stageRect = stage.getBoundingClientRect();
    const stemRect = stem.getBoundingClientRect();
    const listRect = list.getBoundingClientRect();
    const footerRect = footer.getBoundingClientRect();
    const pageHeight = Math.max(document.documentElement.scrollHeight, document.body.scrollHeight);

    return {
      pageOverflow: pageHeight - window.innerHeight,
      shellTop: shellRect.top,
      shellBottom: shellRect.bottom,
      stageTop: stageRect.top,
      stageBottom: stageRect.bottom,
      stemTop: stemRect.top,
      stemBottom: stemRect.bottom,
      footerTop: footerRect.top,
      footerBottom: footerRect.bottom,
      listOverflow: list.scrollHeight - list.clientHeight,
      listOverflowY: getComputedStyle(list).overflowY,
      allOptionsVisible: options.every((option) => {
        const rect = option.getBoundingClientRect();
        return rect.top >= listRect.top - 1 && rect.bottom <= listRect.bottom + 1;
      }),
      optionCount: options.length,
    };
  });
}

test.describe("practice session viewport fit", () => {
  for (const viewport of VIEWPORTS) {
    for (const scenario of SCENARIOS) {
      test(`${viewport.width}x${viewport.height} keeps ${scenario.name} in one screen`, async ({ page }) => {
        await page.setViewportSize({ width: viewport.width, height: viewport.height });
        await mockSession(page, scenario);
        await page.goto(`/uz-Latn/station/session/${SESSION_ID}`);
        await expect(page.getByTestId("question-stage")).toBeVisible();

        const metrics = await layoutMetrics(page);
        await test.info().attach("layout-metrics", {
          body: JSON.stringify({ viewport, scenario: scenario.name, metrics }, null, 2),
          contentType: "application/json",
        });

        expect(metrics.pageOverflow).toBeLessThanOrEqual(1);
        expect(metrics.shellTop).toBeGreaterThanOrEqual(-1);
        expect(metrics.shellBottom).toBeLessThanOrEqual(viewport.height + 1);
        expect(metrics.stemTop).toBeGreaterThanOrEqual(metrics.stageTop - 1);
        expect(metrics.stemBottom).toBeLessThanOrEqual(metrics.stageBottom + 1);
        expect(metrics.footerBottom).toBeLessThanOrEqual(viewport.height + 1);
        expect(metrics.optionCount).toBe(scenario.answers.length);
        expect(metrics.listOverflow).toBeLessThanOrEqual(1);
        expect(metrics.allOptionsVisible).toBe(true);
      });
    }
  }

  test("genuinely oversized answers scroll only inside their list", async ({ page }) => {
    const viewport = VIEWPORTS[3];
    const repeated =
      "Haydovchi xavfsiz tezlikni tanlashi, yo'l belgilari talablariga rioya qilishi va boshqa qatnashchilarga xalaqit bermasligi shart. ";
    const scenario: LayoutScenario = {
      name: "overflow-stress",
      question: "Juda uzun javobli vaziyatda qaysi talablar bajarilishi kerak?",
      imageUrl: null,
      answers: Array.from({ length: 5 }, (_, index) => `${index + 1}. ${repeated.repeat(4)}`),
    };

    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    await mockSession(page, scenario);
    await page.goto(`/uz-Latn/station/session/${SESSION_ID}`);
    await expect(page.getByTestId("question-stage")).toBeVisible();

    const metrics = await layoutMetrics(page);
    expect(metrics.pageOverflow).toBeLessThanOrEqual(1);
    expect(metrics.shellBottom).toBeLessThanOrEqual(viewport.height + 1);
    expect(metrics.stemBottom).toBeLessThanOrEqual(metrics.stageBottom + 1);
    expect(metrics.footerBottom).toBeLessThanOrEqual(viewport.height + 1);
    expect(metrics.listOverflowY).toBe("auto");
    expect(metrics.listOverflow).toBeGreaterThan(1);
    expect(metrics.allOptionsVisible).toBe(false);
  });
});
