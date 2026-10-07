import { beforeEach, describe, expect, it, vi } from "vitest";
import { afterTelegramAuth, withTelegramInitData } from "./auth-body";
import type { TelegramWebApp } from "./web-app";

const cloudRemove = vi.fn();
vi.mock("./web-app", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./web-app")>();
  return { ...actual, cloudRemove: (key: string) => cloudRemove(key) };
});

beforeEach(() => cloudRemove.mockReset().mockResolvedValue(undefined));

describe("withTelegramInitData", () => {
  it("leaves the website body untouched", () => {
    expect(withTelegramInitData({ phone: "901234567" }, null)).toEqual({ phone: "901234567" });
  });
  it("adds the signed launch data inside Telegram", () => {
    const webApp = { initData: "signed" } as TelegramWebApp;
    expect(withTelegramInitData({ phone: "901234567" }, webApp)).toEqual({ phone: "901234567", tg_init_data: "signed" });
  });
  it("adds nothing when Telegram gave no launch data", () => {
    const webApp = { initData: "" } as TelegramWebApp;
    expect(withTelegramInitData({ phone: "9" }, webApp)).toEqual({ phone: "9" });
  });
});

describe("afterTelegramAuth", () => {
  it("clears the auto-login opt-out once linked", async () => {
    await afterTelegramAuth(true);
    expect(cloudRemove).toHaveBeenCalledWith("autologin_off");
  });
  it("does nothing when linking was skipped", async () => {
    await afterTelegramAuth(false);
    expect(cloudRemove).not.toHaveBeenCalled();
  });
});
