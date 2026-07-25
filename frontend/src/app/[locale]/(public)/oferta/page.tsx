"use client";

import { useLocale, useTranslations } from "next-intl";
import { LegalDocShell, LegalSection } from "@/components/legal/legal-doc-shell";

export default function OfertaPage() {
  const t = useTranslations("Legal");
  const tLanding = useTranslations("Landing");
  const locale = useLocale();
  const home = `/${locale}`;

  return (
    <LegalDocShell
      brandName={tLanding("brandName")}
      title={t("ofertaTitle")}
      updatedLabel={t("updated")}
      backHref={home}
      backLabel={t("backHome")}
    >
      <LegalSection title={t("ofertaS1Title")}>
        <p>{t("ofertaS1Body")}</p>
      </LegalSection>
      <LegalSection title={t("ofertaS2Title")}>
        <p>{t("ofertaS2Body")}</p>
      </LegalSection>
      <LegalSection title={t("ofertaS3Title")}>
        <p>{t("ofertaS3Body")}</p>
      </LegalSection>
      <LegalSection title={t("ofertaS4Title")}>
        <p>{t("ofertaS4Body")}</p>
      </LegalSection>
      <LegalSection title={t("ofertaS5Title")}>
        <p>{t("ofertaS5Body")}</p>
      </LegalSection>
      <LegalSection title={t("ofertaS6Title")}>
        <p>{t("ofertaS6Body")}</p>
      </LegalSection>
    </LegalDocShell>
  );
}
