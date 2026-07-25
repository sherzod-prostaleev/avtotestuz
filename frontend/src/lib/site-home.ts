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
