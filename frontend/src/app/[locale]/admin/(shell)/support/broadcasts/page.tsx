"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { AdminTooltip } from "@/components/admin/admin-tooltip";
import { PermissionGate } from "@/components/admin/permission-gate";
import { createBroadcastIdempotencySession } from "@/lib/admin-broadcast-idempotency";

type Banner = {
  enabled: boolean;
  message: string;
  href?: string;
  updated_at?: string;
  updated_by?: string;
};

type Campaign = {
  id: string;
  title: string;
  body: string;
  audience: string;
  channels: string;
  status: string;
  recipient_total: number;
  pending_count: number;
  sent_count: number;
  failed_count: number;
  push_sent_count: number;
  push_failed_count: number;
  created_at?: string;
};

type DryRun = { recipients: number; push_eligible: number };

export default function AdminSupportBroadcastsPage() {
  const t = useTranslations("AdminBroadcast");
  const tNav = useTranslations("AdminNav");
  const [banner, setBanner] = useState<Banner | null>(null);
  const [message, setMessage] = useState("");
  const [href, setHref] = useState("");
  const [enabled, setEnabled] = useState(false);

  const [title, setTitle] = useState("Driver Go");
  const [body, setBody] = useState("");
  const [imageURL, setImageURL] = useState("");
  const [actionURL, setActionURL] = useState("/uz-Latn/dashboard");
  const [audience, setAudience] = useState("all_active");
  const [channels, setChannels] = useState("both");
  const [dryRun, setDryRun] = useState<DryRun | null>(null);
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(false);
  const sendIdempotencyRef = useRef(createBroadcastIdempotencySession());

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [bannerRes, listRes] = await Promise.all([
        fetch("/api/admin/support/banner", { cache: "no-store" }),
        fetch("/api/admin/support/broadcasts?limit=20", { cache: "no-store" }),
      ]);
      const bannerJson = await bannerRes.json();
      if (!bannerRes.ok) {
        setError(bannerJson?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setBanner(null);
        return;
      }
      const data = bannerJson.data as Banner;
      setBanner(data);
      setEnabled(Boolean(data.enabled));
      setMessage(data.message ?? "");
      setHref(data.href ?? "");

      if (listRes.ok) {
        const listJson = await listRes.json();
        setCampaigns((listJson.data?.items as Campaign[]) ?? []);
      }
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

  async function runDryRun() {
    setBusy(true);
    setError(null);
    setDryRun(null);
    try {
      const res = await fetch("/api/admin/support/broadcasts/dry-run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ audience, channels }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorPush"));
        return;
      }
      setDryRun(json.data as DryRun);
    } catch {
      setError(t("errorPush"));
    } finally {
      setBusy(false);
    }
  }

  async function sendCampaign() {
    const ok = window.confirm(
      t("confirmPush", {
        recipients: dryRun?.recipients ?? "?",
      }),
    );
    if (!ok) return;
    const { key, skip } = sendIdempotencyRef.current.begin();
    if (skip) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/support/broadcasts", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": key,
        },
        body: JSON.stringify({
          title,
          body,
          image_url: imageURL,
          action_url: actionURL,
          audience,
          channels,
          confirm: true,
        }),
      });
      const json = await res.json();
      if (!res.ok) {
        // Keep the same key so a user retry / browser retry is one logical send.
        sendIdempotencyRef.current.failKeepKey();
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorPush"));
        return;
      }
      sendIdempotencyRef.current.succeed();
      setBody("");
      setDryRun(null);
      await load();
    } catch {
      sendIdempotencyRef.current.failKeepKey();
      setError(t("errorPush"));
    } finally {
      setBusy(false);
    }
  }

  async function cancelCampaign(id: string) {
    setBusy(true);
    try {
      await fetch(`/api/admin/support/broadcasts/${id}/cancel`, { method: "POST" });
      await load();
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

        <section className="space-y-3 rounded-2xl border border-warning/40 bg-card/70 p-4">
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
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder={t("pushTitleField")}
            className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
          />
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder={t("pushBody")}
            rows={3}
            className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm"
          />
          <input
            value={imageURL}
            onChange={(e) => setImageURL(e.target.value)}
            placeholder={t("imageUrl")}
            className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
          />
          <input
            value={actionURL}
            onChange={(e) => setActionURL(e.target.value)}
            placeholder={t("pushUrl")}
            className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
          />
          <div className="grid gap-2 sm:grid-cols-2">
            <label className="space-y-1 text-xs font-medium">
              <span>{t("audience")}</span>
              <select
                value={audience}
                onChange={(e) => setAudience(e.target.value)}
                className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
              >
                <option value="all_active">{t("audienceAll")}</option>
                <option value="vip">{t("audienceVip")}</option>
                <option value="non_vip">{t("audienceNonVip")}</option>
              </select>
            </label>
            <label className="space-y-1 text-xs font-medium">
              <span>{t("channels")}</span>
              <select
                value={channels}
                onChange={(e) => setChannels(e.target.value)}
                className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
              >
                <option value="inapp">{t("channelsInapp")}</option>
                <option value="both">{t("channelsBoth")}</option>
              </select>
            </label>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void runDryRun()}>
              {t("pushDryRun")}
            </Button>
            <Button type="button" size="sm" disabled={busy || !body.trim()} onClick={() => void sendCampaign()}>
              {t("pushSend")}
            </Button>
          </div>
          {dryRun ? (
            <p className="text-xs text-muted-foreground">
              {t("dryRunResult", {
                recipients: dryRun.recipients,
                pushEligible: dryRun.push_eligible,
              })}
            </p>
          ) : null}
        </section>

        <section className="space-y-3 rounded-2xl border border-border/80 bg-card/70 p-4">
          <h2 className="text-sm font-bold">{t("campaignList")}</h2>
          {campaigns.length === 0 ? (
            <p className="text-xs text-muted-foreground">{t("campaignEmpty")}</p>
          ) : (
            <ul className="space-y-2">
              {campaigns.map((c) => (
                <li
                  key={c.id}
                  className="rounded-xl border border-border/70 bg-background/60 px-3 py-2 text-xs"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="font-semibold">{c.title}</span>
                    <span className="rounded-full bg-muted px-2 py-0.5 font-medium">{c.status}</span>
                  </div>
                  <p className="mt-1 text-muted-foreground line-clamp-2">{c.body}</p>
                  <p className="mt-1 text-muted-foreground">
                    {t("campaignStats", {
                      total: c.recipient_total,
                      pending: c.pending_count,
                      sent: c.sent_count,
                      failed: c.failed_count,
                      pushSent: c.push_sent_count,
                      pushFailed: c.push_failed_count,
                    })}
                  </p>
                  {["queued", "expanding", "sending"].includes(c.status) ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="mt-2"
                      disabled={busy}
                      onClick={() => void cancelCampaign(c.id)}
                    >
                      {t("cancel")}
                    </Button>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="space-y-3 rounded-2xl border border-border/80 bg-card/70 p-4">
          <h2 className="text-sm font-bold">{t("bannerTitle")}</h2>
          <p className="text-[11px] text-muted-foreground">{t("bannerNote")}</p>
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
      </main>
    </PermissionGate>
  );
}
