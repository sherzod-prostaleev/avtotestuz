import { afterEach, describe, expect, it, vi } from "vitest";
import { initSentry, isSentryEnabled, resetSentryForTests, sentryDsn } from "./sentry";

afterEach(() => {
  resetSentryForTests();
  vi.unstubAllEnvs();
});

describe("sentry optional init", () => {
  it("treats empty DSN as disabled no-op", async () => {
    vi.stubEnv("NEXT_PUBLIC_SENTRY_DSN", "");
    expect(sentryDsn()).toBe("");
    expect(isSentryEnabled()).toBe(false);
    await expect(initSentry()).resolves.toBe(false);
  });

  it("trims whitespace-only DSN to disabled", async () => {
    vi.stubEnv("NEXT_PUBLIC_SENTRY_DSN", "   ");
    expect(isSentryEnabled()).toBe(false);
    await expect(initSentry()).resolves.toBe(false);
  });

  it("reports enabled when DSN is set (SDK init smoke)", async () => {
    vi.stubEnv("NEXT_PUBLIC_SENTRY_DSN", "https://publickey@127.0.0.1/1");
    expect(isSentryEnabled()).toBe(true);
    // Real @sentry/browser Init against a loopback DSN — no pager, no network required for init.
    await expect(initSentry()).resolves.toBe(true);
    // Second call is a no-op guard.
    await expect(initSentry()).resolves.toBe(true);
  });
});
