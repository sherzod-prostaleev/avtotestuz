"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { LegalCmsBody } from "@/components/legal/legal-cms-body";
import { LegalDocShell, LegalSection } from "@/components/legal/legal-doc-shell";
import { apiGet } from "@/lib/api-client";
import { legalBodyOrEmpty, type PublicSiteLegal } from "@/lib/site-legal";

export default function PrivacyPage() {
  const t = useTranslations("Legal");
  const tLanding = useTranslations("Landing");
  const locale = useLocale();
  const home = `/${locale}`;
  const [cmsBody, setCmsBody] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void apiGet<PublicSiteLegal>(`site/legal?locale=${encodeURIComponent(locale)}`)
      .then((data) => {
        if (cancelled) return;
        setCmsBody(legalBodyOrEmpty(data.privacy));
      })
      .catch(() => {
        if (!cancelled) setCmsBody("");
      });
    return () => {
      cancelled = true;
    };
  }, [locale]);

  const useCms = Boolean(cmsBody);

  return (
    <LegalDocShell
      brandName={tLanding("brandName")}
      title={t("privacyTitle")}
      updatedLabel={t("updated")}
      backHref={home}
      backLabel={t("backHome")}
    >
      {cmsBody === null ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : useCms ? (
        <LegalCmsBody body={cmsBody} />
      ) : (
        <>
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
        </>
      )}
    </LegalDocShell>
  );
}
