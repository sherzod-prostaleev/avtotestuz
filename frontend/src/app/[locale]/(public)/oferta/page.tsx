"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { LegalCmsBody } from "@/components/legal/legal-cms-body";
import { LegalDocShell, LegalSection } from "@/components/legal/legal-doc-shell";
import { apiGet } from "@/lib/api-client";
import { legalBodyOrEmpty, type PublicSiteLegal } from "@/lib/site-legal";

export default function OfertaPage() {
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
        setCmsBody(legalBodyOrEmpty(data.oferta));
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
      title={t("ofertaTitle")}
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
        </>
      )}
    </LegalDocShell>
  );
}
