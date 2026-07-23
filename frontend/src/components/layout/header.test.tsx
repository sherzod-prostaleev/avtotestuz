import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../messages/uz-Latn.json";
import uzCyrlMessages from "../../../messages/uz-Cyrl.json";
import ruMessages from "../../../messages/ru.json";
import { Header } from "./header";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/uz-Latn/dashboard",
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("@/hooks/use-user-stats", () => ({
  useUserStats: () => ({
    streak: { current_streak: 4 },
    entitlement: { is_vip: false },
  }),
}));

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
});
