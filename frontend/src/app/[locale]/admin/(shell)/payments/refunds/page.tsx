"use client";

import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { PermissionGate } from "@/components/admin/permission-gate";

/** Refunds UI: money path is Payme inbound CancelTransaction (U-04); no admin initiate. */
export default function AdminPaymentsRefundsPage() {
  const t = useTranslations("AdminPayments");
  const tNav = useTranslations("AdminNav");
  const locale = useLocale();

  return (
    <PermissionGate permission="payments.read">
      <main className="mx-auto max-w-2xl space-y-5">
        <AdminPageHeader
          badge={tNav("groupPayments")}
          title={t("refundsTitle")}
          description={t("refundsSubtitle")}
        />
        <div className="space-y-3 rounded-xl border border-border bg-card p-4 text-sm leading-6 text-muted-foreground">
          <p>{t("refundHonest")}</p>
          <p>{t("refundsClick")}</p>
          <p>
            <Link
              href={`/${locale}/admin/payments/transactions?status=refunded`}
              className="font-semibold text-accent hover:underline"
            >
              {t("refundsLink")}
            </Link>
          </p>
        </div>
      </main>
    </PermissionGate>
  );
}
