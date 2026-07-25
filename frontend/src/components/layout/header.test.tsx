import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../messages/uz-Latn.json";
import uzCyrlMessages from "../../../messages/uz-Cyrl.json";
import ruMessages from "../../../messages/ru.json";
import { Header } from "./header";
import * as useUserStatsModule from "@/hooks/use-user-stats";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/uz-Latn/dashboard",
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("@/hooks/use-user-stats", () => ({ useUserStats: vi.fn() }));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

const localeCases = [
  {
    locale: "uz-Latn",
    messages: uzLatnMessages,
    tickets: "Biletlar",
    free: "Bepul",
    profile: "Profilni ochish",
  },
  {
    locale: "uz-Cyrl",
    messages: uzCyrlMessages,
    tickets: "Билетлар",
    free: "Бепул",
    profile: "Профилни очиш",
  },
  {
    locale: "ru",
    messages: ruMessages,
    tickets: "Билеты",
    free: "Бесплатно",
    profile: "Открыть профиль",
  },
] as const;

describe("Header i18n and accessibility", () => {
  beforeEach(() => {
    vi.mocked(useUserStatsModule.useUserStats).mockReturnValue({
      user: null,
      streak: { current_streak: 4, max_streak: 4, today_answered: 0, daily_target: 10 },
      entitlement: { is_vip: false },
      stats: null,
      loading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useUserStatsModule.useUserStats>);
  });

  it.each(localeCases)("renders translated controls for $locale without emoji assets", (localeCase) => {
    const { container } = render(
      <NextIntlClientProvider locale={localeCase.locale} messages={localeCase.messages}>
        <Header />
      </NextIntlClientProvider>
    );

    expect(screen.getByRole("link", { name: localeCase.tickets })).toHaveAttribute(
      "href",
      `/${localeCase.locale}/tickets`
    );
    expect(screen.getByText(localeCase.free)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: localeCase.profile })).toHaveAttribute(
      "href",
      `/${localeCase.locale}/profile`
    );
    expect(screen.getByRole("group")).toBeInTheDocument();
    expect(container.textContent).not.toMatch(/[🚗👋🎉]/u);
  });

  // Regression test: the VIP badge used to default to the "Free" claim
  // while `useUserStats` was still loading, so a VIP user saw it flash
  // "Free" before flipping to "VIP" on every page load.
  it("does not claim the visitor is on the free plan while entitlement is still loading", () => {
    vi.mocked(useUserStatsModule.useUserStats).mockReturnValue({
      user: null,
      streak: { current_streak: 4, max_streak: 4, today_answered: 0, daily_target: 10 },
      entitlement: null,
      stats: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useUserStatsModule.useUserStats>);

    render(
      <NextIntlClientProvider locale="ru" messages={ruMessages}>
        <Header />
      </NextIntlClientProvider>
    );

    expect(screen.queryByText("Бесплатно")).not.toBeInTheDocument();
    expect(screen.queryByText("VIP")).not.toBeInTheDocument();
  });

  it("shows the VIP badge once loading finishes for a VIP visitor", () => {
    vi.mocked(useUserStatsModule.useUserStats).mockReturnValue({
      user: null,
      streak: { current_streak: 4, max_streak: 4, today_answered: 0, daily_target: 10 },
      entitlement: { is_vip: true },
      stats: null,
      loading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useUserStatsModule.useUserStats>);

    render(
      <NextIntlClientProvider locale="ru" messages={ruMessages}>
        <Header />
      </NextIntlClientProvider>
    );

    expect(screen.getByText("VIP")).toBeInTheDocument();
  });
});
