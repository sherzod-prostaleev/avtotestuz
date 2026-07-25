"use client";

import { useCallback, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

type Finding = {
  severity: string;
  code: string;
  payment_id?: string;
  provider?: string;
  detail: string;
};

type ReconPayload = {
  dry_run: boolean;
  note: string;
  result: {
    from: string;
    to: string;
    scanned_payments: number;
    scanned_provider_txns: number;
    findings: Finding[];
  };
};

export default function AdminPaymentsReconPage() {
  const t = useTranslations("AdminRecon");
  const [data, setData] = useState<ReconPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [hours, setHours] = useState("24");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/payments/recon?hours=${encodeURIComponent(hours)}`, {
        cache: "no-store",
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setData(null);
        return;
      }
      setData(json.data as ReconPayload);
    } catch {
      setError(t("errorLoad"));
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [hours, t]);

  return (
    <main className="mx-auto max-w-2xl space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("note")}</p>
      </header>

      <div className="flex flex-wrap items-center gap-2">
        <label className="text-xs text-muted-foreground">
          {t("hours")}
          <input
            value={hours}
            onChange={(e) => setHours(e.target.value)}
            className="ml-2 h-9 w-20 rounded-lg border border-border bg-background px-2 text-sm"
          />
        </label>
        <Button type="button" size="sm" disabled={loading} onClick={() => void load()}>
          <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          {t("run")}
        </Button>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {data ? (
        <div className="space-y-3">
          <p className="text-xs text-muted-foreground">{data.note}</p>
          <p className="text-sm">
            {t("summary", {
              payments: data.result.scanned_payments,
              txns: data.result.scanned_provider_txns,
              findings: data.result.findings?.length ?? 0,
            })}
          </p>
          {(data.result.findings?.length ?? 0) === 0 ? (
            <p className="text-sm text-muted-foreground">{t("empty")}</p>
          ) : (
            <ul className="space-y-2">
              {data.result.findings.map((f, i) => (
                <li key={`${f.code}-${f.payment_id ?? i}`} className="rounded-xl border border-border bg-card px-4 py-3 text-sm">
                  <p className="font-bold">
                    [{f.severity}] {f.code}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {f.provider} {f.payment_id}
                  </p>
                  <p className="mt-1">{f.detail}</p>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">{t("idle")}</p>
      )}
    </main>
  );
}
