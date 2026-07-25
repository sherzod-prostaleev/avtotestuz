"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Activity, ArrowLeft, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { OpsNav } from "@/components/ops/ops-nav";

type ProbeResult = {
  ok: boolean;
  status: number;
  data: unknown;
  error?: string;
};

type HealthPayload = {
  status: string;
  checked_at: string;
  live: ProbeResult;
  ready: ProbeResult;
  metrics?: ProbeResult;
};

function checkMap(data: unknown): Record<string, string> | null {
  if (!data || typeof data !== "object") return null;
  const inner = (data as { data?: unknown }).data;
  if (!inner || typeof inner !== "object") return null;
  const checks = (inner as { checks?: unknown }).checks;
  if (!checks || typeof checks !== "object") return null;
  return checks as Record<string, string>;
}

function metricsSnapshot(data: unknown): {
  uptime_seconds?: number;
  requests_total?: number;
  requests_by_status_class?: Record<string, number>;
} | null {
  if (!data || typeof data !== "object") return null;
  const inner = (data as { data?: unknown }).data;
  if (!inner || typeof inner !== "object") return null;
  return inner as {
    uptime_seconds?: number;
    requests_total?: number;
    requests_by_status_class?: Record<string, number>;
  };
}

function probeStatus(ok: boolean, okLabel: string, failLabel: string): string {
  return ok ? okLabel : failLabel;
}

