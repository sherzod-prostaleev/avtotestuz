/**
 * The telegram_login kill switch as the website sees it (public flags
 * snapshot). Resolves false only on an explicit "off": an unreachable or
 * older API leaves the button in place, and the start call then decides
 * (503 telegram_login_disabled → the "use phone + password" copy).
 */
export async function fetchTelegramLoginEnabled(): Promise<boolean> {
  try {
    const res = await fetch("/api/proxy/flags");
    if (!res.ok) return true;
    const json = (await res.json().catch(() => null)) as { data?: { telegram_login?: unknown } } | null;
    return json?.data?.telegram_login !== false;
  } catch {
    return true;
  }
}
