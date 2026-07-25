"use client";

import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { LegalDocShell, LegalSection } from "@/components/legal/legal-doc-shell";

export default function JarimalarPage() {
  const t = useTranslations("Jarimalar");
  const tLanding = useTranslations("Landing");
  const locale = useLocale();
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