export default function OpsHealthPage() {
  const t = useTranslations("OpsHealth");
  const tProviders = useTranslations("OpsProviders");
  const tUsers = useTranslations("OpsUsers");
  const tPayments = useTranslations("OpsPayments");
  const tAudit = useTranslations("OpsAudit");
  const tLimits = useTranslations("OpsLimits");
  const tContacts = useTranslations("OpsContacts");
  const locale = useLocale();
  const [payload, setPayload] = useState<HealthPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [auto, setAuto] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/ops/health", { cache: "no-store" });
      const json = await res.json();
      if (!json?.data) {
        setError(t("errorLoad"));
        setPayload(null);
        return;
      }
      setPayload(json.data as HealthPayload);
      if (!res.ok) {
        setError(t("errorDegraded"));
      }
    } catch {
      setError(t("errorLoad"));
      setPayload(null);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!auto) return;
    const id = window.setInterval(() => {
      void load();
    }, 10_000);
    return () => window.clearInterval(id);
  }, [auto, load]);

  const readyChecks = payload ? checkMap(payload.ready.data) : null;
  const metrics = payload?.metrics ? metricsSnapshot(payload.metrics.data) : null;

  return (
    <main className="page-shell-tight mx-auto max-w-lg">
      <header className="mb-4">
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("back")}
        </Link>
        <div className="mt-3 inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
          <Activity aria-hidden="true" className="h-4 w-4" />
          {t("eyebrow")}
        </div>
        <h1 className="mt-2 font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-2 text-sm">
          <Link
            href={`/${locale}/admin/monitoring/health`}
            className="font-semibold text-accent underline-offset-2 hover:underline"
          >
            {t("adminHomeLink")}
          </Link>
        </p>
      </header>

      <OpsNav
        locale={locale}
        active="health"
        labels={{
          health: t("navHealth"),
          providers: tProviders("navProviders"),
          contacts: tContacts("navContacts"),
          users: tUsers("navUsers"),
          payments: tPayments("navPayments"),
          audit: tAudit("navAudit"),
          limits: tLimits("navLimits"),
        }}
      />

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
          <RefreshCw aria-hidden="true" className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          {loading ? t("refreshing") : t("refresh")}
        </Button>
        <label className="inline-flex items-center gap-2 text-xs text-muted-foreground">
          <input
            type="checkbox"
            checked={auto}
            onChange={(e) => setAuto(e.target.checked)}
            className="rounded border-border"
          />
          {t("autoRefresh")}
        </label>
        {payload?.checked_at && (
          <span className="text-xs text-muted-foreground">
            {t("lastChecked", { time: new Date(payload.checked_at).toLocaleTimeString() })}
          </span>
        )}
      </div>

      {error && (
        <p role="alert" className="mb-4 rounded-xl border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </p>
      )}

      {payload && (
        <ul className="space-y-3">
          <li className="rounded-2xl border border-border bg-card p-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="font-display text-lg font-bold">{t("liveTitle")}</p>
                <p className="text-xs text-muted-foreground">{t("liveHint")}</p>
              </div>
              <span
                className={`rounded-lg px-2.5 py-1 text-xs font-bold ${
                  payload.live.ok ? "bg-accent/15 text-accent" : "bg-destructive/15 text-destructive"
                }`}
              >
                {probeStatus(payload.live.ok, t("statusOk"), t("statusFail"))}
              </span>
            </div>
            <p className="mt-2 font-mono text-xs text-muted-foreground">HTTP {payload.live.status}</p>
          </li>

          <li className="rounded-2xl border border-border bg-card p-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="font-display text-lg font-bold">{t("readyTitle")}</p>
                <p className="text-xs text-muted-foreground">{t("readyHint")}</p>
              </div>
              <span
                className={`rounded-lg px-2.5 py-1 text-xs font-bold ${
                  payload.ready.ok ? "bg-accent/15 text-accent" : "bg-destructive/15 text-destructive"
                }`}
              >
                {probeStatus(payload.ready.ok, t("statusOk"), t("statusFail"))}
              </span>
            </div>
            <p className="mt-2 font-mono text-xs text-muted-foreground">HTTP {payload.ready.status}</p>
            {readyChecks && (
              <dl className="mt-3 grid grid-cols-2 gap-2 text-xs">
                {Object.entries(readyChecks).map(([name, value]) => (
                  <div key={name} className="rounded-lg border border-border/70 bg-background px-3 py-2">
                    <dt className="font-semibold text-muted-foreground">{name}</dt>
                    <dd
                      className={`mt-0.5 font-mono font-bold ${
                        value === "ok" || value === "skipped" ? "text-foreground" : "text-destructive"
                      }`}
                    >
                      {value}
                    </dd>
                  </div>
                ))}
              </dl>
            )}
          </li>

          <li className="rounded-2xl border border-border bg-card p-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="font-display text-lg font-bold">{t("metricsTitle")}</p>
                <p className="text-xs text-muted-foreground">{t("metricsHint")}</p>
              </div>
              <span
                className={`rounded-lg px-2.5 py-1 text-xs font-bold ${
                  payload.metrics?.ok ? "bg-accent/15 text-accent" : "bg-destructive/15 text-destructive"
                }`}
              >
                {probeStatus(Boolean(payload.metrics?.ok), t("statusOk"), t("statusFail"))}
              </span>
            </div>
            <p className="mt-2 font-mono text-xs text-muted-foreground">
              HTTP {payload.metrics?.status ?? "—"}
            </p>
            {metrics && (
              <dl className="mt-3 grid grid-cols-2 gap-2 text-xs">
                <div className="rounded-lg border border-border/70 bg-background px-3 py-2">
                  <dt className="font-semibold text-muted-foreground">{t("metricsUptime")}</dt>
                  <dd className="mt-0.5 font-mono font-bold text-foreground">
                    {metrics.uptime_seconds ?? "—"}s
                  </dd>
                </div>
                <div className="rounded-lg border border-border/70 bg-background px-3 py-2">
                  <dt className="font-semibold text-muted-foreground">{t("metricsTotal")}</dt>
                  <dd className="mt-0.5 font-mono font-bold text-foreground">
                    {metrics.requests_total ?? "—"}
                  </dd>
                </div>
                {metrics.requests_by_status_class &&
                  Object.entries(metrics.requests_by_status_class).map(([name, value]) => (
                    <div key={name} className="rounded-lg border border-border/70 bg-background px-3 py-2">
                      <dt className="font-semibold text-muted-foreground">{name}</dt>
                      <dd className="mt-0.5 font-mono font-bold text-foreground">{value}</dd>
                    </div>
                  ))}
              </dl>
            )}
          </li>
        </ul>
      )}
    </main>
  );
}
