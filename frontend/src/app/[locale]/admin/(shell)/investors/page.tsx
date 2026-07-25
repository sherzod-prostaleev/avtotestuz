"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

type NameCount = { name: string; count: number };

type Overview = {
  generated_at: string;
  profiles_total: number;
  profiles_created_24h: number;
  profiles_created_7d: number;
  payments_paid_total: number;
  payments_paid_7d: number;
  revenue_paid_uzs_total: number;
  revenue_paid_uzs_7d: number;
  entitlements_active: number;
  events_last_7d: number;
  top_event_names_7d: NameCount[];
  note: string;
};

function fmtUzs(n: number): string {
  return new Intl.NumberFormat("uz-UZ").format(n) + " UZS";
}

export default function AdminInvestorsPage() {
  const t = useTranslations("AdminInvestors");
  const [data, setData] = useState<Overview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/investors/overview", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setData(null);
        return;
      }
      setData(json.data as Overview);
    } catch {
      setError(t("errorLoad"));
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  const tiles = data
    ? [
        { label: t("tileProfiles"), value: String(data.profiles_total) },
        { label: t("tileProfiles7d"), value: String(data.profiles_created_7d) },
        { label: t("tilePaid"), value: String(data.payments_paid_total) },
        { label: t("tileRevenue"), value: fmtUzs(data.revenue_paid_uzs_total) },
        { label: t("tileRevenue7d"), value: fmtUzs(data.revenue_paid_uzs_7d) },
        { label: t("tileEntitlements"), value: String(data.entitlements_active) },
      ]
    : [];

  return (
    <main className="mx-auto max-w-3xl space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("note")}</p>
      </header>

      <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
        <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
        {t("refresh")}
      </Button>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!data ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : (
        <>
          <div className="grid gap-3 sm:grid-cols-3">
            {tiles.map((tile) => (
              <div key={tile.label} className="rounded-xl border border-border bg-card px-4 py-3">
                <p className="text-xs font-semibold text-muted-foreground">{tile.label}</p>
                <p className="mt-1 font-mono text-lg font-bold">{tile.value}</p>
              </div>
            ))}
          </div>
          {data.note ? <p className="text-xs text-muted-foreground">{data.note}</p> : null}
          <p className="text-xs text-muted-foreground">
            {t("generatedAt", { time: new Date(data.generated_at).toLocaleString() })}
          </p>
        </>
      )}
    </main>
  );
}
