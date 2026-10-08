import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, describe, it, expect, vi, beforeEach } from "vitest";
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

    const label = await screen.findByText("Telegram bog'langan");
    expect(screen.getByText("Bog'langan akkaunt: @sherzod")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Qayta bog'lash" })).toBeInTheDocument();
    // Green text on the green tint read at ~3:1; the words are body text,
    // only the tick carries the colour.
    const box = label.closest('[role="status"]') as HTMLElement;
    expect(box.className).not.toMatch(/text-success/);
    expect(label.className).toMatch(/text-foreground/);
    expect(box.querySelector("svg")?.getAttribute("class")).toMatch(/text-success/);
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
    function enterMiniApp(username: string | null = "sherzod", share: "yes" | "no" = "yes") {
      const webApp = {
        initData: "signed-init",
        initDataUnsafe: { user: { id: 42, username: username ?? undefined } },
        isVersionAtLeast: () => true,
        requestContact: vi.fn((cb: (ok: boolean, res?: { response?: string }) => void) =>
          share === "yes" ? cb(true, { response: "contact=signed" }) : cb(false),
        ),
        openLink: vi.fn(),
        openTelegramLink: vi.fn(),
      };
      tg.webApp = webApp;
      installTelegramHost();
      (window as { Telegram?: unknown }).Telegram = { WebApp: webApp };
      return webApp;
    }

    function linkWebApp(linked: boolean) {
      const fetchMock = vi.fn(async () => new Response(JSON.stringify({ data: { linked } }), { status: 200 }));
      vi.stubGlobal("fetch", fetchMock);
      return fetchMock;
    }

    afterEach(() => vi.unstubAllGlobals());

    // The Mini App is itself the link; offering to (re)link from inside it
    // would only send the learner out to the bot and back.
    it("shows the linked status without link actions", async () => {
      enterMiniApp("Sherzod");
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "@sherzod", phone_verified: true });

      renderWithIntl();

      const label = await screen.findByText("Telegram bog'langan");
      // Audit-2 I8: the label is body text, only the tick is green.
      expect(label.className).not.toMatch(/text-success/);
      expect(screen.queryByRole("button", { name: "Qayta bog'lash" })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Telegramni bog'lash" })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Raqamni ulashib bog'lash" })).not.toBeInTheDocument();
      expect(screen.queryByText(/Havola 10 daqiqa/)).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Telegramni uzish" })).toBeInTheDocument();
    });

    // The linked Telegram id decides when the API sends it: usernames change
    // and can be missing, ids cannot.
    it("compares the linked tg_user_id, not usernames, when it is known", async () => {
      enterMiniApp("new_name");
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "old_name", tg_user_id: 42, phone_verified: true });
      renderWithIntl();
      expect(await screen.findByText("Telegram bog'langan")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Raqamni ulashib bog'lash" })).not.toBeInTheDocument();
    });

    // Audit-2 backend change: a link made before phone verification no longer
    // signs in automatically, so it must read as "confirm", not "linked".
    it.each([
      ["phone_verified is false", false],
      ["phone_verified is missing", undefined],
    ] as const)("asks to confirm the phone when %s", async (_name, verified) => {
      const webApp = enterMiniApp("sherzod");
      const get = vi
        .spyOn(apiClient, "apiGet")
        .mockResolvedValueOnce({ linked: true, username: "sherzod", tg_user_id: 42, phone_verified: verified })
        .mockResolvedValue({ linked: true, username: "sherzod", tg_user_id: 42, phone_verified: true });
      const fetchMock = linkWebApp(true);

      renderWithIntl();

      expect(await screen.findByText("Raqamingizni tasdiqlang")).toBeInTheDocument();
      expect(screen.queryByText("Telegram bog'langan")).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Raqamni ulashib bog'lash" }));
      await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/proxy/me/telegram/link-webapp", expect.anything()));
      expect(webApp.openTelegramLink).not.toHaveBeenCalled();
      expect(await screen.findByText("Telegram bog'langan")).toBeInTheDocument();
      expect(get).toHaveBeenCalledTimes(2);
    });

    it("says the account is linked to another Telegram and links this one in-app", async () => {
      const webApp = enterMiniApp("sherzod");
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "sherzod", tg_user_id: 7, phone_verified: true });
      const fetchMock = linkWebApp(true);
      renderWithIntl();
      expect(await screen.findByText("Boshqa Telegram akkauntiga bog'langan")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Qayta bog'lash" })).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Raqamni ulashib bog'lash" }));
      await waitFor(() => expect(fetchMock).toHaveBeenCalled());
      expect(webApp.openTelegramLink).not.toHaveBeenCalled();
    });

    it("links an unlinked learner in-app, never through a t.me deep link", async () => {
      const webApp = enterMiniApp();
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: false });
      const post = vi.spyOn(apiClient, "apiPost");
      const fetchMock = linkWebApp(true);
      renderWithIntl();
      expect(await screen.findByText("Hali Telegram akkaunt bog'lanmagan.")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Raqamni ulashib bog'lash" }));
      await waitFor(() => expect(fetchMock).toHaveBeenCalled());
      expect(webApp.requestContact).toHaveBeenCalledTimes(1);
      expect(post).not.toHaveBeenCalled();
      expect(webApp.openTelegramLink).not.toHaveBeenCalled();
    });

    it("says nothing when the learner closes Telegram's sheet", async () => {
      enterMiniApp("sherzod", "no");
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: false });
      const fetchMock = linkWebApp(true);
      renderWithIntl();
      fireEvent.click(await screen.findByRole("button", { name: "Raqamni ulashib bog'lash" }));
      await waitFor(() => expect(screen.getByRole("button", { name: "Raqamni ulashib bog'lash" })).toBeEnabled());
      expect(fetchMock).not.toHaveBeenCalled();
      expect(screen.queryByRole("alert")).toBeNull();
    });

    it("explains a refused link", async () => {
      enterMiniApp();
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: false });
      linkWebApp(false);
      renderWithIntl();
      fireEvent.click(await screen.findByRole("button", { name: "Raqamni ulashib bog'lash" }));
      expect(await screen.findByRole("alert")).toHaveTextContent(messages.TelegramLink.linkInAppError);
    });
  });

  describe("unlinking", () => {
    it("asks first, then unlinks and refreshes the status", async () => {
      const get = vi
        .spyOn(apiClient, "apiGet")
        .mockResolvedValueOnce({ linked: true, username: "sherzod", phone_verified: true })
        .mockResolvedValue({ linked: false });
      const del = vi.spyOn(apiClient, "apiDelete").mockResolvedValue({ unlinked: true });
      renderWithIntl();
      fireEvent.click(await screen.findByRole("button", { name: "Telegramni uzish" }));
      expect(del).not.toHaveBeenCalled();
      expect(screen.getByText(messages.TelegramLink.unlinkConfirm)).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Ha, uzish" }));
      await waitFor(() => expect(del).toHaveBeenCalledWith("me/telegram"));
      expect(await screen.findByText("Hali Telegram akkaunt bog'lanmagan.")).toBeInTheDocument();
      expect(get).toHaveBeenCalledTimes(2);
      expect(screen.queryByRole("button", { name: "Telegramni uzish" })).not.toBeInTheDocument();
    });

    it("can be cancelled", async () => {
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "sherzod", phone_verified: true });
      const del = vi.spyOn(apiClient, "apiDelete");
      renderWithIntl();
      fireEvent.click(await screen.findByRole("button", { name: "Telegramni uzish" }));
      fireEvent.click(screen.getByRole("button", { name: "Bekor qilish" }));
      expect(screen.queryByText(messages.TelegramLink.unlinkConfirm)).not.toBeInTheDocument();
      expect(del).not.toHaveBeenCalled();
    });

    it("reports a failed unlink", async () => {
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: true, username: "sherzod", phone_verified: true });
      vi.spyOn(apiClient, "apiDelete").mockRejectedValue(new Error("boom"));
      renderWithIntl();
      fireEvent.click(await screen.findByRole("button", { name: "Telegramni uzish" }));
      fireEvent.click(screen.getByRole("button", { name: "Ha, uzish" }));
      expect(await screen.findByRole("alert")).toHaveTextContent(messages.TelegramLink.unlinkError);
    });

    it("is not offered when nothing is linked", async () => {
      vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: false });
      renderWithIntl();
      expect(await screen.findByText("Hali Telegram akkaunt bog'lanmagan.")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Telegramni uzish" })).not.toBeInTheDocument();
    });
  });
});
