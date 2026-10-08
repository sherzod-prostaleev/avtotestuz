import { TELEGRAM_SDK_URL } from "./web-app";

/**
 * Runs inline in /tg's HTML, before hydration, and only when a real Telegram
 * client hosts the page AND the URL carries launch data (the same test as
 * hasTelegramHost + isTelegramMiniApp). Two things cannot wait for React:
 *
 * 1. The SDK request. Started here it overlaps with the JS bundle download
 *    instead of starting after hydration, so ready() (which hides Telegram's
 *    own loading placeholder) comes much sooner. TelegramProvider finds this
 *    <script> by its src and never injects a second one.
 * 2. The colour scheme. The site defaults to dark; a light-theme Telegram
 *    user saw a dark page until the SDK arrived. Telegram puts its theme in
 *    the launch hash, so the right class is set before the first paint and
 *    published as data-tg-scheme for TelegramThemeProvider's first render.
 *
 * Self-contained on purpose: it is serialised with Function#toString, so it
 * must not use imports or anything a compiler would hoist out of it. It never
 * writes storage: the Mini App must not change the website's saved theme.
 */
function telegramBoot(sdkUrl: string): void {
  try {
    const w = window as Window & { TelegramWebviewProxy?: unknown };
    const d = document;
    let host = w.TelegramWebviewProxy !== undefined;
    if (!host) {
      try {
        const ext = (w as { external?: unknown }).external;
        host = !!(ext && typeof ext === "object" && "notify" in (ext as object));
      } catch {
        host = false;
      }
    }
    if (!host) {
      try {
        host = w.parent != null && w.parent !== w;
      } catch {
        host = true;
      }
    }
    if (!host) return;
    const hash = w.location.hash.slice(1);
    if (hash.indexOf("tgWebAppData=") === -1) return;

    if (!d.querySelector('script[src="' + sdkUrl + '"]')) {
      const tag = d.createElement("script");
      tag.src = sdkUrl;
      tag.async = true;
      d.head.appendChild(tag);
    }

    const raw = new URLSearchParams(hash).get("tgWebAppThemeParams");
    if (!raw) return;
    const bg = (JSON.parse(raw) as { bg_color?: unknown }).bg_color;
    if (typeof bg !== "string" || !/^#[0-9a-f]{6}$/i.test(bg)) return;
    const r = parseInt(bg.slice(1, 3), 16);
    const g = parseInt(bg.slice(3, 5), 16);
    const b = parseInt(bg.slice(5, 7), 16);
    // telegram-web-app.js decides colorScheme with exactly this test.
    const scheme = Math.sqrt(0.299 * r * r + 0.587 * g * g + 0.114 * b * b) < 120 ? "dark" : "light";
    const root = d.documentElement;
    root.classList.remove("light", "dark");
    root.classList.add(scheme);
    root.style.colorScheme = scheme;
    root.setAttribute("data-tg-scheme", scheme);
  } catch {
    /* a malformed hash just means no head start */
  }
}

export const TELEGRAM_BOOT_SCRIPT = `(${telegramBoot.toString()})(${JSON.stringify(TELEGRAM_SDK_URL)});`;

/** The scheme the boot script found, or null (website, no launch theme). */
export function earlyTelegramScheme(): "light" | "dark" | null {
  if (typeof document === "undefined") return null;
  const value = document.documentElement.getAttribute("data-tg-scheme");
  return value === "light" || value === "dark" ? value : null;
}
