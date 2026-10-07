import { AUTOLOGIN_OFF_KEY, cloudRemove, type TelegramWebApp } from "./web-app";

/**
 * Inside the Mini App, phone sign-in also asks to link this Telegram account.
 * The server links only with `tg_contact` — Telegram's signed share of the
 * phone — matching the profile's phone; launch data alone links nothing.
 */
export function withTelegramInitData<T extends object>(
  body: T,
  webApp: TelegramWebApp | null,
  signedContact?: string | null,
): T & { tg_init_data?: string; tg_contact?: string } {
  if (!webApp?.initData) return body;
  return signedContact
    ? { ...body, tg_init_data: webApp.initData, tg_contact: signedContact }
    : { ...body, tg_init_data: webApp.initData };
}

/**
 * Opens Telegram's own "share your phone number" sheet and resolves with the
 * signed `response` string (null when declined, unsupported or broken). The
 * SDK throws for clients older than 6.9 and while another request is open.
 */
export function requestSignedContact(webApp: TelegramWebApp): Promise<string | null> {
  if (typeof webApp.requestContact !== "function") return Promise.resolve(null);
  if (typeof webApp.isVersionAtLeast === "function" && !webApp.isVersionAtLeast("6.9")) return Promise.resolve(null);
  return new Promise((resolve) => {
    try {
      webApp.requestContact((shared, res) => {
        const response = res?.response;
        resolve(shared && typeof response === "string" && response !== "" ? response : null);
      });
    } catch {
      resolve(null);
    }
  });
}

// Same budget as the /tg sign-in call: a stalled request must end.
const LINK_TIMEOUT_MS = 15_000;

async function linkWithSignedContact(webApp: TelegramWebApp): Promise<boolean> {
  const contact = await requestSignedContact(webApp);
  if (!contact) return false;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), LINK_TIMEOUT_MS);
  try {
    const res = await fetch("/api/proxy/me/telegram/link-webapp", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: webApp.initData, contact }),
      signal: controller.signal,
    });
    if (!res.ok) return false;
    const json = (await res.json().catch(() => null)) as { data?: { linked?: unknown } } | null;
    return json?.data?.linked === true;
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}

/**
 * After a successful phone sign-in. A link means auto-login should work again
 * next launch. Without one, when `askForContact` (a Mini App sign-in where no
 * number was shared yet), Telegram's sheet is offered once and the signed
 * number linked through POST /me/telegram/link-webapp. Callers fire and
 * forget this: it never throws, and declining the sheet is not an error.
 */
export async function afterTelegramAuth(
  linked: boolean,
  followUp?: { webApp: TelegramWebApp | null; askForContact: boolean },
): Promise<void> {
  try {
    if (!linked) {
      const webApp = followUp?.webApp;
      if (!followUp?.askForContact || !webApp?.initData) return;
      if (!(await linkWithSignedContact(webApp))) return;
    }
    await cloudRemove(AUTOLOGIN_OFF_KEY);
  } catch {
    /* best-effort: the sign-in already succeeded */
  }
}
