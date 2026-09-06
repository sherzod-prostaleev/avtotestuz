import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { locales, type Locale } from "@/i18n/config";
import { canonicalUrl, buildLanguageAlternates } from "@/lib/seo";
import PublicDiagnosticPage from "./diagnostic-client";

// Interactive quiz needs browser state, so it stays a client component
// ("./diagnostic-client"); generateMetadata only runs in a server component.
export async function generateMetadata({ params }: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  if (!locales.includes(locale as Locale)) return {};
  const t = await getTranslations({ locale, namespace: "Metadata.pages.diagnostic" });
  const title = t("title");
  const description = t("description");
  return {
    title,
    description,
    alternates: {
      canonical: canonicalUrl(locale as Locale, "diagnostic"),
      languages: buildLanguageAlternates("diagnostic"),
    },
    openGraph: { title, description },
    twitter: { title, description },
  };
}

export default function Page() {
  return <PublicDiagnosticPage />;
}
