"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { GraduationCap } from "lucide-react";
import { apiGet } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";

type OrgSummary = { id: string; name: string };

export function TeacherPortalCard() {
  const t = useTranslations("Teacher");
  const locale = useLocale();
  const [count, setCount] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    void apiGet<OrgSummary[]>("me/teacher/orgs")
      .then((rows) => {
        if (!cancelled) setCount(Array.isArray(rows) ? rows.length : 0);
      })
      .catch(() => {
        if (!cancelled) setCount(0);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (!count) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <GraduationCap className="h-4 w-4" aria-hidden />
          {t("profileCardTitle")}
        </CardTitle>
      </CardHeader>
      <div className="space-y-2 px-6 pb-6">
        <p className="text-sm text-muted-foreground">{t("profileCardBody", { count })}</p>
        <Link
          href={`/${locale}/teacher`}
          className="inline-flex text-sm font-semibold text-accent hover:underline"
        >
          {t("profileCardCta")}
        </Link>
      </div>
    </Card>
  );
}
