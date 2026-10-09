/**
 * "Signed in as +998 90 ••• •• 67" — handed from the Telegram login to the
 * first learner screen, shown once.
 *
 * With «Telegram orqali kirish» the person at the screen is not necessarily
 * the one who approved in Telegram: the link is also a QR code on that screen
 * (a classroom, a shared PC). The first opener owns it, but the learner must
 * still see at a glance which account the browser ended up in. Only the
 * masked form ever reaches the page; anything else found in storage is dropped.
 */
const KEY = "drivergo:signedInAs";
const MASKED = /^\+998 \d{2} ••• •• \d{2}$/;

export function isMaskedPhone(value: unknown): value is string {
  return typeof value === "string" && MASKED.test(value);
}

export function rememberSignedInAs(maskedPhone: unknown): void {
  if (!isMaskedPhone(maskedPhone)) return;
  try {
    window.sessionStorage.setItem(KEY, maskedPhone);
  } catch {
    /* storage blocked: the notice is lost, the sign-in is not */
  }
}

/** Reads the pending notice and forgets it: it is shown once. */
export function takeSignedInAs(): string | null {
  try {
    const value = window.sessionStorage.getItem(KEY);
    window.sessionStorage.removeItem(KEY);
    return isMaskedPhone(value) ? value : null;
  } catch {
    return null;
  }
}
