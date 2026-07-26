"use client";

import { useTranslations } from "next-intl";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { PermissionGate } from "@/components/admin/permission-gate";

/** Tariffs are seed/DB managed; no admin CRUD API (GET /tariffs is public read-only). */
export default function AdminPaymentsCatalogPage() {
  const t = useTranslations("AdminPaymentsCatalog");
  const tNav = useTranslations("AdminNav");
  const tShell = useTranslations("AdminShell");

  return (
    <PermissionGate permission="payments.read">
      <main className="mx-auto max-w-2xl space-y-5">
        <AdminPageHeader badge={tNav("groupPayments")} title={t("title")} description={t("subtitle")} />
        <AdminEmptyState title={tShell("soonTitle")} description={t("honest")} />
      </main>
    </PermissionGate>
  );
}
