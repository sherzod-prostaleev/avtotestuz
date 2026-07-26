export type SiteContacts = {
  phone: string;
  phoneTel: string;
  email: string;
  address: string;
  hours: string;
  telegram: string;
  telegramUrl: string;
  instagram: string;
  instagramUrl: string;
};

export const EMPTY_SITE_CONTACTS: SiteContacts = {
  phone: "",
  phoneTel: "",
  email: "",
  address: "",
  hours: "",
  telegram: "",
  telegramUrl: "",
  instagram: "",
  instagramUrl: "",
};

/** Prefer API value when non-empty; otherwise fall back (i18n placeholder). */
export function contactOrFallback(apiValue: string | undefined, fallback: string): string {
  const v = (apiValue ?? "").trim();
  return v || fallback;
}

/** Digits + optional leading + for tel: hrefs. */
export function compactPhoneTel(display: string): string {
  const trimmed = display.trim();
  let out = "";
  for (let i = 0; i < trimmed.length; i++) {
    const ch = trimmed[i];
    if (ch >= "0" && ch <= "9") out += ch;
    else if (ch === "+" && i === 0) out += ch;
  }
  return out;
}

/**
 * Resolve display phone + tel: href with cross-fill.
 * Admin often saves only one of the two fields; empty means i18n fallback
 * only when *both* CMS fields are blank.
 */
export function resolvePhonePair(
  phone: string | undefined,
  phoneTel: string | undefined,
  phoneFallback: string,
  phoneTelFallback: string,
): { phone: string; phoneTel: string } {
  const display = (phone ?? "").trim();
  const tel = (phoneTel ?? "").trim();
  if (display && tel) return { phone: display, phoneTel: tel };
  if (display && !tel) {
    const compact = compactPhoneTel(display);
    return { phone: display, phoneTel: compact || phoneTelFallback };
  }
  if (!display && tel) return { phone: tel, phoneTel: tel };
  return { phone: phoneFallback, phoneTel: phoneTelFallback };
}
