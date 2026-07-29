"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { PermissionGate } from "@/components/admin/permission-gate";

type PaymentDetail = {
  id: string;
  profile_id: string;
  phone_masked: string;
  tariff_code: string;
  tariff_days_snapshot: number;
  tariff_price_uzs_snapshot: number;
  amount_uzs: number;
  provider: string;
  status: string;
  provider_txn_id?: string;
  idempotency_key: string;
  meta: Record<string, unknown>;
  created_at: string;
  paid_at?: string;
  entitlement?: {
    id: string;
    starts_at: string;
    ends_at: string;
    active: boolean;
    note?: string;
  };
  payme?: {
    payme_id: string;
    state: number;
    reason?: number;
    create_time: number;
    perform_time: number;
    cancel_time: number;
  };
  click?: {
    click_trans_id: string;
    click_paydoc_id?: string;
    state: number;
    reason?: string;
  };
  refund: {
    action_available: boolean;
    provider_path: string;
    note: string;
  };
};

export default function AdminPaymentDetailPage({ params }: { params: { id: string } }) {
  const t = useTranslations("AdminPayments");
  const tNav = useTranslations("AdminNav");
  const locale = useLocale();
  const { id } = params;
  const [data, setData] = useState<PaymentDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/payments/transactions/${id}`, { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        const code = json?.error?.code;
        setError(
          code === "not_found"
            ? t("errorNotFound")
            : code === "forbidden"
              ? t("errorForbidden")
              : t("errorLoad"),
        );
        setData(null);
        return;
      }
      setData(json.data as PaymentDetail);
    } catch {
      setError(t("errorLoad"));
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [id, t]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <PermissionGate permission="payments.read">
      <main className="mx-auto max-w-3xl space-y-5">
        <Link
          href={`/${locale}/admin/payments/transactions`}
          className="back-link inline-flex items-center gap-1 text-sm"
        >
          <ArrowLeft aria-hidden className="h-4 w-4" /> {t("back")}
        </Link>

        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load()} />
        ) : loading || !data ? (
          <p className="text-sm text-muted-foreground">{t("loading")}</p>
        ) : (
          <>
            <AdminPageHeader badge={tNav("groupPayments")} title={t("detailTitle")} />
            <p className="-mt-2 font-mono text-xs text-muted-foreground">{data.id}</p>

            <dl className="grid gap-3 rounded-xl border border-border bg-card p-4 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-xs text-muted-foreground">{t("colPhone")}</dt>
                <dd className="font-mono">{data.phone_masked}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("colProvider")}</dt>
                <dd>{data.provider}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("colStatus")}</dt>
                <dd className="font-bold uppercase">{data.status}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("colAmount")}</dt>
                <dd className="tabular-nums">{data.amount_uzs.toLocaleString(locale)} UZS</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("colTariff")}</dt>
                <dd>
                  {data.tariff_code} · {data.tariff_days_snapshot}d
                </dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("colCreated")}</dt>
                <dd>{new Date(data.created_at).toLocaleString(locale)}</dd>
              </div>
              <div className="sm:col-span-2">
                <dt className="text-xs text-muted-foreground">{t("colIdempotency")}</dt>
                <dd className="break-all font-mono text-xs">{data.idempotency_key}</dd>
              </div>
              {data.provider_txn_id ? (
                <div className="sm:col-span-2">
                  <dt className="text-xs text-muted-foreground">{t("colProviderTxn")}</dt>
                  <dd className="break-all font-mono text-xs">{data.provider_txn_id}</dd>
                </div>
              ) : null}
            </dl>

            {data.entitlement ? (
              <section className="rounded-xl border border-border bg-card p-4 text-sm">
                <h2 className="font-bold">{t("entitlementTitle")}</h2>
                <p className="mt-1 text-muted-foreground">
                  {data.entitlement.active ? t("entitlementActive") : t("entitlementInactive")} ·{" "}
                  {new Date(data.entitlement.starts_at).toLocaleString(locale)} →{" "}
                  {new Date(data.entitlement.ends_at).toLocaleString(locale)}
                </p>
              </section>
            ) : null}

            {data.payme ? (
              <section className="rounded-xl border border-border bg-card p-4 text-sm">
                <h2 className="font-bold">{t("paymeTitle")}</h2>
                <p className="mt-1 font-mono text-xs">
                  {data.payme.payme_id} · state={data.payme.state}
                </p>
              </section>
            ) : null}

            {data.click ? (
              <section className="rounded-xl border border-border bg-card p-4 text-sm">
                <h2 className="font-bold">{t("clickTitle")}</h2>
                <p className="mt-1 font-mono text-xs">
                  {data.click.click_trans_id} · state={data.click.state}
                </p>
              </section>
            ) : null}

            <section className="rounded-xl border border-border bg-card p-4 text-sm">
              <h2 className="font-bold">{t("refundTitle")}</h2>
              <p className="mt-1 text-muted-foreground">{data.refund.note}</p>
              <p className="mt-2 text-xs text-muted-foreground">
                {t("refundPath")}: <code>{data.refund.provider_path}</code> ·{" "}
                {data.refund.action_available ? t("refundAvailable") : t("refundUnavailable")}
              </p>
            </section>

            <section className="rounded-xl border border-border bg-card p-4 text-sm">
              <h2 className="font-bold">{t("metaTitle")}</h2>
              <pre className="mt-2 overflow-x-auto text-xs text-muted-foreground">
                {JSON.stringify(data.meta, null, 2)}
              </pre>
            </section>

            <p className="text-xs text-muted-foreground">
              <Link href={`/${locale}/admin/users/${data.profile_id}`} className="text-accent-ink hover:underline">
                {t("openUser")}
              </Link>
            </p>
          </>
        )}
      </main>
    </PermissionGate>
  );
}
