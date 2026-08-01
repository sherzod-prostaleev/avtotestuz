import { getTranslations } from "next-intl/server";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { PermissionGate } from "@/components/admin/permission-gate";

/** Tariffs are seed/DB managed; no admin CRUD API (GET /tariffs is public read-only). */
export default async function AdminPaymentsCatalogPage() {
  const [t, tNav, tShell] = await Promise.all([
    getTranslations("AdminPaymentsCatalog"),
    getTranslations("AdminNav"),
    getTranslations("AdminShell"),
  ]);

  return (
    <PermissionGate permission="payments.read">
      <main className="mx-auto max-w-2xl space-y-5">
        <AdminPageHeader badge={tNav("groupPayments")} title={t("title")} description={t("subtitle")} />
        <AdminEmptyState title={tShell("soonTitle")} description={t("honest")} />
      </main>
    </PermissionGate>
  );
}
