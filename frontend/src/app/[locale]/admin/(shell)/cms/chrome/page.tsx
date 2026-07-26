"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminTooltip, type AdminTooltipContent } from "@/components/admin/admin-tooltip";
import { PermissionGate } from "@/components/admin/permission-gate";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import { hasPermission } from "@/lib/admin-permissions";
import { EMPTY_SITE_CONTACTS, type SiteContacts } from "@/lib/site-contacts";

type FieldDef = {
  key: keyof SiteContacts;
  labelKey: string;
  hintKey?: string;
  tooltipPrefix?: string;
};

const FIELDS: FieldDef[] = [
  { key: "phone", labelKey: "fieldPhone" },
  { key: "phoneTel", labelKey: "fieldPhoneTel", hintKey: "hintPhoneTel", tooltipPrefix: "tipPhoneTel" },
  { key: "email", labelKey: "fieldEmail" },
  { key: "address", labelKey: "fieldAddress" },
  { key: "hours", labelKey: "fieldHours" },
  { key: "telegram", labelKey: "fieldTelegram" },
  { key: "telegramUrl", labelKey: "fieldTelegramUrl", tooltipPrefix: "tipSocialUrl" },
  { key: "instagram", labelKey: "fieldInstagram" },
  { key: "instagramUrl", labelKey: "fieldInstagramUrl", tooltipPrefix: "tipSocialUrl" },
];

function tooltipFromPrefix(t: (k: string) => string, prefix: string): AdminTooltipContent {
  return {
    what: t(`${prefix}What`),
    why: t(`${prefix}Why`),
    when: t(`${prefix}When`),
    risks: t(`${prefix}Risks`),
    recommend: t(`${prefix}Recommend`),
  };
}

export default function AdminCMSChromePage() {
  const t = useTranslations("AdminCMS");
  const tNav = useTranslations("AdminNav");
  const me = useAdminMeOptional();
  const canWrite = hasPermission(me?.permissions, "cms.write");
  const [form, setForm] = useState<SiteContacts>(EMPTY_SITE_CONTACTS);
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
      const res = await fetch("/api/admin/cms/contacts", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenRead") : t("errorLoad"));
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
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function save() {
    setSaving(true);
    setError(null);
    setOk(null);
    try {
      const res = await fetch("/api/admin/cms/contacts", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenWrite") : t("errorSave"));
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

  return (
    <PermissionGate permission="cms.read">
      <main className="mx-auto max-w-lg space-y-5">
        <AdminPageHeader
          badge={tNav("groupCMS")}
          title={t("chromeTitle")}
          description={t("chromeSubtitle")}
          actions={
            <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
              <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              {t("refresh")}
            </Button>
          }
        />
        <p className="-mt-2 text-xs text-muted-foreground">{t("chromeNote")}</p>

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
            {FIELDS.map((field) => (
              <label key={field.key} className="block space-y-1.5">
                <span className="flex items-center gap-1 text-xs font-semibold text-muted-foreground">
                  {t(field.labelKey)}
                  {field.tooltipPrefix ? (
                    <AdminTooltip
                      label={t("tooltipLabel")}
                      content={tooltipFromPrefix(t, field.tooltipPrefix)}
                    />
                  ) : null}
                </span>
                <input
                  type="text"
                  value={form[field.key]}
                  readOnly={!canWrite}
                  onChange={(e) => setForm((prev) => ({ ...prev, [field.key]: e.target.value }))}
                  className="h-11 w-full rounded-xl border border-border bg-background px-3 text-sm read-only:opacity-70"
                />
                {field.hintKey ? (
                  <span className="block text-[11px] text-muted-foreground">{t(field.hintKey)}</span>
                ) : null}
              </label>
            ))}
            <p className="text-xs text-muted-foreground">{t("emptyHint")}</p>
            <PermissionGate permission="cms.write" mode="hide">
              <Button type="submit" disabled={saving}>
                {saving ? t("saving") : t("save")}
              </Button>
            </PermissionGate>
          </form>
        )}
      </main>
    </PermissionGate>
  );
}
