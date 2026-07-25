"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowLeft, RefreshCw, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import { OpsNav } from "@/components/ops/ops-nav";

type UserRow = {
  id: string;
  phone_masked: string;
  name: string;
  locale_pref: string;
  status: string;
  created_at: string;
};

const TOKEN_KEY = "drivergo:ops-admin-token";

export default function OpsUsersPage() {
  const t = useTranslations("OpsUsers");
  const tHealth = useTranslations("OpsHealth");
  const tProviders = useTranslations("OpsProviders");
  const tPayments = useTranslations("OpsPayments");
  const tAudit = useTranslations("OpsAudit");
  const tLimits = useTranslations("OpsLimits");
  const locale = useLocale();
  const [token, setToken] = useState("");
  const [tokenDraft, setTokenDraft] = useState("");
  const [q, setQ] = useState("");
  const [rows, setRows] = useState<UserRow[] | null>(null);
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
    async (opsToken: string, query: string) => {
      if (!opsToken) {
        setRows(null);
        return;
      }
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams({ limit: "50" });
        if (query.trim()) params.set("q", query.trim());
        const res = await fetch(`/api/ops/users?${params}`, {
          headers: { "X-Ops-Token": opsToken },
          cache: "no-store",
        });
        const json = await res.json();
        if (!res.ok) {
          setError(json?.error?.code === "unauthorized" ? t("errorUnauthorized") : t("errorLoad"));
          setRows(null);
          return;
        }
        setRows(json.data as UserRow[]);
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
    if (token) void load(token, q);
  }, [token, load]); // eslint-disable-line react-hooks/exhaustive-deps -- search on submit

  function saveToken() {
    const next = tokenDraft.trim();
    try {
      sessionStorage.setItem(TOKEN_KEY, next);
    } catch {
      /* ignore */
    }
    setToken(next);
  }

  return (
    <main className="page-shell-tight mx-auto max-w-2xl">
      <header className="mb-4">
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("back")}
        </Link>
        <div className="mt-3 inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
          <Users aria-hidden="true" className="h-4 w-4" />
          {t("eyebrow")}
        </div>
        <h1 className="mt-2 font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">{t("subtitle")}</p>
      </header>

      <OpsNav
        locale={locale}
        active="users"
        labels={{
          health: tHealth("navHealth"),
          providers: tProviders("navProviders"),
          users: t("navUsers"),
          payments: tPayments("navPayments"),
          audit: tAudit("navAudit"),
          limits: tLimits("navLimits"),
        }}
      />

      <div className="mb-4 space-y-3 rounded-2xl border border-border bg-card p-4">
        <label className="block text-xs font-medium text-muted-foreground">
          {tProviders("tokenLabel")}
          <input
            type="password"
            value={tokenDraft}
            onChange={(e) => setTokenDraft(e.target.value)}
            placeholder={tProviders("tokenPlaceholder")}
            className="mt-1 w-full rounded-xl border border-border bg-background px-3 py-2 text-sm"
          />
        </label>
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" onClick={saveToken}>
            {tProviders("tokenSave")}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={loading || !token} onClick={() => void load(token, q)}>
            <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            {loading ? tProviders("refreshing") : tProviders("refresh")}
          </Button>
        </div>
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            void load(token, q);
          }}
        >
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={t("searchPlaceholder")}
            className="min-w-0 flex-1 rounded-xl border border-border bg-background px-3 py-2 text-sm"
          />
          <Button type="submit" size="sm" variant="outline" disabled={!token || loading}>
            {t("search")}
          </Button>
        </form>
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
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <p className="font-mono text-sm">{row.phone_masked}</p>
                <span className="text-xs uppercase tracking-wide text-muted-foreground">{row.status}</span>
              </div>
              <p className="mt-1 text-sm font-medium">{row.name || "—"}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                {row.locale_pref} · {new Date(row.created_at).toLocaleString(locale)} ·{" "}
                <span className="font-mono">{row.id.slice(0, 8)}</span>
              </p>
            </li>
          ))}
        </ul>
      ) : null}
    </main>
  );
}
