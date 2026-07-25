"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowLeft, ScrollText, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { OpsNav } from "@/components/ops/ops-nav";

type AuditRow = {
  id: string;
  actor_id?: string | null;
  action: string;
  entity: string;
  entity_id?: string | null;
  created_at: string;
};

const TOKEN_KEY = "drivergo:ops-admin-token";

export default function OpsAuditPage() {
  const t = useTranslations("OpsAudit");
  const tHealth = useTranslations("OpsHealth");
  const tProviders = useTranslations("OpsProviders");
  const tUsers = useTranslations("OpsUsers");
  const tPayments = useTranslations("OpsPayments");
  const locale = useLocale();
  const [token, setToken] = useState("");
  const [tokenDraft, setTokenDraft] = useState("");
  const [rows, setRows] = useState<AuditRow[] | null>(null);
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
        const res = await fetch("/api/ops/audit?limit=50", {
          headers: { "X-Ops-Token": opsToken },
          cache: "no-store",
        });
        const json = await res.json();
        if (!res.ok) {
          setError(json?.error?.code === "unauthorized" ? t("errorUnauthorized") : t("errorLoad"));
          setRows(null);
          return;
        }
        setRows(json.data as AuditRow[]);
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
    users: tUsers("navUsers"),
    payments: tPayments("navPayments"),
    audit: t("navAudit"),
  };

  return (
    <main className="page-shell-tight mx-auto max-w-2xl">
      <header className="mb-4">
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden className="h-4 w-4" /> {t("back")}
        </Link>
        <div className="mt-3 inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
          <ScrollText aria-hidden className="h-4 w-4" />
          {t("eyebrow")}
        </div>
        <h1 className="mt-2 font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">{t("subtitle")}</p>
      </header>

      <OpsNav locale={locale} active="audit" labels={navLabels} />

      <div className="mb-4 space-y-3 rounded-2xl border border-border bg-card p-4">
        <label className="block text-xs font-medium text-muted-foreground">
          {tProviders("tokenLabel")}
          <input
            type="password"
            value={tokenDraft}
            onChange={(e) => setTokenDraft(e.target.value)}
            className="mt-1 w-full rounded-xl border border-border bg-background px-3 py-2 text-sm"
          />
        </label>
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" onClick={saveToken}>
            {tProviders("tokenSave")}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={!token || loading} onClick={() => void load(token)}>
            <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            {tProviders("refresh")}
          </Button>
        </div>
      </div>

      {!token ? (
        <p className="text-sm text-muted-foreground">{tProviders("needToken")}</p>
      ) : error ? (
        <p className="text-sm text-destructive">{error}</p>
      ) : rows && rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("empty")}</p>
      ) : rows ? (
        <ul className="space-y-2">
          {rows.map((row) => (
            <li key={row.id} className="rounded-2xl border border-border bg-card px-4 py-3">
              <p className="text-sm font-medium">
                {row.action} · {row.entity}
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                {new Date(row.created_at).toLocaleString(locale)}
                {row.entity_id ? (
                  <>
                    {" "}
                    · <span className="font-mono">{row.entity_id.slice(0, 8)}</span>
                  </>
                ) : null}
              </p>
            </li>
          ))}
        </ul>
      ) : null}
    </main>
  );
}
