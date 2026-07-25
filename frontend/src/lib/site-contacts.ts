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
