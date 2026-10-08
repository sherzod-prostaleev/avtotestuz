/**
 * The /tg sign-in verdict "this Telegram account needs a phone sign-in",
 * remembered for the webview's lifetime. Back from /login to /tg remounts the
 * entry; without this every Back spent another rate-limited sign-in call on an
 * answer that cannot have changed. Keyed by the launching Telegram user id so
 * a different account in the same webview never reuses it. Cleared by any
 * successful sign-in and by logout (forgetNeedPhone).
 */
const PREFIX = "tg-need-phone:";

function keyFor(tgUserId: number | undefined): string | null {
  return typeof tgUserId === "number" && tgUserId > 0 ? `${PREFIX}${tgUserId}` : null;
}

export function rememberNeedPhone(tgUserId: number | undefined, firstName: string): void {
  const key = keyFor(tgUserId);
  if (!key) return;
  try {
    sessionStorage.setItem(key, JSON.stringify({ firstName }));
  } catch {
    /* storage blocked: /tg simply asks the server again */
  }
}

export function recallNeedPhone(tgUserId: number | undefined): { firstName: string } | null {
  const key = keyFor(tgUserId);
  if (!key) return null;
  try {
    const raw = sessionStorage.getItem(key);
    if (raw === null) return null;
    const parsed = JSON.parse(raw) as { firstName?: unknown };
    return typeof parsed?.firstName === "string" ? { firstName: parsed.firstName } : null;
  } catch {
    return null;
  }
}

export function forgetNeedPhone(): void {
  try {
    const stale: string[] = [];
    for (let i = 0; i < sessionStorage.length; i++) {
      const key = sessionStorage.key(i);
      if (key?.startsWith(PREFIX)) stale.push(key);
    }
    stale.forEach((key) => sessionStorage.removeItem(key));
  } catch {
    /* storage blocked: nothing was remembered either */
  }
}
