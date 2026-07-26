export type SiteHomeHero = {
  headline: string;
  subtitle: string;
  ctaLabel: string;
  ctaHref: string;
};

export const EMPTY_SITE_HOME: SiteHomeHero = {
  headline: "",
  subtitle: "",
  ctaLabel: "",
  ctaHref: "",
};

/** Prefer API value when non-empty; otherwise fall back (i18n placeholder). */
export function homeOrFallback(apiValue: string | undefined, fallback: string): string {
  const v = (apiValue ?? "").trim();
  return v || fallback;
}

const CMS_LOCALES = ["uz-Latn", "uz-Cyrl", "ru"] as const;

/**
 * Rewrite a CMS CTA path that hardcodes one locale prefix to the visitor's
 * locale (admin tip historically suggested `/uz-Latn/login`).
 */
export function localizeCmsHref(href: string, locale: string): string {
  const path = (href ?? "").trim();
  if (!path.startsWith("/")) return path;
  for (const loc of CMS_LOCALES) {
    if (path === `/${loc}`) return `/${locale}`;
    const prefix = `/${loc}/`;
    if (path.startsWith(prefix)) {
      return `/${locale}/${path.slice(prefix.length)}`;
    }
  }
  return path;
}
