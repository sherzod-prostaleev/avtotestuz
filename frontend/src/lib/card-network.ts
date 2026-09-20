export type CardNetwork = "uzcard" | "humo";

/** `^\d{16}$` — the same length `ValidatePayoutCard` enforces. */
export const CARD_NUMBER_LENGTH = 16;

export function cardDigits(raw: string): string {
  return raw.replace(/\D/g, "");
}

/**
 * The payout endpoint's own rule, mirrored.
 *
 * `billing.DetectCardNetwork` reads the first two digits — 98 is Humo, 86 is
 * Uzcard — and returns nothing for anything else. `ValidatePayoutCard` then
 * accepts a card it cannot place as long as the caller names a network, and
 * refuses only a prefix that contradicts the name.
 *
 * The phone form used to insist on exactly 8600 or 9860 and refuse everything
 * else before sending anything, so a card the desktop form had already paid
 * out to was rejected out of hand. A client may be stricter than a server about
 * typos; it may not be stricter about which cards exist.
 */
export function detectCardNetwork(raw: string): CardNetwork | null {
  const digits = cardDigits(raw);
  if (digits.length < 4) return null;
  if (digits.startsWith("98")) return "humo";
  if (digits.startsWith("86")) return "uzcard";
  return null;
}

/** True once the number is long enough for the prefix to mean anything. */
export function canJudgeCardNetwork(raw: string): boolean {
  return cardDigits(raw).length >= 4;
}
