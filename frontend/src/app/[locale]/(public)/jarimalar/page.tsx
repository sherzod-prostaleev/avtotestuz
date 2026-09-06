import type { Metadata } from "next";
import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";
import { LegalDocShell, LegalSection } from "@/components/legal/legal-doc-shell";
import { locales, type Locale } from "@/i18n/config";
import { canonicalUrl, buildLanguageAlternates } from "@/lib/seo";

export async function generateMetadata({ params }: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  if (!locales.includes(locale as Locale)) return {};
  const t = await getTranslations({ locale, namespace: "Metadata.pages.jarimalar" });
  const title = t("title");
  const description = t("description");
  return {
    title,
    description,
    alternates: {
      canonical: canonicalUrl(locale as Locale, "jarimalar"),
      languages: buildLanguageAlternates("jarimalar"),
    },
    openGraph: { title, description },
    twitter: { title, description },
  };
}

export default async function JarimalarPage() {
  const [t, tLanding, locale] = await Promise.all([
    getTranslations("Jarimalar"),
    getTranslations("Landing"),
    getLocale(),
  ]);
  const home = `/${locale}`;

  return (
    <LegalDocShell
      brandName={tLanding("brandName")}
      title={t("title")}
      updatedLabel={t("updated")}
      backHref={home}
      backLabel={t("backHome")}
    >
      <LegalSection title={t("s1Title")}>
        <p>{t("s1Body")}</p>
      </LegalSection>
      <LegalSection title={t("s2Title")}>
        <p>{t("s2Body")}</p>
      </LegalSection>
      <LegalSection title={t("s3Title")}>
        <p>{t("s3Body")}</p>
      </LegalSection>
      <LegalSection title={t("s4Title")}>
        <p>{t("s4Body")}</p>
        <p className="pt-2">
          <Link
            href={`/${locale}/signs`}
            className="font-semibold text-accent underline-offset-4 hover:underline"
          >
            {t("ctaSigns")}
          </Link>
          {" · "}
          <Link
            href={`/${locale}/login`}
            className="font-semibold text-accent underline-offset-4 hover:underline"
          >
            {t("ctaPractice")}
          </Link>
        </p>
      </LegalSection>
    </LegalDocShell>
  );
}
