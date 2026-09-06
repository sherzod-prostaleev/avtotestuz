import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { locales, type Locale } from "@/i18n/config";
import { canonicalUrl, buildLanguageAlternates } from "@/lib/seo";
import PrivacyPage from "./privacy-client";

// CMS-backed legal body fetched client-side ("./privacy-client");
// generateMetadata only runs in a server component.
export async function generateMetadata({ params }: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  if (!locales.includes(locale as Locale)) return {};
  const title = await getTranslations({ locale, namespace: "Legal" }).then((t) => t("privacyTitle"));
  return {
    title,
    alternates: {
      canonical: canonicalUrl(locale as Locale, "privacy"),
      languages: buildLanguageAlternates("privacy"),
    },
  };
}

export default function Page() {
  return <PrivacyPage />;
}
