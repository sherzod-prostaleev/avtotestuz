import { afterEach, describe, expect, it, vi } from "vitest";
import { configuredBotUsername } from "./bot-username";

afterEach(() => vi.unstubAllEnvs());

describe("configuredBotUsername", () => {
  it("returns the configured bot, without a leading @", () => {
    vi.stubEnv("TELEGRAM_BOT_USERNAME", " @DriverGouzBot ");
    expect(configuredBotUsername()).toBe("DriverGouzBot");
  });
  it("is null when unset", () => {
    vi.stubEnv("TELEGRAM_BOT_USERNAME", "");
    expect(configuredBotUsername()).toBeNull();
  });
  it("is null for a value that is not a Telegram username", () => {
    vi.stubEnv("TELEGRAM_BOT_USERNAME", "evil.com/x");
    expect(configuredBotUsername()).toBeNull();
  });
});
