"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { AdminBarChart } from "@/components/admin/admin-bar-chart";
import { Button } from "@/components/ui/button";

type NameCount = { name: string; count: number };
type DayCount = { day: string; count: number };

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
  signups_by_day_14d?: DayCount[];
  revenue_by_day_14d?: DayCount[];
  note: string;
};

function fmtUzs(n: number): string {
  return new Intl.NumberFormat("uz-UZ").format(n) + " UZS";
}

export default function AdminAnalyticsOverviewPage() {
  const t = useTranslations("AdminAnalytics");
  const locale = useLocale();
  const [data, setData] = useState<Overview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/analytics/overview", { cache: "no-store" });
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
        { label: t("tileProfiles24h"), value: String(data.profiles_created_24h) },
        { label: t("tileProfiles7d"), value: String(data.profiles_created_7d) },
        { label: t("tilePaid"), value: String(data.payments_paid_total) },
        { label: t("tilePaid7d"), value: String(data.payments_paid_7d) },
        { label: t("tileRevenue"), value: fmtUzs(data.revenue_paid_uzs_total) },
        { label: t("tileRevenue7d"), value: fmtUzs(data.revenue_paid_uzs_7d) },
        { label: t("tileEntitlements"), value: String(data.entitlements_active) },
        { label: t("tileEvents7d"), value: String(data.events_last_7d) },
      ]
    : [];

  return (
    <main className="mx-auto max-w-6xl space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="text-[11px] font-extrabold uppercase tracking-[0.18em] text-accent">
            {t("controlBadge")}
          </p>
          <h1 className="mt-1 font-display text-3xl font-black tracking-tight">{t("title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
          <p className="mt-2 text-xs text-muted-foreground">{t("note")}</p>
        </div>
        <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
          <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          {t("refresh")}
        </Button>
      </header>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!data ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : (
        <>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {tiles.map((tile) => (
              <div
                key={tile.label}
                className="rounded-2xl border border-border/80 bg-card/80 px-4 py-3"
              >
                <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                  {tile.label}
                </p>
                <p className="mt-1.5 font-display text-xl font-black tabular-nums">{tile.value}</p>
              </div>
            ))}
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <section className="rounded-2xl border border-border/80 bg-card/70 p-4">
              <h2 className="text-sm font-bold">{t("chartSignups")}</h2>
              <div className="mt-3">
                <AdminBarChart
                  points={(data.signups_by_day_14d ?? []).map((p) => ({
                    label: p.day,
                    value: p.count,
                  }))}
                  emptyLabel={t("chartEmpty")}
                />
              </div>
            </section>
            <section className="rounded-2xl border border-border/80 bg-card/70 p-4">
              <h2 className="text-sm font-bold">{t("chartRevenue")}</h2>
              <div className="mt-3">
                <AdminBarChart
                  points={(data.revenue_by_day_14d ?? []).map((p) => ({
                    label: p.day,
                    value: p.count,
                  }))}
                  emptyLabel={t("chartEmpty")}
                  valueFormatter={fmtUzs}
                />
              </div>
            </section>
          </div>

          <section className="space-y-2">
            <h2 className="text-sm font-bold">{t("eventsTitle")}</h2>
            {data.top_event_names_7d.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("eventsEmpty")}</p>
            ) : (
              <ul className="space-y-2">
                {data.top_event_names_7d.map((row) => {
                  const max = Math.max(1, ...data.top_event_names_7d.map((r) => r.count));
                  const pct = Math.round((row.count / max) * 100);
                  return (
                    <li
                      key={row.name}
                      className="rounded-xl border border-border/70 bg-card/60 px-3 py-2 text-sm"
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className="font-mono text-xs sm:text-sm">{row.name}</span>
                        <span className="font-bold tabular-nums">{row.count}</span>
                      </div>
                      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-border/60">
                        <div
                          className="h-full rounded-full bg-accent"
                          style={{ width: `${pct}%` }}
                        />
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          {data.note ? <p className="text-xs text-muted-foreground">{data.note}</p> : null}
          <p className="text-xs text-muted-foreground">
            {t("generatedAt", { time: new Date(data.generated_at).toLocaleString(locale) })}
          </p>
        </>
      )}
    </main>
  );
}
