"use client";

import { useTranslations } from "next-intl";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";

type ComingSoonProps = {
  path: string;
  phase?: string;
};

/** Honest stub — never fake data. */
export function ComingSoon({ path, phase }: ComingSoonProps) {
  const t = useTranslations("AdminShell");
  return (
    <main className="mx-auto max-w-2xl space-y-5">
      <AdminPageHeader badge={t("soonBadge")} title={t("soonTitle")} description={t("soonBody")} />
      <AdminEmptyState
        title={path}
        description={phase ? t("soonPhase", { phase }) : t("soonHonest")}
      />
    </main>
  );
}
