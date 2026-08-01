import { getTranslations } from "next-intl/server";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { PermissionGate } from "@/components/admin/permission-gate";

/** No admin webhook inbox API — inbound Payme/Click only, no persisted audit table. */
export default async function AdminPaymentsWebhooksPage() {
  const [t, tNav, tShell] = await Promise.all([
    getTranslations("AdminPaymentsWebhooks"),
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
