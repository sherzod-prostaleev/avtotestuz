import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import { MobileTelegram } from "../mobile-telegram";
import * as apiClient from "@/lib/api-client";

const tg = vi.hoisted(() => ({ webApp: null as null | Record<string, unknown> }));
vi.mock("@/components/telegram/telegram-provider", () => ({ useTelegram: () => tg.webApp }));

function renderPanel() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <MobileTelegram onBack={() => {}} />
    </NextIntlClientProvider>,
  );
}

function enterMiniApp() {
  const webApp = { initData: "x", openLink: vi.fn(), openTelegramLink: vi.fn() };
  tg.webApp = webApp;
  (window as { Telegram?: unknown }).Telegram = { WebApp: webApp };
  return webApp;
}

const LINK = {
  token: "tok123",
  deep_link: "https://t.me/AvtoTestBot?start=tok123",
  expires_at: "2026-07-26T01:00:00Z",
};

describe("MobileTelegram", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    tg.webApp = null;
    delete (window as { Telegram?: unknown }).Telegram;
  });

  it("keeps the link and relink actions on the website", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "sherzod" });
    vi.spyOn(apiClient, "apiPost").mockResolvedValue(LINK);
    const openSpy = vi.spyOn(window, "open").mockReturnValue(null);

    renderPanel();
    fireEvent.click(await screen.findByRole("button", { name: "Qayta bog'lash" }));

    await waitFor(() =>
      expect(openSpy).toHaveBeenCalledWith(LINK.deep_link, "_blank", "noopener,noreferrer"),
    );
  });

  it("shows only the linked status inside the Mini App", async () => {
    enterMiniApp();
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "sherzod" });

    renderPanel();

    expect(await screen.findByText("Telegram ulangan")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Qayta bog'lash" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Telegramni bog'lash" })).not.toBeInTheDocument();
  });

  it("still lets an unlinked Mini App learner link, through Telegram", async () => {
    const webApp = enterMiniApp();
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: false });
    vi.spyOn(apiClient, "apiPost").mockResolvedValue(LINK);
    const openSpy = vi.spyOn(window, "open").mockReturnValue(null);

    renderPanel();
    fireEvent.click(await screen.findByRole("button", { name: "Telegramni bog'lash" }));

    await waitFor(() => expect(webApp.openTelegramLink).toHaveBeenCalledWith(LINK.deep_link));
    expect(openSpy).not.toHaveBeenCalled();
  });
});
