"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowLeft, CreditCard, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { OpsNav } from "@/components/ops/ops-nav";
import { OpsDeprecatedBanner } from "@/components/ops/ops-deprecated-banner";

type ProviderRow = { provider: string; enabled: boolean };

const TOKEN_KEY = "drivergo:ops-admin-token";

export default function OpsPaymentProvidersPage() {
  const t = useTranslations("OpsProviders");
  const tHealth = useTranslations("OpsHealth");
  const tUsers = useTranslations("OpsUsers");
  const tPayments = useTranslations("OpsPayments");
  const tAudit = useTranslations("OpsAudit");
  const tLimits = useTranslations("OpsLimits");
  const tContacts = useTranslations("OpsContacts");
  const locale = useLocale();
  const [token, setToken] = useState("");
  const [tokenDraft, setTokenDraft] = useState("");
  const [rows, setRows] = useState<ProviderRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [lastLoadedAt, setLastLoadedAt] = useState<string | null>(null);

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
        setLastLoadedAt(null);
        return;
      }
      setLoading(true);
      setError(null);
      try {
        const res = await fetch("/api/ops/payment-providers", {
          headers: { "X-Ops-Token": opsToken },
          cache: "no-store",
        });
        const json = await res.json();
        if (!res.ok) {
          setError(json?.error?.code === "unauthorized" ? t("errorUnauthorized") : t("errorLoad"));
          setRows(null);
          return;
        }
        setRows(json.data as ProviderRow[]);
        setLastLoadedAt(new Date().toISOString());
      } catch {
        setError(t("errorLoad"));
        setRows(null);
      } finally {
        setLoading(false);
      }
    },
    [t]
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

  async function toggle(provider: string, enabled: boolean) {
    if (!token) return;
    if (!enabled) {
      const ok = window.confirm(t("confirmDisable", { provider }));
      if (!ok) return;
    }
    setBusy(provider);
    setError(null);
    try {
      const res = await fetch(`/api/ops/payment-providers/${provider}`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          "X-Ops-Token": token,
        },
        body: JSON.stringify({ enabled }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(t("errorToggle"));
        return;
      }
      setRows((prev) =>
        (prev ?? []).map((row) =>
          row.provider === provider ? { ...row, enabled: Boolean(json.data?.enabled ?? enabled) } : row
        )
      );
      setLastLoadedAt(new Date().toISOString());
    } catch {
      setError(t("errorToggle"));
    } finally {
      setBusy(null);
    }
  }

  return (
    <main className="page-shell-tight mx-auto max-w-lg">
      <header className="mb-4">
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("back")}
        </Link>
        <div className="mt-3 inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
          <CreditCard aria-hidden="true" className="h-4 w-4" />
          {t("eyebrow")}
        </div>
        <h1 className="mt-2 font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">{t("subtitle")}</p>
      </header>

      <OpsNav
        locale={locale}
        active="providers"
        adminLabel={tHealth("navAdmin")}
        labels={{
          health: tHealth("navHealth"),
          providers: t("navProviders"),
          contacts: tContacts("navContacts"),
          users: tUsers("navUsers"),
          payments: tPayments("navPayments"),
          audit: tAudit("navAudit"),
          limits: tLimits("navLimits"),
        }}
      />

      <OpsDeprecatedBanner
        locale={locale}
        href="payments/providers"
        label={t("adminHomeLink")}
        note="DEPRECATED"
      />

      <section className="mb-6 space-y-3 rounded-2xl border border-border bg-card p-5">
        <label className="block text-xs font-semibold text-muted-foreground" htmlFor="ops-token">
          {t("tokenLabel")}
        </label>
        <input
          id="ops-token"
          type="password"
          autoComplete="off"
          value={tokenDraft}
          onChange={(e) => setTokenDraft(e.target.value)}
          className="h-11 w-full rounded-xl border border-border bg-background px-3 text-sm"
          placeholder={t("tokenPlaceholder")}
        />
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" onClick={saveToken}>
            {t("tokenSave")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={!token || loading}
            onClick={() => void load(token)}
          >
            <RefreshCw aria-hidden="true" className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            {loading ? t("refreshing") : t("refresh")}
          </Button>
        </div>
        {lastLoadedAt && (
          <p className="text-xs text-muted-foreground">
            {t("lastLoaded", { time: new Date(lastLoadedAt).toLocaleTimeString() })}
          </p>
        )}
      </section>

      {error && (
        <p role="alert" className="mb-4 rounded-xl border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </p>
      )}

      {rows && (
        <ul className="space-y-3">
          {rows.map((row) => (
            <li
              key={row.provider}
              className="flex items-center justify-between gap-3 rounded-2xl border border-border bg-card p-4"
            >
              <div>
                <p className="font-display text-lg font-bold capitalize">{row.provider}</p>
                <p className="text-xs text-muted-foreground">
                  {row.enabled ? t("statusOn") : t("statusOff")}
                </p>
              </div>
              <Button
                type="button"
                size="sm"
                variant={row.enabled ? "outline" : "gold"}
                disabled={busy === row.provider}
                onClick={() => void toggle(row.provider, !row.enabled)}
              >
                {busy === row.provider
                  ? t("saving")
                  : row.enabled
                    ? t("disable")
                    : t("enable")}
              </Button>
            </li>
          ))}
        </ul>
      )}

      {!token && (
        <p className="text-sm text-muted-foreground">{t("needToken")}</p>
      )}
    </main>
  );
}
