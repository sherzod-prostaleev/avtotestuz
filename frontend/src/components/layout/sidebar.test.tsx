import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../messages/uz-Latn.json";
import uzCyrlMessages from "../../../messages/uz-Cyrl.json";
import ruMessages from "../../../messages/ru.json";
import { Sidebar } from "./sidebar";
import * as useUserStatsModule from "@/hooks/use-user-stats";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/uz-Latn/dashboard",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

vi.mock("@/i18n/navigation", () => ({
  usePathname: () => "/dashboard",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

vi.mock("@/hooks/use-user-stats", () => ({ useUserStats: vi.fn() }));

vi.mock("@/hooks/use-variant-count", () => ({ useVariantCount: () => 63 }));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

const localeCases = [
  {
    locale: "uz-Latn",
    messages: uzLatnMessages,
    dashboard: "Bosh sahifa",
    saved: "Saqlangan savollar",
    more: "Ko'proq",
    user: "O'quvchi",
    openMenu: "Menyuni ochish",
    imageQuestions: "Rasmli savollar",
    textQuestions: "Rasmsiz savollar",
  },
  {
    locale: "uz-Cyrl",
    messages: uzCyrlMessages,
    dashboard: "Бош саҳифа",
    saved: "Сақланган саволлар",
    more: "Кўпроқ",
    user: "Ўқувчи",
    openMenu: "Менюни очиш",
    imageQuestions: "Расмли саволлар",
    textQuestions: "Расмсиз саволлар",
  },
  {
    locale: "ru",
    messages: ruMessages,
    dashboard: "Главная",
    saved: "Сохранённые вопросы",
    more: "Ещё",
    user: "Ученик",
    openMenu: "Открыть меню",
    imageQuestions: "Вопросы с картинкой",
    textQuestions: "Вопросы без картинки",
  },
] as const;

function renderWithIntl(localeCase: (typeof localeCases)[number]) {
  return render(
    <NextIntlClientProvider locale={localeCase.locale} messages={localeCase.messages}>
      <Sidebar />
    </NextIntlClientProvider>
  );
}

describe("Sidebar i18n and accessibility", () => {
  beforeEach(() => {
    vi.mocked(useUserStatsModule.useUserStats).mockReturnValue({
      user: null,
      entitlement: null,
      streak: null,
      stats: null,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  it.each(localeCases)("renders translated navigation for $locale", (localeCase) => {
    const { container } = renderWithIntl(localeCase);

    const dashboard = screen
      .getAllByRole("link")
      .find((link) => link.getAttribute("href") === `/${localeCase.locale}/dashboard` && link.textContent?.includes(localeCase.dashboard));
    expect(dashboard).toBeTruthy();
    const ticketsLabel = localeCase.messages.Sidebar.navTickets.replace("{count}", "63");
    expect(screen.getByRole("link", { name: ticketsLabel })).toHaveAttribute(
      "href",
      `/${localeCase.locale}/tickets`
    );
    fireEvent.click(screen.getByRole("button", { name: localeCase.more }));
    expect(screen.getByRole("link", { name: localeCase.saved })).toHaveAttribute(
      "href",
      `/${localeCase.locale}/saved`
    );
    expect(screen.getByText(localeCase.user)).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: localeCase.openMenu }).length).toBeGreaterThan(0);
    expect(container.textContent).not.toMatch(/[🚗👋🎉]/u);
  });

  // The image split lives inside Practice, not in the nav. Two sibling entries
  // differing only by a boolean cluttered the sidebar and pointed several nav
  // items at /session/start, which starts a session as a mount side effect.
  it.each(localeCases)("keeps question filters out of the nav for $locale", (localeCase) => {
    renderWithIntl(localeCase);

    expect(screen.queryByText(localeCase.imageQuestions)).not.toBeInTheDocument();
    expect(screen.queryByText(localeCase.textQuestions)).not.toBeInTheDocument();
    // One exam entry in the desktop sidebar list + one in the thumb-zone tabs.
    expect(
      screen.queryAllByRole("link").filter((link) => link.getAttribute("href")?.includes("session/start"))
    ).toHaveLength(2);
  });

  it.each(localeCases.slice(1))("does not leak Latin Uzbek chrome into $locale", (localeCase) => {
    const { container } = renderWithIntl(localeCase);

    expect(container.textContent).not.toContain("Bosh sahifa");
    expect(container.textContent).not.toContain("Kunlik Streak");
    expect(container.textContent).not.toContain("Profilni ko'rish");
    expect(container.textContent).not.toContain("Yo'l belgilari");
  });

  // Regression test for the F5 flicker: entitlement defaults to "not VIP"
  // before the fetch resolves, so a VIP user briefly saw "Upgrade to VIP"
  // and the trial countdown panel vanish on every refresh. While loading,
  // neither claim should render.
  it("shows a neutral placeholder instead of the free/VIP badge while entitlement is loading", () => {
    vi.mocked(useUserStatsModule.useUserStats).mockReturnValue({
      user: null,
      entitlement: null,
      streak: null,
      stats: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl(localeCases[2]);

    expect(screen.queryByText("VIP")).not.toBeInTheDocument();
    expect(screen.queryByText("Оформить VIP")).not.toBeInTheDocument();
  });

  it("renders the real VIP badge once the entitlement finishes loading", () => {
    vi.mocked(useUserStatsModule.useUserStats).mockReturnValue({
      user: { id: "u1", phone: "+998901234567" },
      entitlement: { is_vip: true, valid_until: null },
      streak: { current_streak: 3, max_streak: 3, today_answered: 1, daily_target: 10 },
      stats: null,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl(localeCases[2]);

    expect(screen.getByText("VIP")).toBeInTheDocument();
  });
});
