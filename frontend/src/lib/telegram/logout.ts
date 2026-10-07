import { AUTOLOGIN_OFF_KEY, cloudSet, isTelegramMiniApp } from "./web-app";

/**
 * Signing out inside the Mini App keeps the Telegram link (bot digests and
 * password reset depend on it, spec D6) and instead switches auto-login off
 * for this Telegram user, so /tg shows the welcome screen next time. cloudSet
 * gives up after 3 s, so a silent bridge never holds the logout hostage.
 */
export async function markTelegramLogout(): Promise<void> {
  await cloudSet(AUTOLOGIN_OFF_KEY, "1");
}

/** Where to land after logout or session expiry. */
export function postLogoutPath(locale: string, fallback: string): string {
  return isTelegramMiniApp() ? `/${locale}/tg` : fallback;
}
