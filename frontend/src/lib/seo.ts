import { locales, defaultLocale, type Locale } from "@/i18n/config";

export const SITE_URL = "https://drivergo.uz";
export const SITE_NAME = "Driver Go";

/** BCP-47 tags Google expects in hreflang; next-intl's URL segments (uz-Latn/uz-Cyrl/ru) aren't valid hreflang on their own. */
const HREFLANG_BY_LOCALE: Record<Locale, string> = {
  "uz-Latn": "uz-Latn-UZ",
  "uz-Cyrl": "uz-Cyrl-UZ",
  ru: "ru-UZ",
};

/** Absolute canonical URL for a locale + optional path (no leading slash needed). */
export function canonicalUrl(locale: Locale, path = ""): string {
  const clean = path.replace(/^\/+/, "");
  return `${SITE_URL}/${locale}${clean ? `/${clean}` : ""}`;
}

/** `alternates.languages` map for a given path, shared across every locale variant + x-default. */
export function buildLanguageAlternates(path = ""): Record<string, string> {
  const entries: Record<string, string> = {};
  for (const locale of locales) {
    entries[HREFLANG_BY_LOCALE[locale]] = canonicalUrl(locale, path);
  }
  entries["x-default"] = canonicalUrl(defaultLocale, path);
  return entries;
}
