import { getWebApp } from "./web-app";

const TELEGRAM_HOSTS = new Set(["t.me", "telegram.me", "www.t.me", "www.telegram.me"]);

/**
 * How the Mini App should open `href`: "telegram" (t.me — Telegram opens it
 * natively, without leaving the app), "external" (any other http(s) origin —
 * Telegram's in-app browser) or null (same-origin, mailto:, tel:, tg:,
 * javascript:, unparsable — the webview handles those itself or not at all).
 */
export function telegramLinkKind(href: string, base: string): "telegram" | "external" | null {
  let url: URL;
  let here: URL;
  try {
    url = new URL(href, base);
    here = new URL(base);
  } catch {
    return null;
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") return null;
  if (url.origin === here.origin) return null;
  return TELEGRAM_HOSTS.has(url.hostname) ? "telegram" : "external";
}

/**
 * Opens an absolute external URL in a new context. Inside Telegram,
 * window.open from the webview is unreliable (blocked or swallowed on some
 * clients), so the SDK's openers are used; the website keeps window.open.
 */
export function openExternalUrl(url: string): void {
  const webApp = getWebApp();
  if (webApp) {
    const kind = telegramLinkKind(url, window.location.href);
    if (kind === "telegram") {
      webApp.openTelegramLink(url);
      return;
    }
    if (kind === "external") {
      webApp.openLink(url);
      return;
    }
  }
  window.open(url, "_blank", "noopener,noreferrer");
}
