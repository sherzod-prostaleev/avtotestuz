import { AUTOLOGIN_OFF_KEY, cloudRemove, type TelegramWebApp } from "./web-app";

/** Inside the Mini App, phone sign-in also links this Telegram account. */
export function withTelegramInitData<T extends object>(body: T, webApp: TelegramWebApp | null): T & { tg_init_data?: string } {
  return webApp?.initData ? { ...body, tg_init_data: webApp.initData } : body;
}

/** A successful link means auto-login should work again next launch. */
export async function afterTelegramAuth(linked: boolean): Promise<void> {
  if (linked) await cloudRemove(AUTOLOGIN_OFF_KEY);
}
