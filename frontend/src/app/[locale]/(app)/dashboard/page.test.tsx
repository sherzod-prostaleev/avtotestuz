import { fireEvent, render, screen, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../../../messages/uz-Latn.json";
import uzCyrlMessages from "../../../../../messages/uz-Cyrl.json";
import ruMessages from "../../../../../messages/ru.json";
import DashboardPage from "./page";
import * as useUserStatsModule from "@/hooks/use-user-stats";
import * as useSessionHistoryModule from "@/hooks/use-session-history";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

// GrandMockCard makes its own network call (GET /me/mock-eligibility) and has
// its own dedicated test suite (grand-mock-card.test.tsx); stubbing it here
// keeps this file focused on the dashboard's own chrome/i18n/resume-banner
// behavior instead of also driving that unrelated request to resolution.
vi.mock("@/components/mock/grand-mock-card", () => ({ GrandMockCard: () => null }));

const localeCases = [
  {
    locale: "uz-Latn",
    messages: uzLatnMessages,
    welcome: "Xush kelibsiz, Dilshod!",
    modes: "O'quv rejimlari",
    nextAction: "Keyingi eng yaxshi qadam",
    signs: "O'zbekiston yo'l belgilari katalogi",
    saved: "Saqlangan savollar",
    ready: "Yuqori qamrov",
    resumeTitle: "Tugallanmagan sessiya",
    resumeMode: "Mashq",
    resumeTotal: "Jami: 20 savol",
    resumeCta: "Davom ettirish",
    resumeStartedPrefix: "Boshlangan:",
    resumeLoading: "Tugallanmagan sessiya tekshirilmoqda...",
    resumeError: "Tugallanmagan sessiyani tekshirib bo'lmadi.",
    coreError: "Kabinet ma'lumotlarini yuklab bo'lmadi.",
  },
  {
    locale: "uz-Cyrl",
    messages: uzCyrlMessages,
    welcome: "Хуш келибсиз, Dilshod!",
    modes: "Ўқув режимлари",
    nextAction: "Кейинги энг яхши қадам",
    signs: "Ўзбекистон йўл белгилари каталоги",
    saved: "Сақланган саволлар",
    ready: "Юқори қамров",
    resumeTitle: "Тугалланмаган сессия",
    resumeMode: "Машқ",
    resumeTotal: "Жами: 20 савол",
    resumeCta: "Давом эттириш",
    resumeStartedPrefix: "Бошланган:",
    resumeLoading: "Тугалланмаган сессия текширилмоқда...",
    resumeError: "Тугалланмаган сессияни текшириб бўлмади.",
    coreError: "Кабинет маълумотларини юклаб бўлмади.",
  },
  {
    locale: "ru",
    messages: ruMessages,
    welcome: "Добро пожаловать, Dilshod!",
    modes: "Режимы обучения",
    nextAction: "Следующий лучший шаг",
    signs: "Каталог дорожных знаков Узбекистана",
    saved: "Сохранённые вопросы",
    ready: "Высокое покрытие",
    resumeTitle: "Незавершённая сессия",
    resumeMode: "Практика",
    resumeTotal: "Всего: 20 вопросов",
    resumeCta: "Продолжить",
    resumeStartedPrefix: "Начата:",
    resumeLoading: "Проверяем незавершённую сессию...",
    resumeError: "Не удалось проверить незавершённую сессию.",
    coreError: "Не удалось загрузить данные кабинета.",
  },
] as const;

function mockStats(error: string | null = null) {
  vi.spyOn(useUserStatsModule, "useUserStats").mockReturnValue({
    user: { id: "u1", phone: "+998901234567", name: "Dilshod" },
    entitlement: { is_vip: true },
    streak: { current_streak: 7, max_streak: 15, today_answered: 5, daily_target: 10 },
    stats: {
      readiness_pct: 82,
      due_questions_count: 5,
      total_answered: 200,
      total_correct: 180,
      category_mastery: [],
    },
    loading: false,
    error,
    refetch: vi.fn(),
  });
}

function mockStatsLoading() {
  vi.spyOn(useUserStatsModule, "useUserStats").mockReturnValue({
    user: null,
    entitlement: null,
    streak: null,
    stats: null,
    loading: true,
    error: null,
    refetch: vi.fn(),
  });
}

function historyState(
  overrides: Partial<ReturnType<typeof useSessionHistoryModule.useSessionHistory>> = {}
): ReturnType<typeof useSessionHistoryModule.useSessionHistory> {
  return {
    sessions: [],
    loading: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  };
}

function setHistory(
  overrides: Partial<ReturnType<typeof useSessionHistoryModule.useSessionHistory>> = {}
) {
  vi.mocked(useSessionHistoryModule.useSessionHistory).mockReturnValue(historyState(overrides));
}

function renderWithIntl(localeCase: (typeof localeCases)[number]) {
  return render(
    <NextIntlClientProvider locale={localeCase.locale} messages={localeCase.messages}>
      <DashboardPage />
    </NextIntlClientProvider>
  );
}

describe("DashboardPage i18n and accessibility", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
    window.localStorage.setItem("dg-onboarding-v1-dismissed", "1");
    vi.spyOn(useSessionHistoryModule, "useSessionHistory").mockReturnValue(historyState());
  });

  it.each(localeCases)("renders translated dashboard chrome for $locale", (localeCase) => {
    mockStats();
    setHistory({
      sessions: [
        {
          id: "resume-current",
          mode: "practice",
          status: "in_progress",
          total: 20,
          started_at: "2026-07-22T10:00:00Z",
        },
      ],
    });
    const { container } = renderWithIntl(localeCase);

    expect(screen.getByText(localeCase.welcome)).toBeInTheDocument();
    expect(screen.getByText(localeCase.modes)).toBeInTheDocument();
    expect(screen.getByText(localeCase.nextAction)).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /O'quv yo'li|Ўқув йўли|Учебный путь/ })).not.toBeInTheDocument();
    expect(screen.getByText(localeCase.signs)).toBeInTheDocument();
    expect(screen.getByText(localeCase.ready)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: new RegExp(localeCase.saved) })).toHaveAttribute(
      "href",
      `/${localeCase.locale}/saved`
    );
    const resumeRegion = screen.getByRole("region", { name: localeCase.resumeTitle });
    expect(within(resumeRegion).getByText(localeCase.resumeMode)).toBeInTheDocument();
    expect(within(resumeRegion).getByText(localeCase.resumeTotal)).toBeInTheDocument();
    expect(
      within(resumeRegion).getByText((content) => content.startsWith(localeCase.resumeStartedPrefix))
    ).toBeInTheDocument();
    expect(resumeRegion).not.toHaveTextContent("0/20");
    expect(screen.getByRole("link", { name: localeCase.resumeCta })).toHaveAttribute(
      "href",
      `/${localeCase.locale}/session/resume-current`
    );
    expect(container.textContent).not.toMatch(/[🚗👋🎉]/u);
  });

  it("shows only the newest valid in-progress session", () => {
    mockStats();
    setHistory({
      sessions: [
        {
          id: "older",
          mode: "exam",
          status: "in_progress",
          total: 20,
          started_at: "2026-07-21T10:00:00Z",
        },
        {
          id: "newest",
          mode: "practice",
          status: "in_progress",
          total: 20,
          started_at: "2026-07-22T10:00:00Z",
        },
        {
          id: "finished",
          mode: "practice",
          status: "passed",
          total: 20,
          started_at: "2026-07-22T11:00:00Z",
        },
      ],
    });
    renderWithIntl(localeCases[0]);

    expect(screen.getByRole("link", { name: "Davom ettirish" })).toHaveAttribute(
      "href",
      "/uz-Latn/session/newest"
    );
  });

  it.each(localeCases)(
    "surfaces translated load failures for core stats and resume history in $locale",
    (localeCase) => {
      mockStats("Failed to load user data");
      setHistory({ error: "Failed to load session history", sessions: [] });
      const { rerender } = renderWithIntl(localeCase);

      const alerts = screen.getAllByRole("alert");
      expect(alerts[0]).toHaveTextContent(localeCase.coreError);
      expect(alerts[1]).toHaveTextContent(localeCase.resumeError);
      expect(screen.queryByText("Failed to load user data")).not.toBeInTheDocument();
      expect(screen.queryByText("Failed to load session history")).not.toBeInTheDocument();

      setHistory({ loading: true });
      rerender(
        <NextIntlClientProvider locale={localeCase.locale} messages={localeCase.messages}>
          <DashboardPage />
        </NextIntlClientProvider>
      );
      expect(screen.getByRole("status")).toHaveTextContent(localeCase.resumeLoading);
    }
  );

  it.each(localeCases.slice(1))("does not leak Latin Uzbek chrome into $locale", (localeCase) => {
    mockStats();
    const { container } = renderWithIntl(localeCase);

    expect(container.textContent).not.toContain("O'quv Rejimlari");
    expect(container.textContent).not.toContain("O'quv yo'li");
    expect(container.textContent).not.toContain("Aqlli takrorlash");
    expect(container.textContent).not.toContain("Imtihonga tayyor");
    expect(container.textContent).not.toContain("Katalogni ko'rish");
    expect(container.textContent).not.toContain("ta takrorlash");
  });

  it("uses a localized error fallback instead of leaking hook text", () => {
    mockStats("Failed to load user data");
    renderWithIntl(localeCases[2]);

    expect(screen.getByRole("alert")).toHaveTextContent("Не удалось загрузить данные кабинета.");
    expect(screen.queryByText("Failed to load user data")).not.toBeInTheDocument();
  });

  // Regression coverage for the F5 flicker: `entitlement`/`stats` default to
  // null/false while `useUserStats` is loading, which used to render the
  // "Free" VIP badge, a "0%"-style readiness card, and a specific next-step
  // recommendation immediately — then swap to the real (possibly very
  // different) VIP badge, numbers, and recommendation once the fetch
  // resolved. None of those guesses should render while loading.
  it("does not flash the free-tier badge, stat numbers, or a guessed recommendation while loading", () => {
    mockStatsLoading();
    setHistory({ loading: true });

    const { rerender } = renderWithIntl(localeCases[2]);

    expect(screen.queryByText("VIP-доступ")).not.toBeInTheDocument();
    expect(screen.queryByText("Оформить VIP")).not.toBeInTheDocument();
    expect(screen.queryByText("Следующий лучший шаг")).not.toBeInTheDocument();
    expect(screen.queryByText("0%")).not.toBeInTheDocument();
    expect(screen.queryByText("Слабая зона пока не определена")).not.toBeInTheDocument();

    mockStats();
    setHistory();
    rerender(
      <NextIntlClientProvider locale={localeCases[2].locale} messages={localeCases[2].messages}>
        <DashboardPage />
      </NextIntlClientProvider>
    );

    expect(screen.getByText("VIP-доступ")).toBeInTheDocument();
    expect(screen.queryByText("Оформить VIP")).not.toBeInTheDocument();
    expect(screen.getByText("Следующий лучший шаг")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Учебный путь" })).not.toBeInTheDocument();
  });

  it("reserves the weakest-categories layout with a skeleton instead of popping the section in", () => {
    mockStatsLoading();

    const { container } = renderWithIntl(localeCases[2]);

    expect(screen.queryByText(/Самые слабые темы/)).not.toBeInTheDocument();
    expect(container.querySelectorAll(".animate-pulse").length).toBeGreaterThan(0);
  });

  it("shows a dismissible onboarding callout for new users and remembers dismissal", () => {
    window.localStorage.removeItem("dg-onboarding-v1-dismissed");
    mockStats();
    vi.spyOn(useUserStatsModule, "useUserStats").mockReturnValue({
      user: { id: "u1", phone: "+998901234567", name: "Dilshod" },
      entitlement: { is_vip: false },
      streak: { current_streak: 0, max_streak: 0, today_answered: 0, daily_target: 10 },
      stats: {
        readiness_pct: 0,
        due_questions_count: 0,
        total_answered: 0,
        total_correct: 0,
        category_mastery: [],
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    setHistory({ sessions: [] });

    renderWithIntl(localeCases[0]);

    expect(screen.getByRole("heading", { name: "Qanday boshlash kerak?" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Mashqni boshlash/ })).toHaveAttribute(
      "href",
      "/uz-Latn/practice"
    );

    fireEvent.click(screen.getByRole("button", { name: "Yopish" }));

    expect(screen.queryByRole("heading", { name: "Qanday boshlash kerak?" })).not.toBeInTheDocument();
    expect(window.localStorage.getItem("dg-onboarding-v1-dismissed")).toBe("1");
  });

  it("recommends weakest-category practice via next-best-step when due queue is empty", () => {
    mockStats();
    vi.spyOn(useUserStatsModule, "useUserStats").mockReturnValue({
      user: { id: "u1", phone: "+998901234567", name: "Dilshod" },
      entitlement: { is_vip: false },
      streak: { current_streak: 1, max_streak: 1, today_answered: 2, daily_target: 10 },
      stats: {
        readiness_pct: 20,
        due_questions_count: 0,
        total_answered: 20,
        total_correct: 14,
        category_mastery: [
          { code: "signs", name: "Belgilar", answered: 4, correct: 2, mastery_pct: 10, studied: 2, total: 40 },
          { code: "priority", name: "Imtiyoz", answered: 6, correct: 4, mastery_pct: 40, studied: 4, total: 30 },
        ],
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    setHistory();

    renderWithIntl(localeCases[0]);

    expect(screen.queryByRole("heading", { name: "O'quv yo'li" })).not.toBeInTheDocument();
    expect(screen.getByText("Keyingi eng yaxshi qadam")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Mashqni boshlash/ })).toHaveAttribute(
      "href",
      "/uz-Latn/session/start?mode=practice&category_id=signs&count=20"
    );
  });
});
