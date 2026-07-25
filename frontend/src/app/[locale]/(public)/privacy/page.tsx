"use client";

import { useLocale, useTranslations } from "next-intl";
import { LegalDocShell, LegalSection } from "@/components/legal/legal-doc-shell";

export default function PrivacyPage() {
  const t = useTranslations("Legal");
  const tLanding = useTranslations("Landing");
  const locale = useLocale();
  const home = `/${locale}`;

  return (
    <LegalDocShell
      brandName={tLanding("brandName")}
      title={t("privacyTitle")}
      updatedLabel={t("updated")}
      backHref={home}
      backLabel={t("backHome")}
    >
      <LegalSection title={t("privacyS1Title")}>
        <p>{t("privacyS1Body")}</p>
      </LegalSection>
      <LegalSection title={t("privacyS2Title")}>
        <p>{t("privacyS2Body")}</p>
      </LegalSection>
      <LegalSection title={t("privacyS3Title")}>
        <p>{t("privacyS3Body")}</p>
      </LegalSection>
      <LegalSection title={t("privacyS4Title")}>
        <p>{t("privacyS4Body")}</p>
      </LegalSection>
      <LegalSection title={t("privacyS5Title")}>
        <p>{t("privacyS5Body")}</p>
      </LegalSection>
      <LegalSection title={t("privacyS6Title")}>
        <p>{t("privacyS6Body")}</p>
      </LegalSection>
    </LegalDocShell>
  );
}
