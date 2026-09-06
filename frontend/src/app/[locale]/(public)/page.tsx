import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { locales, type Locale } from "@/i18n/config";
import { canonicalUrl, buildLanguageAlternates } from "@/lib/seo";
import LandingPage from "./home-client";

// The landing page fetches CMS data client-side (site/contacts, site/home),
// so it stays a client component ("./home-client"); generateMetadata only
// runs in a server component, hence this thin wrapper.
export { clearLandingCacheForTests } from "./home-client";

export async function generateMetadata({ params }: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  if (!locales.includes(locale as Locale)) return {};
  const t = await getTranslations({ locale, namespace: "Metadata.pages.home" });
  const title = t("title");
  const description = t("description");
  return {
    title,
    description,
    alternates: {
      canonical: canonicalUrl(locale as Locale),
      languages: buildLanguageAlternates(),
    },
    openGraph: { title, description },
    twitter: { title, description },
  };
}

export default function Page() {
  return <LandingPage />;
}
