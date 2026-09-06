import type { MetadataRoute } from "next";
import { SITE_URL } from "@/lib/seo";

// Auth-gated surfaces (dashboard, practice, exam, admin, station kiosk, …) are
// crawlable but marked noindex via each route group's layout metadata — see
// src/app/[locale]/(app)/layout.tsx and friends. Disallowing them here instead
// would stop Googlebot from ever seeing that noindex tag and can backfire into
// a bare, snippet-less index entry. /api/ has no HTML/meta to read, so blocking
// the crawl outright is safe and saves crawl budget.
export default function robots(): MetadataRoute.Robots {
  return {
    rules: [{ userAgent: "*", allow: "/", disallow: ["/api/"] }],
    sitemap: `${SITE_URL}/sitemap.xml`,
    host: SITE_URL,
  };
}
