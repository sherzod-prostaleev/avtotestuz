/** National 9-digit UZ mobile (strips optional 998 country code). */
export function normalizeNationalPhone(input: string): string {
  let digits = input.replace(/\D/g, "");
  if (digits.startsWith("998") && digits.length >= 12) {
    digits = digits.slice(3);
  }
  return digits.slice(0, 9);
}

/** Display grouping: 90 123 45 67 */
export function formatNationalPhone(digits: string): string {
  const d = digits.replace(/\D/g, "").slice(0, 9);
  const parts = [d.slice(0, 2), d.slice(2, 5), d.slice(5, 7), d.slice(7, 9)].filter(Boolean);
  return parts.join(" ");
}

/**
 * HTML maxLength for the national input after the +998 prefix.
 * 9 digits grouped as "90 123 45 67" is 12 characters — not 9.
 * A maxLength of 9 stops typing after 7 digits ("90 123 45").
 */
export const NATIONAL_PHONE_INPUT_MAX_LENGTH = formatNationalPhone("000000000").length;

export function parsePasswordResetTokenFromBotURL(botURL: string): string | null {
  try {
    const url = new URL(botURL);
    const start = url.searchParams.get("start") ?? "";
    if (!start.startsWith("pwr_") || start.length <= 4) return null;
    return start.slice(4);
  } catch {
    return null;
  }
}
