import { render, screen, within } from "@testing-library/react";
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

const localeCases = [
  {
    locale: "uz-Latn",
    messages: uzLatnMessages,
    welcome: "Xush kelibsiz, Dilshod!",
    modes: "O'quv rejimlari",
    signs: "O'zbekiston yo'l belgilari katalogi",
    saved: "Saqlangan savollar",
    ready: "Imtihonga tayyor!",
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
    signs: "Ўзбекистон йўл белгилари каталоги",
    saved: "Сақланган саволлар",
    ready: "Имтиҳонга тайёр!",
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
    signs: "Каталог дорожных знаков Узбекистана",
    saved: "Сохранённые вопросы",
    ready: "Готов к экзамену!",
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
          id: "finished-newest",
          mode: "exam",
          status: "passed",
          total: 20,
          started_at: "2026-07-22T13:00:00Z",
        },
        {
          id: "",
          mode: "exam",
          status: "in_progress",
          total: 20,
          started_at: "2026-07-22T12:00:00Z",
        },
        {
          id: "invalid-date",
          mode: "variant",
          status: "in_progress",
          total: 20,
          started_at: "not-a-date",
        },
        {
          id: "resume-newest",
          mode: "mistakes",
          status: "in_progress",
          total: 12,
          started_at: "2026-07-22T11:00:00Z",
        },
        {
          id: "resume-older",
          mode: "practice",
          status: "in_progress",
          total: 20,
          started_at: "2026-07-22T10:00:00Z",
        },
      ],
    });

    renderWithIntl(localeCases[2]);

    expect(screen.getByRole("link", { name: "Продолжить" })).toHaveAttribute(
      "href",
      "/ru/session/resume-newest"
    );
    const resumeRegion = screen.getByRole("region", { name: "Незавершённая сессия" });
    expect(within(resumeRegion).getByText("Работа над ошибками")).toBeInTheDocument();
    expect(within(resumeRegion).getByText("Всего: 12 вопросов")).toBeInTheDocument();
    expect(screen.getAllByText("Незавершённая сессия")).toHaveLength(1);
  });

  it("does not render the resume banner without a valid in-progress session", () => {
    mockStats();
    setHistory({
      sessions: [
        {
          id: "finished",
          mode: "exam",
          status: "passed",
          total: 20,
          started_at: "2026-07-22T10:00:00Z",
        },
      ],
    });

    renderWithIntl(localeCases[0]);

    expect(screen.queryByText("Tugallanmagan sessiya")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Davom ettirish" })).not.toBeInTheDocument();
  });

  it.each(localeCases)(
    "keeps history loading and errors localized and separate from core stats errors for $locale",
    (localeCase) => {
      mockStats("Failed to load user data");
      setHistory({ error: "Failed to load session history" });

      const { rerender } = renderWithIntl(localeCase);

      const alerts = screen.getAllByRole("alert");
      expect(alerts).toHaveLength(2);
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
});
