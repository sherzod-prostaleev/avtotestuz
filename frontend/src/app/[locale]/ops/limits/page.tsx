"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowLeft, Gauge, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { OpsNav } from "@/components/ops/ops-nav";
import { OpsDeprecatedBanner } from "@/components/ops/ops-deprecated-banner";

type LimitRow = {
  key: string;
  free_value: number;
  vip_value: number;
  updated_at: string;
};

const TOKEN_KEY = "drivergo:ops-admin-token";

export default function OpsLimitsPage() {
  const t = useTranslations("OpsLimits");
  const tContacts = useTranslations("OpsContacts");
  const tHealth = useTranslations("OpsHealth");
  const tProviders = useTranslations("OpsProviders");
  const tUsers = useTranslations("OpsUsers");
  const tPayments = useTranslations("OpsPayments");
  const tAudit = useTranslations("OpsAudit");
  const locale = useLocale();
  const [token, setToken] = useState("");
  const [tokenDraft, setTokenDraft] = useState("");
  const [rows, setRows] = useState<LimitRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    try {
      const saved = sessionStorage.getItem(TOKEN_KEY) ?? "";
      setToken(saved);
      setTokenDraft(saved);
    } catch {
      /* ignore */
    }
  }, []);

  const load = useCallback(
    async (opsToken: string) => {
      if (!opsToken) {
        setRows(null);
        return;
      }
      setLoading(true);
      setError(null);
      try {
        const res = await fetch("/api/ops/limits", {
          headers: { "X-Ops-Token": opsToken },
          cache: "no-store",
        });
        const json = await res.json();
        if (!res.ok) {
          setError(json?.error?.code === "unauthorized" ? t("errorUnauthorized") : t("errorLoad"));
          setRows(null);
          return;
        }
        setRows(json.data as LimitRow[]);
      } catch {
        setError(t("errorLoad"));
        setRows(null);
      } finally {
        setLoading(false);
      }
    },
    [t],
  );

  useEffect(() => {
    if (token) void load(token);
  }, [token, load]);

  function saveToken() {
    const next = tokenDraft.trim();
    try {
      sessionStorage.setItem(TOKEN_KEY, next);
    } catch {
      /* ignore */
    }
    setToken(next);
  }

  const navLabels = {
    health: tHealth("navHealth"),
    providers: tProviders("navProviders"),
    contacts: tContacts("navContacts"),
    users: tUsers("navUsers"),
    payments: tPayments("navPayments"),
    audit: tAudit("navAudit"),
    limits: t("navLimits"),
  };

  return (
    <main className="page-shell-tight mx-auto max-w-2xl">
      <header className="mb-4">
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden className="h-4 w-4" /> {t("back")}
        </Link>
        <div className="mt-3 inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
          <Gauge aria-hidden className="h-4 w-4" />
          {t("eyebrow")}
        </div>
        <h1 className="mt-2 font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">{t("subtitle")}</p>
      </header>

      <OpsNav
        locale={locale}
        active="limits"
        adminLabel={tHealth("navAdmin")}
        labels={navLabels}
      />

      <OpsDeprecatedBanner
        locale={locale}
        href="settings/limits"
        label={t("adminHomeLink")}
        note="DEPRECATED"
      />

      <div className="mb-4 space-y-3 rounded-2xl border border-border bg-card p-4">
        <label className="block text-xs font-medium text-muted-foreground">
          {tProviders("tokenLabel")}
          <input
            type="password"
            autoComplete="off"
            value={tokenDraft}
            onChange={(e) => setTokenDraft(e.target.value)}
            className="field-input mt-1"
          />
        </label>
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" onClick={saveToken}>
            {tProviders("tokenSave")}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={loading || !token} onClick={() => void load(token)}>
            <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            {loading ? t("refreshing") : t("refresh")}
          </Button>
        </div>
      </div>

      {error && (
        <p role="alert" className="mb-4 rounded-xl border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </p>
      )}

      {rows && (
        <ul className="space-y-2">
          {rows.length === 0 ? (
            <li className="text-sm text-muted-foreground">{t("empty")}</li>
          ) : (
            rows.map((row) => (
              <li key={row.key} className="rounded-2xl border border-border bg-card px-4 py-3 text-sm">
                <p className="font-mono text-xs font-bold text-foreground">{row.key}</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("freeLabel")}: <span className="font-mono font-bold text-foreground">{row.free_value}</span>
                  {" · "}
                  {t("vipLabel")}: <span className="font-mono font-bold text-foreground">{row.vip_value}</span>
                </p>
              </li>
            ))
          )}
        </ul>
      )}
    </main>
  );
}
