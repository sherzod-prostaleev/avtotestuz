import type { TelegramWebApp } from "./web-app";

function normalize(username: string | undefined | null): string | null {
  const value = username?.trim().replace(/^@/, "").toLowerCase();
  return value ? value : null;
}

/**
 * Whether the profile's linked Telegram account is the one that opened the
 * Mini App. GET me/telegram exposes only the linked username, so that is what
 * is compared (Telegram usernames are case-insensitive). A missing username on
 * either side means "cannot tell" and counts as not the same account: the
 * learner is then offered the normal link card rather than a false "linked".
 */
export function isLinkedToCurrentUser(
  linkedUsername: string | undefined,
  webApp: Pick<TelegramWebApp, "initDataUnsafe"> | null,
): boolean {
  const linked = normalize(linkedUsername);
  const current = normalize(webApp?.initDataUnsafe?.user?.username);
  return linked !== null && linked === current;
}
