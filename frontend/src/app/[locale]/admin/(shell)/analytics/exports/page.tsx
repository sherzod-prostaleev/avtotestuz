"use client";

import { useTranslations } from "next-intl";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { PermissionGate } from "@/components/admin/permission-gate";

/** export_job table not migrated — no async export jobs yet. */
export default function AdminAnalyticsExportsPage() {
  const t = useTranslations("AdminAnalyticsExports");
  const tNav = useTranslations("AdminNav");
  const tShell = useTranslations("AdminShell");

  return (
    <PermissionGate permission="analytics.export">
      <main className="mx-auto max-w-2xl space-y-5">
        <AdminPageHeader badge={tNav("groupAnalytics")} title={t("title")} description={t("subtitle")} />
        <AdminEmptyState title={tShell("soonTitle")} description={t("honest")} />
      </main>
    </PermissionGate>
  );
}
