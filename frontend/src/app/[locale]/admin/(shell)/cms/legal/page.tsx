"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminTooltip } from "@/components/admin/admin-tooltip";
import { PermissionGate } from "@/components/admin/permission-gate";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import { hasPermission } from "@/lib/admin-permissions";
import {
  LEGAL_LOCALES,
  emptyLegalBundle,
  normalizeLegalBundle,
  type SiteLegalBundle,
  type SiteLegalDoc,
  type SiteLegalLocale,
} from "@/lib/site-legal";

type DocFieldDef = {
  key: keyof SiteLegalDoc;
  labelKey: string;
  hintKey?: string;
  tooltipPrefix?: string;
  rows?: number;
};

const DOC_FIELDS: DocFieldDef[] = [
  { key: "oferta", labelKey: "fieldOferta", hintKey: "hintLegalBody", tooltipPrefix: "tipLegalBody", rows: 12 },
  { key: "privacy", labelKey: "fieldPrivacy", hintKey: "hintLegalBody", tooltipPrefix: "tipLegalBody", rows: 12 },
  { key: "refund", labelKey: "fieldRefund", hintKey: "hintRefund", tooltipPrefix: "tipRefund", rows: 6 },
];

export default function AdminCMSLegalPage() {
  const t = useTranslations("AdminCMS");
  const tNav = useTranslations("AdminNav");
  const me = useAdminMeOptional();
  const canWrite = hasPermission(me?.permissions, "cms.write");
  const [bundle, setBundle] = useState<SiteLegalBundle>(emptyLegalBundle);
  const [activeLocale, setActiveLocale] = useState<SiteLegalLocale>("uz-Latn");
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
      const res = await fetch("/api/admin/cms/legal", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenRead") : t("errorLoad"));
        setLoaded(false);
        return;
      }
      setBundle(normalizeLegalBundle(json.data));
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

  function updateField(key: keyof SiteLegalDoc, value: string) {
    setBundle((prev) => ({
      locales: {
        ...prev.locales,
        [activeLocale]: { ...prev.locales[activeLocale], [key]: value },
      },
    }));
  }

  async function save() {
    setSaving(true);
    setError(null);
    setOk(null);
    try {
      const res = await fetch("/api/admin/cms/legal", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(bundle),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenWrite") : t("errorSave"));
        return;
      }
      setBundle(normalizeLegalBundle(json.data));
      setOk(t("legalSaved"));
    } catch {
      setError(t("errorSave"));
    } finally {
      setSaving(false);
    }
  }

  const doc = bundle.locales[activeLocale];

  return (
    <PermissionGate permission="cms.read">
      <main className="mx-auto max-w-3xl space-y-5">
        <AdminPageHeader
          badge={tNav("groupCMS")}
          title={t("legalTitle")}
          description={t("legalSubtitle")}
          actions={
            <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
              <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              {t("refresh")}
            </Button>
          }
        />
        <p className="-mt-2 text-xs text-muted-foreground">{t("legalNote")}</p>

        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load()} />
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
            {!canWrite ? (
              <p className="rounded-xl border border-border/80 bg-card/50 px-3 py-2 text-xs text-muted-foreground">
                {t("readOnlyHint")}
              </p>
            ) : null}

            <div className="flex flex-wrap gap-2" role="tablist" aria-label={t("legalLocaleLabel")}>
              {LEGAL_LOCALES.map((loc) => (
                <button
                  key={loc}
                  type="button"
                  role="tab"
                  aria-selected={activeLocale === loc}
                  onClick={() => setActiveLocale(loc)}
                  className={`min-h-11 rounded-xl border px-3 text-xs font-bold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    activeLocale === loc
                      ? "border-accent bg-accent/15 text-foreground"
                      : "border-border bg-card text-muted-foreground hover:border-accent/50"
                  }`}
                >
                  {loc}
                </button>
              ))}
            </div>

            {DOC_FIELDS.map((field) => (
              <label key={field.key} className="block space-y-1.5">
                <span className="flex items-center gap-1 text-xs font-semibold text-muted-foreground">
                  {t(field.labelKey)}
                  {field.tooltipPrefix ? (
                    <AdminTooltip
                      label={t("tooltipLabel")}
                      content={{
                        what: t(`${field.tooltipPrefix}What`),
                        why: t(`${field.tooltipPrefix}Why`),
                        risks: t(`${field.tooltipPrefix}Risks`),
                        recommend: t(`${field.tooltipPrefix}Recommend`),
                      }}
                    />
                  ) : null}
                </span>
                {field.hintKey ? (
                  <span className="block text-[11px] text-muted-foreground">{t(field.hintKey)}</span>
                ) : null}
                <textarea
                  value={doc[field.key]}
                  readOnly={!canWrite}
                  onChange={(e) => updateField(field.key, e.target.value)}
                  rows={field.rows ?? 12}
                  className="w-full rounded-xl border border-border bg-background px-3 py-2.5 font-mono text-xs leading-relaxed text-foreground read-only:opacity-70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                />
              </label>
            ))}

            <PermissionGate permission="cms.write" mode="hide">
              <Button type="submit" variant="game" size="sm" disabled={saving}>
                {saving ? t("saving") : t("save")}
              </Button>
            </PermissionGate>
          </form>
        )}
      </main>
    </PermissionGate>
  );
}
