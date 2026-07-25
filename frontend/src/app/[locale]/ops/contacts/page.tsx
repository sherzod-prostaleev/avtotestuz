"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowLeft, Phone, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { OpsNav } from "@/components/ops/ops-nav";
import { EMPTY_SITE_CONTACTS, type SiteContacts } from "@/lib/site-contacts";

const TOKEN_KEY = "drivergo:ops-admin-token";

const FIELDS: { key: keyof SiteContacts; labelKey: string; hintKey?: string }[] = [
  { key: "phone", labelKey: "fieldPhone" },
  { key: "phoneTel", labelKey: "fieldPhoneTel", hintKey: "hintPhoneTel" },
  { key: "email", labelKey: "fieldEmail" },
  { key: "address", labelKey: "fieldAddress" },
  { key: "hours", labelKey: "fieldHours" },
  { key: "telegram", labelKey: "fieldTelegram" },
  { key: "telegramUrl", labelKey: "fieldTelegramUrl" },
  { key: "instagram", labelKey: "fieldInstagram" },
  { key: "instagramUrl", labelKey: "fieldInstagramUrl" },
];

export default function OpsContactsPage() {
  const t = useTranslations("OpsContacts");
  const tHealth = useTranslations("OpsHealth");
  const tProviders = useTranslations("OpsProviders");
  const tUsers = useTranslations("OpsUsers");
  const tPayments = useTranslations("OpsPayments");
  const tAudit = useTranslations("OpsAudit");
  const tLimits = useTranslations("OpsLimits");
  const locale = useLocale();
  const [token, setToken] = useState("");
  const [tokenDraft, setTokenDraft] = useState("");
  const [form, setForm] = useState<SiteContacts>(EMPTY_SITE_CONTACTS);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

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
        setLoaded(false);
        setForm(EMPTY_SITE_CONTACTS);
        return;
      }
      setLoading(true);
      setError(null);
      setOk(null);
      try {
        const res = await fetch("/api/ops/site-contacts", {
          headers: { "X-Ops-Token": opsToken },
          cache: "no-store",
        });
        const json = await res.json();
        if (!res.ok) {
          setError(json?.error?.code === "unauthorized" ? t("errorUnauthorized") : t("errorLoad"));
          setLoaded(false);
          return;
        }
        setForm({ ...EMPTY_SITE_CONTACTS, ...(json.data as SiteContacts) });
        setLoaded(true);
      } catch {
        setError(t("errorLoad"));
        setLoaded(false);
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

  async function save() {
    if (!token) return;
    setSaving(true);
    setError(null);
    setOk(null);
    try {
      const res = await fetch("/api/ops/site-contacts", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          "X-Ops-Token": token,
        },
        body: JSON.stringify(form),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "unauthorized" ? t("errorUnauthorized") : t("errorSave"));
        return;
      }
      setForm({ ...EMPTY_SITE_CONTACTS, ...(json.data as SiteContacts) });
      setOk(t("saved"));
    } catch {
      setError(t("errorSave"));
    } finally {
      setSaving(false);
    }
  }

  const navLabels = {
    health: tHealth("navHealth"),
    providers: tProviders("navProviders"),
    contacts: t("navContacts"),
    users: tUsers("navUsers"),
    payments: tPayments("navPayments"),
    audit: tAudit("navAudit"),
    limits: tLimits("navLimits"),
  };

  return (
    <main className="page-shell-tight mx-auto max-w-lg">
      <header className="mb-4">
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("back")}
        </Link>
        <div className="mt-3 inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
          <Phone aria-hidden="true" className="h-4 w-4" />
          {t("eyebrow")}
        </div>
        <h1 className="mt-2 font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-2 text-sm">
          <Link href={`/${locale}/admin/cms/chrome`} className="font-semibold text-accent underline-offset-2 hover:underline">
            {t("adminHomeLink")}
          </Link>
        </p>
      </header>

      <OpsNav locale={locale} active="contacts" labels={navLabels} />

      <section className="mb-6 space-y-3 rounded-2xl border border-border bg-card p-5">
        <label className="block text-xs font-semibold text-muted-foreground" htmlFor="ops-token">
          {tProviders("tokenLabel")}
        </label>
        <input
          id="ops-token"
          type="password"
          autoComplete="off"
          value={tokenDraft}
          onChange={(e) => setTokenDraft(e.target.value)}
          className="h-11 w-full rounded-xl border border-border bg-background px-3 text-sm"
          placeholder={tProviders("tokenPlaceholder")}
        />
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" onClick={saveToken}>
            {tProviders("tokenSave")}
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
      </section>

      {error && (
        <p role="alert" className="mb-4 rounded-xl border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </p>
      )}
      {ok && (
        <p role="status" className="mb-4 rounded-xl border border-success/40 bg-success/10 px-4 py-3 text-sm text-foreground">
          {ok}
        </p>
      )}

      {!token && <p className="text-sm text-muted-foreground">{t("needToken")}</p>}

      {loaded && (
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
        >
          {FIELDS.map((field) => (
            <label key={field.key} className="block space-y-1.5">
              <span className="text-xs font-semibold text-muted-foreground">{t(field.labelKey)}</span>
              <input
                type="text"
                value={form[field.key]}
                onChange={(e) => setForm((prev) => ({ ...prev, [field.key]: e.target.value }))}
                className="h-11 w-full rounded-xl border border-border bg-background px-3 text-sm"
              />
              {field.hintKey && (
                <span className="block text-[11px] text-muted-foreground">{t(field.hintKey)}</span>
              )}
            </label>
          ))}
          <p className="text-xs text-muted-foreground">{t("emptyHint")}</p>
          <Button type="submit" disabled={saving}>
            {saving ? t("saving") : t("save")}
          </Button>
        </form>
      )}
    </main>
  );
}
