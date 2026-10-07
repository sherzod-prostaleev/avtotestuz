import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import { MobileTelegram } from "../mobile-telegram";
import * as apiClient from "@/lib/api-client";
import { installTelegramHost } from "@/test/telegram-host";

const tg = vi.hoisted(() => ({ webApp: null as null | Record<string, unknown> }));
vi.mock("@/components/telegram/telegram-provider", () => ({ useTelegram: () => tg.webApp }));

function renderPanel() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <MobileTelegram onBack={() => {}} />
    </NextIntlClientProvider>,
  );
}

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
    enterMiniApp("Sherzod");
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "@sherzod" });

    renderPanel();

    expect(await screen.findByText("Telegram ulangan")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Qayta bog'lash" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Telegramni bog'lash" })).not.toBeInTheDocument();
  });

  // The linked Telegram id decides when the API sends it: usernames change
  // and can be missing, ids cannot.
  it("compares the linked tg_user_id, not usernames, when it is known", async () => {
    enterMiniApp("new_name");
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "old_name", tg_user_id: 42 });
    renderPanel();
    expect(await screen.findByText("Telegram ulangan")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Qayta bog'lash" })).not.toBeInTheDocument();
  });

  it("offers relinking for another tg_user_id even with the same username", async () => {
    enterMiniApp("sherzod");
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "sherzod", tg_user_id: 7 });
    renderPanel();
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

    renderPanel();

    fireEvent.click(await screen.findByRole("button", { name: "Qayta bog'lash" }));
    expect(screen.queryByText("Telegram ulangan")).not.toBeInTheDocument();
    await waitFor(() =>
      expect(webApp.openTelegramLink).toHaveBeenCalledWith("https://t.me/AvtoTestBot?start=tok123"),
    );
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
