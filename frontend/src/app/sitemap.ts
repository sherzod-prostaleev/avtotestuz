import type { MetadataRoute } from "next";
import { locales, defaultLocale } from "@/i18n/config";
import { SITE_URL, buildLanguageAlternates } from "@/lib/seo";

// Only server-rendered, login-free marketing pages belong here. (app)/(auth)/
// (kiosk)/(session)/admin surfaces are noindex (see robots.ts comment) and add
// nothing for a crawler to rank on.
const PUBLIC_PATHS: Array<{
  path: string;
  priority: number;
  changeFrequency: NonNullable<MetadataRoute.Sitemap[number]["changeFrequency"]>;
}> = [
  { path: "", priority: 1.0, changeFrequency: "weekly" },
  { path: "diagnostic", priority: 0.8, changeFrequency: "monthly" },
  { path: "narxlar", priority: 0.7, changeFrequency: "monthly" },
  { path: "maktab", priority: 0.6, changeFrequency: "monthly" },
  { path: "jarimalar", priority: 0.6, changeFrequency: "monthly" },
  { path: "privacy", priority: 0.2, changeFrequency: "yearly" },
  { path: "oferta", priority: 0.2, changeFrequency: "yearly" },
];

export default function sitemap(): MetadataRoute.Sitemap {
  const lastModified = new Date();
  const entries: MetadataRoute.Sitemap = [];

  for (const { path, priority, changeFrequency } of PUBLIC_PATHS) {
    for (const locale of locales) {
      entries.push({
        url: `${SITE_URL}/${locale}${path ? `/${path}` : ""}`,
        lastModified,
        changeFrequency,
        priority: locale === defaultLocale ? priority : Number((priority * 0.9).toFixed(2)),
        alternates: { languages: buildLanguageAlternates(path) },
      });
    }
  }

  return entries;
}
