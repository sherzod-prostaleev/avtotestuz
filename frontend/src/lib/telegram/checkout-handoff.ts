/**
 * Mini App payment hand-off. Payme/Click open through Telegram's openLink
 * after an awaited checkout call, which some clients treat as a popup without
 * a user gesture and drop; the URL is kept for this webview's lifetime so the
 * pending screen can reopen it from a real tap.
 */
export const CHECKOUT_URL_KEY = "tg-checkout-url";

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
    sessionStorage.setItem(CHECKOUT_URL_KEY, safe);
  } catch {
    /* storage blocked: the pending screen simply offers no reopen button */
  }
}

export function readCheckoutUrl(): string | null {
  try {
    return httpUrl(sessionStorage.getItem(CHECKOUT_URL_KEY));
  } catch {
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
