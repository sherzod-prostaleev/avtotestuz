import type { TelegramWebApp } from "./web-app";

/** The part of GET me/telegram that says WHICH Telegram account is linked. */
export interface LinkedTelegram {
  tg_user_id?: number;
  username?: string;
}

function normalize(username: string | undefined | null): string | null {
  const value = username?.trim().replace(/^@/, "").toLowerCase();
  return value ? value : null;
}

/**
 * Whether the profile's linked Telegram account is the one that opened the
 * Mini App. The linked tg_user_id decides when the API sends it (ids never
 * change; usernames are optional and can). Only a response without it falls
 * back to usernames (case-insensitive). Anything we cannot tell counts as not
 * the same account: the learner is then offered the normal link card rather
 * than a false "linked".
 */
export function isLinkedToCurrentUser(
  linked: LinkedTelegram | null | undefined,
  webApp: Pick<TelegramWebApp, "initDataUnsafe"> | null,
): boolean {
  if (!linked) return false;
  const currentUser = webApp?.initDataUnsafe?.user;
  if (typeof linked.tg_user_id === "number" && linked.tg_user_id > 0) {
    return currentUser?.id === linked.tg_user_id;
  }
  const linkedName = normalize(linked.username);
  return linkedName !== null && linkedName === normalize(currentUser?.username);
}
