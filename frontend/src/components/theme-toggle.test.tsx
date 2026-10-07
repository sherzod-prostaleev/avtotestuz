import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { beforeEach, describe, it, expect, vi } from "vitest";
import { ThemeProvider } from "next-themes";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../messages/uz-Latn.json";
import { ThemeToggle } from "./theme-toggle";

const tg = vi.hoisted(() => ({ webApp: null as null | object }));
vi.mock("@/components/telegram/telegram-provider", () => ({ useTelegram: () => tg.webApp }));

function renderWithTheme() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ThemeProvider
        attribute="class"
        defaultTheme="dark"
        enableSystem={false}
        value={{ light: "light", dark: "dark" }}
      >
        <ThemeToggle />
      </ThemeProvider>
    </NextIntlClientProvider>
  );
}

describe("ThemeToggle", () => {
  beforeEach(() => {
    tg.webApp = null;
  });

  it("shows the sun icon (offering to switch to light) while the theme is dark", async () => {
    renderWithTheme();
    await waitFor(() => expect(screen.getByTestId("theme-toggle-sun")).toBeInTheDocument());
  });

  it("switches to the moon icon after being clicked", async () => {
    renderWithTheme();
    await waitFor(() => expect(screen.getByRole("button")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button"));
    await waitFor(() => expect(screen.getByTestId("theme-toggle-moon")).toBeInTheDocument());
  });

  // Inside the Mini App Telegram's colour scheme drives the theme; a toggle
  // would fight it and lose on the next themeChanged or launch.
  it("renders nothing inside the Telegram Mini App", async () => {
    tg.webApp = { initData: "x" };
    const { container } = renderWithTheme();
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    // Not even the pre-mount placeholder box (next-themes' own <script> stays).
    expect(container.querySelector("span, button")).toBeNull();
  });
});
