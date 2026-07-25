"use client";

import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";

/** Refunds UI: money path is Payme inbound CancelTransaction (U-04); no admin initiate. */
export default function AdminPaymentsRefundsPage() {
  const t = useTranslations("AdminPayments");
  const locale = useLocale();

  return (
    <main className="mx-auto max-w-2xl space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("refundsTitle")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("refundsSubtitle")}</p>
      </header>
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
  );
}
