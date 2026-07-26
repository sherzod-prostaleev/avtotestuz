"use client";

import { useTranslations } from "next-intl";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { PermissionGate } from "@/components/admin/permission-gate";

/** Client events lack visit→OTP→paywall→paid chain — no fake funnel. */
export default function AdminAnalyticsFunnelsPage() {
  const t = useTranslations("AdminAnalyticsFunnels");
  const tNav = useTranslations("AdminNav");
  const tShell = useTranslations("AdminShell");

  return (
    <PermissionGate permission="analytics.read">
      <main className="mx-auto max-w-2xl space-y-5">
        <AdminPageHeader badge={tNav("groupAnalytics")} title={t("title")} description={t("subtitle")} />
        <AdminEmptyState title={tShell("soonTitle")} description={t("honest")} />
      </main>
    </PermissionGate>
  );
}
