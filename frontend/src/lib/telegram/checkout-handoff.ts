/**
 * Mini App payment hand-off. Payme/Click open through Telegram's openLink
 * after an awaited checkout call, which some clients treat as a popup without
 * a user gesture and drop; the URL is kept for this webview's lifetime so the
 * pending screen can reopen it from a real tap.
 */
export const CHECKOUT_URL_KEY = "tg-checkout-url";

/**
 * A hosted Payme/Click page is only worth reopening for a short while: the
 * payer either finished or abandoned it, and an old URL would send them to a
 * dead or already-settled payment. 30 minutes comfortably outlasts a card
 * entry plus the bank's SMS confirmation.
 */
export const CHECKOUT_URL_MAX_AGE_MS = 30 * 60 * 1000;

/** Sent with POST me/checkout from the Mini App; see backend checkoutBody. */
export const CHECKOUT_RETURN_CONTEXT = "telegram";

function httpUrl(value: string | null): string | null {
  if (!value) return null;
  try {
    const url = new URL(value);
    return url.protocol === "https:" || url.protocol === "http:" ? url.href : null;
  } catch {
    return null;
  }
}

export function rememberCheckoutUrl(url: string): void {
  const safe = httpUrl(url);
  if (!safe) return;
  try {
    sessionStorage.setItem(CHECKOUT_URL_KEY, JSON.stringify({ url: safe, at: Date.now() }));
  } catch {
    /* storage blocked: the pending screen simply offers no reopen button */
  }
}

export function readCheckoutUrl(): string | null {
  try {
    const raw = sessionStorage.getItem(CHECKOUT_URL_KEY);
    if (!raw) return null;
    const entry: unknown = JSON.parse(raw);
    if (typeof entry !== "object" || entry === null) return null;
    const { url, at } = entry as { url?: unknown; at?: unknown };
    if (typeof url !== "string" || typeof at !== "number") return null;
    const age = Date.now() - at;
    if (age < 0 || age > CHECKOUT_URL_MAX_AGE_MS) {
      forgetCheckoutUrl();
      return null;
    }
    return httpUrl(url);
  } catch {
    // Storage blocked or a value this code did not write.
    return null;
  }
}

export function forgetCheckoutUrl(): void {
  try {
    sessionStorage.removeItem(CHECKOUT_URL_KEY);
  } catch {
    /* storage blocked: nothing was stored */
  }
}
