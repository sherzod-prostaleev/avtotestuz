import { afterEach, describe, expect, it } from "vitest";
import { TELEGRAM_BOOT_SCRIPT, earlyTelegramScheme } from "./boot-script";
import { TELEGRAM_SDK_URL } from "./web-app";
import { installTelegramHost, removeTelegramHost } from "@/test/telegram-host";

function launchHash(bg: string): string {
  return (
    "#tgWebAppData=" +
    encodeURIComponent("user=%7B%22id%22%3A1%7D&hash=00") +
    "&tgWebAppThemeParams=" +
    encodeURIComponent(JSON.stringify({ bg_color: bg }))
  );
}

// Exactly what the inline <script> in /tg does.
function run() {
  new Function(TELEGRAM_BOOT_SCRIPT)();
}

afterEach(() => {
  removeTelegramHost();
  document.head.querySelectorAll("script").forEach((s) => s.remove());
  const root = document.documentElement;
  root.className = "";
  root.removeAttribute("data-tg-scheme");
  root.style.colorScheme = "";
  window.history.replaceState(null, "", "/");
});

describe("Telegram boot script", () => {
  it("sets the light class from a light Telegram theme before React runs", () => {
    installTelegramHost();
    document.documentElement.classList.add("dark");
    window.history.replaceState(null, "", "/uz-Latn/tg" + launchHash("#ffffff"));
    run();
    expect(document.documentElement.classList.contains("light")).toBe(true);
    expect(document.documentElement.classList.contains("dark")).toBe(false);
    expect(earlyTelegramScheme()).toBe("light");
  });

  it("keeps dark for a dark Telegram theme", () => {
    installTelegramHost();
    window.history.replaceState(null, "", "/uz-Latn/tg" + launchHash("#17212b"));
    run();
    expect(earlyTelegramScheme()).toBe("dark");
  });

  it("starts the SDK download once", () => {
    installTelegramHost();
    window.history.replaceState(null, "", "/uz-Latn/tg" + launchHash("#ffffff"));
    run();
    run();
    expect(document.querySelectorAll(`script[src="${TELEGRAM_SDK_URL}"]`)).toHaveLength(1);
  });

  // C1: a planted launch hash in a plain browser must change nothing.
  it("does nothing without a Telegram host", () => {
    window.history.replaceState(null, "", "/uz-Latn/tg" + launchHash("#ffffff"));
    run();
    expect(document.querySelector("script[src]")).toBeNull();
    expect(earlyTelegramScheme()).toBeNull();
  });

  it("does nothing inside a host without launch data", () => {
    installTelegramHost();
    window.history.replaceState(null, "", "/uz-Latn/tg");
    run();
    expect(document.querySelector("script[src]")).toBeNull();
    expect(earlyTelegramScheme()).toBeNull();
  });

  it("ignores a malformed theme but still starts the SDK", () => {
    installTelegramHost();
    window.history.replaceState(null, "", "/uz-Latn/tg#tgWebAppData=x&tgWebAppThemeParams=%7Bnot-json");
    run();
    expect(document.querySelectorAll(`script[src="${TELEGRAM_SDK_URL}"]`)).toHaveLength(1);
    expect(earlyTelegramScheme()).toBeNull();
  });
});
