import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { locales, type Locale } from "@/i18n/config";
import { canonicalUrl, buildLanguageAlternates } from "@/lib/seo";
import OfertaPage from "./oferta-client";

// CMS-backed legal body fetched client-side ("./oferta-client");
// generateMetadata only runs in a server component.
export async function generateMetadata({ params }: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  if (!locales.includes(locale as Locale)) return {};
  const title = await getTranslations({ locale, namespace: "Legal" }).then((t) => t("ofertaTitle"));
  return {
    title,
    alternates: {
      canonical: canonicalUrl(locale as Locale, "oferta"),
      languages: buildLanguageAlternates("oferta"),
    },
  };
}

export default function Page() {
  return <OfertaPage />;
}
