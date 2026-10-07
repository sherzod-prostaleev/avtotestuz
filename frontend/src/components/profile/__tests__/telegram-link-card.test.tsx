import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import { TelegramLinkCard } from "../telegram-link-card";
import * as apiClient from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";
import { installTelegramHost } from "@/test/telegram-host";

const tg = vi.hoisted(() => ({ webApp: null as null | Record<string, unknown> }));
vi.mock("@/components/telegram/telegram-provider", () => ({ useTelegram: () => tg.webApp }));

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
    tg.webApp = null;
    delete (window as { Telegram?: unknown }).Telegram;
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

  describe("inside the Mini App", () => {
    function enterMiniApp(username: string | null = "sherzod") {
      const webApp = {
        initData: "x",
        initDataUnsafe: { user: { id: 42, username: username ?? undefined } },
        openLink: vi.fn(),
        openTelegramLink: vi.fn(),
      };
      tg.webApp = webApp;
      installTelegramHost();
      (window as { Telegram?: unknown }).Telegram = { WebApp: webApp };
      return webApp;
    }

    // The Mini App is itself the link; offering to (re)link from inside it
    // would only send the learner out to the bot and back.
    it("shows the linked status without link actions", async () => {
      enterMiniApp("Sherzod");
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "@sherzod" });

      renderWithIntl();

      expect(await screen.findByText("Telegram ulangan")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Qayta bog'lash" })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Telegramni bog'lash" })).not.toBeInTheDocument();
    });

    // The linked Telegram id decides when the API sends it: usernames change
    // and can be missing, ids cannot.
    it("compares the linked tg_user_id, not usernames, when it is known", async () => {
      enterMiniApp("new_name");
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "old_name", tg_user_id: 42 });
      renderWithIntl();
      expect(await screen.findByText("Telegram ulangan")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Qayta bog'lash" })).not.toBeInTheDocument();
    });

    it("offers relinking for another tg_user_id even with the same username", async () => {
      enterMiniApp("sherzod");
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "sherzod", tg_user_id: 7 });
      renderWithIntl();
      expect(await screen.findByRole("button", { name: "Qayta bog'lash" })).toBeInTheDocument();
    });

    // The profile is linked, but to another Telegram account than the one that
    // opened the app (or we cannot tell): the learner must be able to relink.
    it.each([
      ["linked to a different account", "aziz", "sherzod"],
      ["the current user has no username", null, "sherzod"],
      ["the linked account has no username", "sherzod", undefined],
    ] as const)("offers relinking when %s", async (_name, current, linked) => {
      const webApp = enterMiniApp(current);
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: linked });
      vi.spyOn(apiClient, "apiPost").mockResolvedValue({
        token: "tok123",
        deep_link: "https://t.me/AvtoTestBot?start=tok123",
        expires_at: "2026-07-26T01:00:00Z",
      });

      renderWithIntl();

      fireEvent.click(await screen.findByRole("button", { name: "Qayta bog'lash" }));
      expect(screen.queryByText("Telegram ulangan")).not.toBeInTheDocument();
      await waitFor(() =>
        expect(webApp.openTelegramLink).toHaveBeenCalledWith("https://t.me/AvtoTestBot?start=tok123"),
      );
    });

    // Launch data 1–24 h old signs in but skips linking: the learner must
    // still be able to link, and the bot link opens inside Telegram.
    it("keeps the link action when not linked and opens the bot through Telegram", async () => {
      const webApp = enterMiniApp();
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: false });
      vi.spyOn(apiClient, "apiPost").mockResolvedValue({
        token: "tok123",
        deep_link: "https://t.me/AvtoTestBot?start=tok123",
        expires_at: "2026-07-26T01:00:00Z",
      });
      const openSpy = vi.spyOn(window, "open").mockReturnValue(null);

      renderWithIntl();
      fireEvent.click(await screen.findByRole("button", { name: "Telegramni bog'lash" }));

      await waitFor(() =>
        expect(webApp.openTelegramLink).toHaveBeenCalledWith("https://t.me/AvtoTestBot?start=tok123"),
      );
      expect(openSpy).not.toHaveBeenCalled();
    });
  });
});
