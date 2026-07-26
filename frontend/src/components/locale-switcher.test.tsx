import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../messages/uz-Latn.json";
import { LocaleSwitcher } from "./locale-switcher";

const replaceMock = vi.fn();

vi.mock("@/i18n/navigation", () => ({
  usePathname: () => "/dashboard",
  useRouter: () => ({ replace: replaceMock, push: vi.fn() }),
}));

describe("LocaleSwitcher", () => {
  beforeEach(() => {
    replaceMock.mockClear();
    window.history.replaceState({}, "", "/uz-Latn/dashboard");
  });

  it("exposes a language group and soft-switches locale without scrolling", () => {
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        <LocaleSwitcher />
      </NextIntlClientProvider>
    );

    expect(screen.getByRole("group", { name: "Tilni tanlash" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "O'z", pressed: true })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Ru" }));
    expect(replaceMock).toHaveBeenCalledWith("/dashboard", {
      locale: "ru",
      scroll: false,
    });
  });

  it("keeps the current query string when switching locale", () => {
    window.history.replaceState({}, "", "/uz-Latn/session/start?mode=exam");

    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        <LocaleSwitcher />
      </NextIntlClientProvider>
    );

    fireEvent.click(screen.getByRole("button", { name: "Ru" }));
    expect(replaceMock).toHaveBeenCalledWith(
      { pathname: "/dashboard", query: { mode: "exam" } },
      { locale: "ru", scroll: false }
    );
  });
});
