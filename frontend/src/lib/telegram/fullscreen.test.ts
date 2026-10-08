import { describe, expect, it, vi } from "vitest";
import { requestPhoneFullscreen } from "./fullscreen";
import type { TelegramWebApp } from "./web-app";

const app = (over: Partial<TelegramWebApp> & { atLeast?: string } = {}) => {
  const { atLeast = "8.0", ...rest } = over;
  return {
    platform: "android",
    isVersionAtLeast: (v: string) => Number(v) <= Number(atLeast),
    requestFullscreen: vi.fn(),
    isFullscreen: false,
    ...rest,
  } as unknown as TelegramWebApp & { requestFullscreen: ReturnType<typeof vi.fn> };
};

describe("requestPhoneFullscreen", () => {
  it.each(["android", "ios"])("asks for fullscreen on %s with Bot API 8.0+", (platform) => {
    const a = app({ platform });
    expect(requestPhoneFullscreen(a)).toBe(true);
    expect(a.requestFullscreen).toHaveBeenCalledTimes(1);
  });

  // Desktop and web clients would turn a window into a fullscreen takeover.
  it.each(["tdesktop", "macos", "web", "weba", "webk", "unknown", ""])("never on %s", (platform) => {
    const a = app({ platform });
    expect(requestPhoneFullscreen(a)).toBe(false);
    expect(a.requestFullscreen).not.toHaveBeenCalled();
  });

  it("skips clients older than Bot API 8.0", () => {
    const a = app({ atLeast: "7.10" });
    expect(requestPhoneFullscreen(a)).toBe(false);
    expect(a.requestFullscreen).not.toHaveBeenCalled();
  });

  it("does not ask again when already fullscreen", () => {
    const a = app({ isFullscreen: true });
    expect(requestPhoneFullscreen(a)).toBe(false);
    expect(a.requestFullscreen).not.toHaveBeenCalled();
  });

  it("tolerates a client without the method or one that throws", () => {
    expect(requestPhoneFullscreen(app({ requestFullscreen: undefined }))).toBe(false);
    const throwing = app({
      requestFullscreen: () => {
        throw new Error("WebAppMethodUnsupported");
      },
    });
    expect(requestPhoneFullscreen(throwing)).toBe(false);
  });
});
