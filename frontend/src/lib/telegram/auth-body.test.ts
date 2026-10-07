import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { afterTelegramAuth, withTelegramInitData } from "./auth-body";
import type { TelegramWebApp } from "./web-app";

const cloudRemove = vi.fn();
vi.mock("./web-app", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./web-app")>();
  return { ...actual, cloudRemove: (key: string) => cloudRemove(key) };
});

beforeEach(() => cloudRemove.mockReset().mockResolvedValue(undefined));
afterEach(() => vi.unstubAllGlobals());

describe("withTelegramInitData", () => {
  it("leaves the website body untouched", () => {
    expect(withTelegramInitData({ phone: "901234567" }, null)).toEqual({ phone: "901234567" });
    expect(withTelegramInitData({ phone: "901234567" }, null, "signed-contact")).toEqual({ phone: "901234567" });
  });
  it("adds the signed launch data inside Telegram", () => {
    const webApp = { initData: "signed" } as TelegramWebApp;
    expect(withTelegramInitData({ phone: "901234567" }, webApp)).toEqual({ phone: "901234567", tg_init_data: "signed" });
  });
  it("adds the signed contact next to the launch data when the phone was shared", () => {
    const webApp = { initData: "signed" } as TelegramWebApp;
    expect(withTelegramInitData({ phone: "901234567" }, webApp, "contact=x")).toEqual({
      phone: "901234567",
      tg_init_data: "signed",
      tg_contact: "contact=x",
    });
  });
  it("adds nothing when Telegram gave no launch data", () => {
    const webApp = { initData: "" } as TelegramWebApp;
    expect(withTelegramInitData({ phone: "9" }, webApp, "contact=x")).toEqual({ phone: "9" });
  });
});

type ContactCb = (shared: boolean, res?: { response?: string }) => void;

function webAppSharing(answer: ((cb: ContactCb) => void) | null): TelegramWebApp {
  return {
    initData: "signed-init",
    isVersionAtLeast: () => true,
    requestContact: answer ? (cb: ContactCb) => answer(cb) : undefined,
  } as unknown as TelegramWebApp;
}

function linkReply(linked: boolean) {
  return vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { linked } }), { status: 200 }));
}

describe("afterTelegramAuth", () => {
  it("clears the auto-login opt-out once linked", async () => {
    await afterTelegramAuth(true);
    expect(cloudRemove).toHaveBeenCalledWith("autologin_off");
  });
  it("does nothing when linking was skipped and no follow-up is wanted", async () => {
    await afterTelegramAuth(false);
    expect(cloudRemove).not.toHaveBeenCalled();
  });

  // A learner who typed their number is asked once, through Telegram's own
  // sheet, for the signed number that the link needs.
  it("asks Telegram for the number once and links with the signed response", async () => {
    const fetchMock = linkReply(true);
    vi.stubGlobal("fetch", fetchMock);
    const requestContact = vi.fn((cb: ContactCb) => cb(true, { response: "contact=signed" }));
    await afterTelegramAuth(false, { webApp: webAppSharing(requestContact), askForContact: true });
    expect(requestContact).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/proxy/me/telegram/link-webapp");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ init_data: "signed-init", contact: "contact=signed" });
    expect(cloudRemove).toHaveBeenCalledWith("autologin_off");
  });
  it("keeps auto-login off when the server does not accept the proof", async () => {
    vi.stubGlobal("fetch", linkReply(false));
    await afterTelegramAuth(false, { webApp: webAppSharing((cb) => cb(true, { response: "contact=x" })), askForContact: true });
    expect(cloudRemove).not.toHaveBeenCalled();
  });
  it("is quiet when the learner declines the sheet", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    await afterTelegramAuth(false, { webApp: webAppSharing((cb) => cb(false)), askForContact: true });
    expect(fetchMock).not.toHaveBeenCalled();
    expect(cloudRemove).not.toHaveBeenCalled();
  });
  it("does not ask when told not to, or on a client without requestContact", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const requestContact = vi.fn();
    await afterTelegramAuth(false, { webApp: webAppSharing(requestContact), askForContact: false });
    await afterTelegramAuth(false, { webApp: webAppSharing(null), askForContact: true });
    await afterTelegramAuth(false, { webApp: null, askForContact: true });
    expect(requestContact).not.toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
  });
  it("swallows a throwing SDK and a failing request", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    await expect(
      afterTelegramAuth(false, {
        webApp: webAppSharing(() => {
          throw new Error("WebAppContactRequested");
        }),
        askForContact: true,
      }),
    ).resolves.toBeUndefined();
    await expect(
      afterTelegramAuth(false, { webApp: webAppSharing((cb) => cb(true, { response: "c" })), askForContact: true }),
    ).resolves.toBeUndefined();
    expect(cloudRemove).not.toHaveBeenCalled();
  });
});
