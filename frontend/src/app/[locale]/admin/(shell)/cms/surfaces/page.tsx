import { getTranslations } from "next-intl/server";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { PermissionGate } from "@/components/admin/permission-gate";

/** Popups / banners CMS — no admin API yet (PRD M3-4). */
export default async function AdminCMSSurfacesPage() {
  const [t, tNav, tShell] = await Promise.all([
    getTranslations("AdminCMSSurfaces"),
    getTranslations("AdminNav"),
    getTranslations("AdminShell"),
  ]);

  return (
    <PermissionGate permission="cms.read">
      <main className="mx-auto max-w-2xl space-y-5">
        <AdminPageHeader badge={tNav("groupCMS")} title={t("title")} description={t("subtitle")} />
        <AdminEmptyState title={tShell("soonTitle")} description={t("honest")} />
      </main>
    </PermissionGate>
  );
}
