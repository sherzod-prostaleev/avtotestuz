// Telegram's own username rule. Anything that ends up in a t.me link is
// checked against it, whatever the source.
export const BOT_USERNAME = /^[A-Za-z0-9_]{5,32}$/;

/**
 * Our bot's username from the server environment (TELEGRAM_BOT_USERNAME),
 * read per request so one image serves any deployment. Null when missing or
 * malformed: callers then show no bot link rather than a guessed one.
 * Server-only: the variable is not exposed to the client bundle.
 */
export function configuredBotUsername(): string | null {
  const configured = (process.env.TELEGRAM_BOT_USERNAME ?? "").trim().replace(/^@/, "");
  return BOT_USERNAME.test(configured) ? configured : null;
}
