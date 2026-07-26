"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { Button } from "@/components/ui/button";
import { PermissionGate } from "@/components/admin/permission-gate";

type PayoutRow = {
  id: string;
  profile_id: string;
  profile_phone: string;
  profile_name: string;
  amount_uzs: number;
  card_number: string;
  card_masked: string;
  card_network: string;
  status: string;
  admin_note: string;
  created_at: string;
};

function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

export default function AdminReferralPayoutsPage() {
  const t = useTranslations("AdminReferral");
  const [status, setStatus] = useState("pending");
  const [items, setItems] = useState<PayoutRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const qs = status ? `?status=${encodeURIComponent(status)}` : "";
      const res = await fetch(`/api/admin/referral/payouts${qs}`, { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(t("errorLoad"));
        setItems([]);
        return;
      }
      setItems((json.data?.items ?? []) as PayoutRow[]);
    } catch {
      setError(t("errorLoad"));
    } finally {
      setLoading(false);
    }
  }, [status, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function act(id: string, action: "paid" | "reject") {
    setBusyId(id);
    try {
      const res = await fetch(`/api/admin/referral/payouts/${id}/${action}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ note: "" }),
      });
      if (!res.ok) {
        setError(t("errorAction"));
        return;
      }
      await load();
    } catch {
      setError(t("errorAction"));
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="space-y-6">
      <AdminPageHeader title={t("title")} description={t("subtitle")} />

      <div className="flex flex-wrap gap-2">
        {(["pending", "paid", "rejected", ""] as const).map((s) => (
          <Button
            key={s || "all"}
            type="button"
            size="sm"
            variant={status === s ? "game" : "outline"}
            onClick={() => setStatus(s)}
          >
            {s === "" ? t("filterAll") : t(`filter_${s}` as "filter_pending")}
          </Button>
        ))}
      </div>

      {loading && <AdminSkeleton rows={6} />}
      {error && <AdminErrorState message={error} onRetry={() => void load()} />}

      {!loading && !error && (
        <div className="overflow-x-auto rounded-xl border border-border">
          <table className="w-full text-left text-sm">
            <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
              <tr>
                <th className="px-3 py-2">{t("colUser")}</th>
                <th className="px-3 py-2">{t("colAmount")}</th>
                <th className="px-3 py-2">{t("colCard")}</th>
                <th className="px-3 py-2">{t("colStatus")}</th>
                <th className="px-3 py-2">{t("colCreated")}</th>
                <th className="px-3 py-2">{t("colActions")}</th>
              </tr>
            </thead>
            <tbody>
              {items.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-3 py-8 text-center text-muted-foreground">
                    {t("empty")}
                  </td>
                </tr>
              )}
              {items.map((row) => (
                <tr key={row.id} className="border-t border-border">
                  <td className="px-3 py-2">
                    <div className="font-medium">{row.profile_name || "—"}</div>
                    <div className="text-xs text-muted-foreground">{row.profile_phone}</div>
                  </td>
                  <td className="px-3 py-2 font-mono">{formatSom(row.amount_uzs)}</td>
                  <td className="px-3 py-2">
                    <div className="font-mono text-xs">{row.card_number}</div>
                    <div className="text-[11px] uppercase text-muted-foreground">{row.card_network}</div>
                  </td>
                  <td className="px-3 py-2">{row.status}</td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">
                    {new Date(row.created_at).toLocaleString()}
                  </td>
                  <td className="px-3 py-2">
                    {row.status === "pending" && (
                      <PermissionGate permission="referral.payouts.manage" mode="hide">
                        <div className="flex gap-2">
                          <Button
                            type="button"
                            size="sm"
                            variant="success"
                            disabled={busyId === row.id}
                            onClick={() => void act(row.id, "paid")}
                          >
                            {t("markPaid")}
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            variant="outline"
                            disabled={busyId === row.id}
                            onClick={() => void act(row.id, "reject")}
                          >
                            {t("reject")}
                          </Button>
                        </div>
                      </PermissionGate>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
