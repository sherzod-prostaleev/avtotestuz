import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import { TelegramLinkCard } from "../telegram-link-card";
import * as apiClient from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <TelegramLinkCard />
    </NextIntlClientProvider>
  );
}

describe("TelegramLinkCard", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("shows not-linked state and mints a deep link", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: false });
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({
      token: "tok123",
      deep_link: "https://t.me/AvtoTestBot?start=tok123",
      expires_at: "2026-07-26T01:00:00Z",
    });
    const openSpy = vi.spyOn(window, "open").mockReturnValue(null);

    renderWithIntl();

    expect(await screen.findByText("Hali Telegram akkaunt bog'lanmagan.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Telegramni bog'lash" }));

    await waitFor(() => {
      expect(post).toHaveBeenCalledWith("me/telegram/link-token");
      expect(openSpy).toHaveBeenCalledWith(
        "https://t.me/AvtoTestBot?start=tok123",
        "_blank",
        "noopener,noreferrer"
      );
    });
    expect(screen.getByText("https://t.me/AvtoTestBot?start=tok123")).toBeInTheDocument();
  });

  it("shows linked username", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      linked: true,
      username: "sherzod",
      linked_at: "2026-07-26T00:00:00Z",
    });

    renderWithIntl();

    expect(await screen.findByText("Telegram bog'langan")).toBeInTheDocument();
    expect(screen.getByText(/@sherzod/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Qayta bog'lash" })).toBeInTheDocument();
  });

  it("shows unconfigured message when bot username is missing", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: false });
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(
      new ApiError("bot off", "telegram_bot_unconfigured", 503)
    );

    renderWithIntl();
    fireEvent.click(await screen.findByRole("button", { name: "Telegramni bog'lash" }));

    expect(
      await screen.findByText(/Telegram bot hozircha sozlanmagan/)
    ).toBeInTheDocument();
  });
});
