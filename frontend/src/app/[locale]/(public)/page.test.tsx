import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../../messages/uz-Latn.json";
import uzCyrlMessages from "../../../../messages/uz-Cyrl.json";
import ruMessages from "../../../../messages/ru.json";
import LandingPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/uz-Latn",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}));

vi.mock("@/i18n/navigation", () => ({
  usePathname: () => "/",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

const languageSwitcherLabel = {
  "uz-Latn": "Tilni tanlash",
  "uz-Cyrl": "Тилни танлаш",
  ru: "Выбор языка",
} as const;

const localeCases = [
  {
    locale: "uz-Latn",
    messages: uzLatnMessages,
    hero: "tayyor",
    login: "Kirish",
    feature: "Aqlli takrorlash",
    footer: "O'zbekiston yo'l harakati qoidalari bo'yicha o'quv platformasi",
    question: "O'ngdan keluvchi haydovchi qachon yo'l beradi?",
  },
  {
    locale: "uz-Cyrl",
    messages: uzCyrlMessages,
    hero: "тайёр",
    login: "Кириш",
    feature: "Ақлли такрорлаш",
    footer: "Ўзбекистон йўл ҳаракати қоидалари бўйича ўқув платформаси",
    question: "Ўнгдан келаётган ҳайдовчи қачон йўл беради?",
  },
  {
    locale: "ru",
    messages: ruMessages,
    hero: "готовым",
    login: "Войти",
    feature: "Умное повторение",
    footer: "Учебная платформа по правилам дорожного движения Узбекистана",
    question: "Когда водитель должен уступить машине справа?",
  },
] as const;

function renderWithIntl(localeCase: (typeof localeCases)[number]) {
  return render(
    <NextIntlClientProvider locale={localeCase.locale} messages={localeCase.messages}>
      <LandingPage />
    </NextIntlClientProvider>
  );
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockImplementation((input: string) => {
      const localeCase = localeCases.find(({ locale }) => input.includes(`locale=${locale}`)) ?? localeCases[0];
      return Promise.resolve(
        new Response(
          JSON.stringify({
            data: {
              id: "question-1",
              text: localeCase.question,
              image_url: null,
              answers: [
                { id: "answer-1", position: 1, text: "A", image_url: null },
                { id: "answer-2", position: 2, text: "B", image_url: null },
              ],
            },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        )
      );
    })
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("LandingPage i18n and accessibility", () => {
  it.each(localeCases)("renders complete translated content for $locale", async (localeCase) => {
    const { container } = renderWithIntl(localeCase);

    expect(screen.getByText(localeCase.hero)).toBeInTheDocument();
    expect(
      screen.getByRole("group", { name: languageSwitcherLabel[localeCase.locale] })
    ).toBeInTheDocument();
    const loginLinks = screen.getAllByRole("link", { name: localeCase.login });
    expect(loginLinks[0]).toHaveAttribute("href", `/${localeCase.locale}/login`);
    expect(loginLinks.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(localeCase.feature)).toBeInTheDocument();
    expect(screen.getByText(localeCase.footer)).toBeInTheDocument();
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /\+998 71 200 00 00/ })).toHaveAttribute(
      "href",
      "tel:+998712000000"
    );
    expect(screen.getByRole("link", { name: /support@drivergo\.uz/ })).toBeInTheDocument();
    expect(await screen.findByText(localeCase.question)).toBeInTheDocument();
    expect(screen.getAllByText("Driver Go").length).toBeGreaterThan(0);
    expect(container.textContent).not.toMatch(/[🚗👋🎉]/u);
  });

  it.each(localeCases.slice(1))("does not leak Latin Uzbek chrome into $locale", async (localeCase) => {
    const { container } = renderWithIntl(localeCase);
    await screen.findByText(localeCase.question);

    expect(container.textContent).not.toContain("Bepul boshlash");
    expect(container.textContent).not.toContain("Yo'l belgilari katalogi");
    expect(container.textContent).not.toContain("Imtihon zaliga");
    expect(container.textContent).not.toContain("Minglab bo'lajak haydovchilar");
  });

  it("requests the real demo in the current locale", async () => {
    renderWithIntl(localeCases[2]);
    await screen.findByText(localeCases[2].question);
    expect(fetch).toHaveBeenCalledWith(
      "/api/proxy/demo/question?locale=ru",
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
  });

  it("opens the public diagnostic instead of redirecting the primary CTA to login", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: string) => {
        if (input.includes("/site/home")) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                data: {
                  headline: "",
                  subtitle: "",
                  ctaLabel: "CMS diagnostika",
                  ctaHref: "/uz-Latn/login",
                },
              }),
              { status: 200, headers: { "Content-Type": "application/json" } },
            ),
          );
        }
        return Promise.resolve(
          new Response(
            JSON.stringify({
              data: {
                id: "question-1",
                text: localeCases[0].question,
                image_url: null,
                answers: [
                  { id: "answer-1", position: 1, text: "A", image_url: null },
                  { id: "answer-2", position: 2, text: "B", image_url: null },
                ],
              },
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        );
      }),
    );
    renderWithIntl(localeCases[0]);
    const diagnosticLinks = await screen.findAllByRole("link", { name: "CMS diagnostika" });
    expect(diagnosticLinks.length).toBeGreaterThanOrEqual(1);
    for (const item of diagnosticLinks) {
      expect(item).toHaveAttribute("href", "/uz-Latn/diagnostic");
    }
  });
});
