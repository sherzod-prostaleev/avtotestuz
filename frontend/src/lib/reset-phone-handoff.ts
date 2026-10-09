import { normalizeNationalPhone } from "@/lib/phone-format";

/**
 * /login → /forgot-password hand-off of the number the learner already typed
 * («Parol o'rnatish» on a passwordless account). It travels in sessionStorage,
 * never in the URL: a `?phone=` ends up in access logs, browser history and
 * Referer headers. Read once, then gone.
 */
const KEY = "drivergo:resetPhone";

export function rememberResetPhone(phone: string): void {
  const national = normalizeNationalPhone(phone);
  if (national.length !== 9) return;
  try {
    window.sessionStorage.setItem(KEY, national);
  } catch {
    /* storage blocked: the learner types the number again */
  }
}

export function takeResetPhone(): string | null {
  try {
    const value = window.sessionStorage.getItem(KEY);
    window.sessionStorage.removeItem(KEY);
    const national = normalizeNationalPhone(value ?? "");
    return national.length === 9 ? national : null;
  } catch {
    return null;
  }
}
