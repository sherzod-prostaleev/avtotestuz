import { showTelegramHint } from "./hint";
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

/** Whether this client can show Telegram's phone-share sheet (Bot API 6.9+). */
export function canRequestContact(webApp: TelegramWebApp): boolean {
  if (typeof webApp.requestContact !== "function") return false;
  return typeof webApp.isVersionAtLeast !== "function" || webApp.isVersionAtLeast("6.9");
}

/**
 * Opens Telegram's own "share your phone number" sheet and resolves with the
 * signed `response` string (null when declined, unsupported or broken). The
 * SDK throws for clients older than 6.9 and while another request is open.
 */
export function requestSignedContact(webApp: TelegramWebApp): Promise<string | null> {
  if (!canRequestContact(webApp)) return Promise.resolve(null);
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

export type InAppLinkResult = "linked" | "declined" | "failed";

/**
 * Links the signed-in profile to the Telegram account running the Mini App:
 * Telegram's own phone-share sheet, then POST /me/telegram/link-webapp with
 * the signed contact. "declined" means the learner closed the sheet (or the
 * client cannot show it) — not an error to report; "failed" means the server
 * refused the proof (a number that is not the profile's) or the call failed.
 */
export async function linkTelegramInApp(webApp: TelegramWebApp): Promise<InAppLinkResult> {
  const contact = await requestSignedContact(webApp);
  if (!contact) return "declined";
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), LINK_TIMEOUT_MS);
  try {
    const res = await fetch("/api/proxy/me/telegram/link-webapp", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: webApp.initData, contact }),
      signal: controller.signal,
    });
    if (!res.ok) return "failed";
    const json = (await res.json().catch(() => null)) as { data?: { linked?: unknown } } | null;
    return json?.data?.linked === true ? "linked" : "failed";
  } catch {
    return "failed";
  } finally {
    clearTimeout(timer);
  }
}

/**
 * After a successful phone sign-in. A link means auto-login should work again
 * next launch. Without one, when `askForContact` (a Mini App sign-in where no
 * number was shared yet), Telegram's sheet is offered once and the signed
 * number linked through POST /me/telegram/link-webapp. `explain` is shown
 * first: by then the form is gone, and a sheet out of nowhere asking for the
 * phone number looks like phishing. Callers fire and forget this: it never
 * throws, and declining the sheet is not an error.
 */
export async function afterTelegramAuth(
  linked: boolean,
  followUp?: { webApp: TelegramWebApp | null; askForContact: boolean; explain?: string },
): Promise<void> {
  try {
    if (!linked) {
      const webApp = followUp?.webApp;
      if (!followUp?.askForContact || !webApp?.initData) return;
      if (!canRequestContact(webApp)) return;
      if (followUp.explain) showTelegramHint(followUp.explain);
      if ((await linkTelegramInApp(webApp)) !== "linked") return;
    }
    await cloudRemove(AUTOLOGIN_OFF_KEY);
  } catch {
    /* best-effort: the sign-in already succeeded */
  }
}
