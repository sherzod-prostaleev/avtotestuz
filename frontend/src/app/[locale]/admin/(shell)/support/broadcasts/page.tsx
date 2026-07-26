"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { AdminTooltip } from "@/components/admin/admin-tooltip";
import { PermissionGate } from "@/components/admin/permission-gate";

type Banner = {
  enabled: boolean;
  message: string;
  href?: string;
  updated_at?: string;
  updated_by?: string;
};

type BroadcastResult = {
  recipients: number;
  notified: number;
  deliveries: number;
  errors: number;
  dry_run: boolean;
};

export default function AdminSupportBroadcastsPage() {
  const t = useTranslations("AdminBroadcast");
  const tNav = useTranslations("AdminNav");
  const [banner, setBanner] = useState<Banner | null>(null);
  const [message, setMessage] = useState("");
  const [href, setHref] = useState("");
  const [enabled, setEnabled] = useState(false);
  const [pushTitle, setPushTitle] = useState("Driver Go");
  const [pushBody, setPushBody] = useState("");
  const [pushURL, setPushURL] = useState("/uz-Latn/dashboard");
  const [lastPush, setLastPush] = useState<BroadcastResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/support/banner", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setBanner(null);
        return;
      }
      const data = json.data as Banner;
      setBanner(data);
      setEnabled(Boolean(data.enabled));
      setMessage(data.message ?? "");
      setHref(data.href ?? "");
    } catch {
      setError(t("errorLoad"));
      setBanner(null);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function saveBanner() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/support/banner", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled, message, href }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorBanner"));
        return;
      }
      const data = json.data as Banner;
      setBanner(data);
      setEnabled(Boolean(data.enabled));
      setMessage(data.message ?? "");
      setHref(data.href ?? "");
    } catch {
      setError(t("errorBanner"));
    } finally {
      setBusy(false);
    }
  }

  async function sendPush(dryRun: boolean) {
    if (!dryRun) {
      const ok = window.confirm(t("confirmPush"));
      if (!ok) return;
    }
    setBusy(true);
    setError(null);
    setLastPush(null);
    try {
      const res = await fetch("/api/admin/support/broadcasts/webpush", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title: pushTitle,
          body: pushBody,
          url: pushURL,
          dry_run: dryRun,
        }),
      });
      const json = await res.json();
      if (!res.ok) {
        if (json?.error?.code === "web_push_unconfigured") {
          setError(t("errorUnconfigured"));
        } else if (json?.error?.code === "forbidden") {
          setError(t("errorForbidden"));
        } else {
          setError(t("errorPush"));
        }
        return;
      }
      setLastPush(json.data as BroadcastResult);
    } catch {
      setError(t("errorPush"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <PermissionGate permission="support.broadcast">
      <main className="mx-auto max-w-2xl space-y-5">
        <AdminPageHeader
          badge={tNav("groupSupport")}
          title={t("title")}
          description={`${t("subtitle")} ${t("note")}`}
          actions={
            <Button type="button" size="sm" variant="outline" disabled={loading || busy} onClick={() => void load()}>
              <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              {t("refresh")}
            </Button>
          }
        />

        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load()} />
        ) : null}

        {loading && !banner ? <AdminSkeleton rows={4} /> : null}

        <section className="space-y-3 rounded-2xl border border-border/80 bg-card/70 p-4">
          <h2 className="text-sm font-bold">{t("bannerTitle")}</h2>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
            {t("bannerEnabled")}
          </label>
          <textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder={t("bannerMessage")}
            rows={3}
            className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm"
          />
          <input
            value={href}
            onChange={(e) => setHref(e.target.value)}
            placeholder={t("bannerHref")}
            className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
          />
          <Button type="button" size="sm" disabled={busy} onClick={() => void saveBanner()}>
            {t("bannerSave")}
          </Button>
          {banner?.updated_at ? (
            <p className="text-[11px] text-muted-foreground">
              {banner.updated_at}
              {banner.updated_by ? ` · ${banner.updated_by}` : ""}
            </p>
          ) : null}
        </section>

        <section className="space-y-3 rounded-2xl border border-amber-500/30 bg-card/70 p-4">
          <div className="flex items-center gap-1">
            <h2 className="text-sm font-bold">{t("pushTitle")}</h2>
            <AdminTooltip
              content={{
                what: t("pushTipWhat"),
                why: t("pushTipWhy"),
                risks: t("pushTipRisks"),
                recommend: t("pushTipRecommend"),
              }}
              label={t("pushTipLabel")}
            />
          </div>
          <input
            value={pushTitle}
            onChange={(e) => setPushTitle(e.target.value)}
            placeholder={t("pushTitleField")}
            className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
          />
          <textarea
            value={pushBody}
            onChange={(e) => setPushBody(e.target.value)}
            placeholder={t("pushBody")}
            rows={3}
            className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm"
          />
          <input
            value={pushURL}
            onChange={(e) => setPushURL(e.target.value)}
            placeholder={t("pushUrl")}
            className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
          />
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={busy || !pushBody.trim()}
              onClick={() => void sendPush(true)}
            >
              {t("pushDryRun")}
            </Button>
            <Button type="button" size="sm" disabled={busy || !pushBody.trim()} onClick={() => void sendPush(false)}>
              {t("pushSend")}
            </Button>
          </div>
          {lastPush ? (
            <p className="text-xs text-muted-foreground">
              {t("pushResult", {
                recipients: lastPush.recipients,
                notified: lastPush.notified,
                deliveries: lastPush.deliveries,
                errors: lastPush.errors,
                mode: lastPush.dry_run ? t("modeDry") : t("modeLive"),
              })}
            </p>
          ) : null}
        </section>
      </main>
    </PermissionGate>
  );
}
