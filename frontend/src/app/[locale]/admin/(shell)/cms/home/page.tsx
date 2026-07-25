"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EMPTY_SITE_HOME, type SiteHomeHero } from "@/lib/site-home";

const FIELDS: { key: keyof SiteHomeHero; labelKey: string; hintKey?: string }[] = [
  { key: "headline", labelKey: "fieldHeadline" },
  { key: "subtitle", labelKey: "fieldSubtitle" },
  { key: "ctaLabel", labelKey: "fieldCtaLabel" },
  { key: "ctaHref", labelKey: "fieldCtaHref", hintKey: "hintCtaHref" },
];

export default function AdminCMSHomePage() {
  const t = useTranslations("AdminCMS");
  const [form, setForm] = useState<SiteHomeHero>(EMPTY_SITE_HOME);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setOk(null);
    try {
      const res = await fetch("/api/admin/cms/home", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenRead") : t("errorLoad"));
        setLoaded(false);
        return;
      }
      setForm({ ...EMPTY_SITE_HOME, ...(json.data as SiteHomeHero) });
      setLoaded(true);
    } catch {
      setError(t("errorLoad"));
      setLoaded(false);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function save() {
    setSaving(true);
    setError(null);
    setOk(null);
    try {
      const res = await fetch("/api/admin/cms/home", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenWrite") : t("errorSave"));
        return;
      }
      setForm({ ...EMPTY_SITE_HOME, ...(json.data as SiteHomeHero) });
      setOk(t("homeSaved"));
    } catch {
      setError(t("errorSave"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <main className="mx-auto max-w-lg space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("homeTitle")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("homeSubtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("homeNote")}</p>
      </header>

      <div className="flex gap-2">
        <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
          <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          {t("refresh")}
        </Button>
      </div>

      {error ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
      {ok ? (
        <p role="status" className="text-sm text-foreground">
          {ok}
        </p>
      ) : null}

      {!loaded ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : (
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
              {field.hintKey ? (
                <span className="block text-[11px] text-muted-foreground">{t(field.hintKey)}</span>
              ) : null}
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
