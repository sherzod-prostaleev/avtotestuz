import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../messages/uz-Latn.json";
import uzCyrlMessages from "../../../messages/uz-Cyrl.json";
import ruMessages from "../../../messages/ru.json";
import { Sidebar } from "./sidebar";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/dashboard",
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("@/hooks/use-user-stats", () => ({
  useUserStats: () => ({ user: null, entitlement: null, streak: null }),
}));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

const localeCases = [
  {
    locale: "uz-Latn",
    messages: uzLatnMessages,
    dashboard: "Bosh sahifa",
    saved: "Saqlangan savollar",
    user: "O'quvchi",
    openMenu: "Menyuni ochish",
  },
  {
    locale: "uz-Cyrl",
    messages: uzCyrlMessages,
    dashboard: "Бош саҳифа",
    saved: "Сақланган саволлар",
    user: "Ўқувчи",
    openMenu: "Менюни очиш",
  },
  {
    locale: "ru",
    messages: ruMessages,
    dashboard: "Главная",
    saved: "Сохранённые вопросы",
    user: "Ученик",
    openMenu: "Открыть меню",
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
  it.each(localeCases)("renders translated navigation for $locale", (localeCase) => {
    const { container } = renderWithIntl(localeCase);

    expect(screen.getByRole("link", { name: localeCase.dashboard })).toHaveAttribute(
      "href",
      `/${localeCase.locale}/dashboard`
    );
    expect(screen.getByRole("link", { name: localeCase.saved })).toHaveAttribute(
      "href",
      `/${localeCase.locale}/saved`
    );
    expect(screen.getByText(localeCase.user)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: localeCase.openMenu })).toBeInTheDocument();
    expect(container.textContent).not.toMatch(/[🚗👋🎉]/u);
  });

  it.each(localeCases.slice(1))("does not leak Latin Uzbek chrome into $locale", (localeCase) => {
    const { container } = renderWithIntl(localeCase);

    expect(container.textContent).not.toContain("Bosh sahifa");
    expect(container.textContent).not.toContain("Kunlik Streak");
    expect(container.textContent).not.toContain("Profilni ko'rish");
    expect(container.textContent).not.toContain("Yo'l belgilari");
  });
});
