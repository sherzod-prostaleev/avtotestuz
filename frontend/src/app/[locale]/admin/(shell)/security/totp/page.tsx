"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminTooltip } from "@/components/admin/admin-tooltip";
import { DangerConfirm } from "@/components/admin/danger-confirm";
import { useAdminMe } from "@/components/admin/admin-me-context";
import { Button } from "@/components/ui/button";

export default function AdminTOTPPage() {
  const t = useTranslations("AdminTOTP");
  const me = useAdminMe();
  const [secret, setSecret] = useState<string | null>(null);
  const [otpauth, setOtpauth] = useState<string | null>(null);
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [disableOpen, setDisableOpen] = useState(false);
  const [enabled, setEnabled] = useState(Boolean(me.totp_enabled));

  async function enroll() {
    setBusy(true);
    setError(null);
    setOk(null);
    try {
      const res = await fetch("/api/admin/security/totp/enroll", { method: "POST" });
      const json = await res.json();
      if (!res.ok) {
        setError(t("error"));
        return;
      }
      setSecret(json.data.secret as string);
      setOtpauth(json.data.otpauth_url as string);
    } catch {
      setError(t("error"));
    } finally {
      setBusy(false);
    }
  }

  async function confirm() {
    if (!secret) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/security/totp/confirm", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ secret, code }),
      });
      if (!res.ok) {
        setError(t("errorCode"));
        return;
      }
      setEnabled(true);
      setSecret(null);
      setOtpauth(null);
      setCode("");
      setOk(t("enabledOk"));
    } catch {
      setError(t("error"));
    } finally {
      setBusy(false);
    }
  }

  async function disable() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/security/totp/disable", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code }),
      });
      if (!res.ok) {
        setError(t("errorCode"));
        return;
      }
      setEnabled(false);
      setDisableOpen(false);
      setCode("");
      setOk(t("disabledOk"));
    } catch {
      setError(t("error"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto max-w-2xl space-y-5">
      <AdminPageHeader
        badge={t("badge")}
        title={t("title")}
        description={t("subtitle")}
        actions={
          <AdminTooltip
            content={{
              what: t("tipWhat"),
              why: t("tipWhy"),
              when: t("tipWhen"),
              risks: t("tipRisks"),
              recommend: t("tipRecommend"),
            }}
          />
        }
      />

      <section className="rounded-2xl border border-border/80 bg-card/70 p-4 space-y-3">
        <p className="text-sm">
          {t("status")}:{" "}
          <span className="font-bold">{enabled ? t("statusOn") : t("statusOff")}</span>
        </p>
        {me.totp_setup_required && !enabled ? (
          <p className="rounded-xl border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-100">
            {t("enforceHint")}
          </p>
        ) : null}
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        {ok ? <p className="text-sm text-emerald-300">{ok}</p> : null}

        {!enabled && !secret ? (
          <Button type="button" disabled={busy} onClick={() => void enroll()}>
            {t("startEnroll")}
          </Button>
        ) : null}

        {secret ? (
          <div className="space-y-3">
            <p className="text-xs text-muted-foreground">{t("secretOnce")}</p>
            <code className="block break-all rounded-xl border border-border bg-background px-3 py-2 font-mono text-xs">
              {secret}
            </code>
            {otpauth ? (
              <p className="break-all font-mono text-[11px] text-muted-foreground">{otpauth}</p>
            ) : null}
            <label className="block space-y-1.5">
              <span className="text-xs font-semibold text-muted-foreground">{t("code")}</span>
              <input
                value={code}
                onChange={(e) => setCode(e.target.value)}
                className="h-10 w-full rounded-xl border border-border bg-background px-3 font-mono text-sm"
                placeholder="000000"
              />
            </label>
            <Button type="button" disabled={busy || code.length < 6} onClick={() => void confirm()}>
              {t("confirm")}
            </Button>
          </div>
        ) : null}

        {enabled ? (
          <div className="space-y-3 border-t border-border/60 pt-3">
            <label className="block space-y-1.5">
              <span className="text-xs font-semibold text-muted-foreground">{t("code")}</span>
              <input
                value={code}
                onChange={(e) => setCode(e.target.value)}
                className="h-10 w-full rounded-xl border border-border bg-background px-3 font-mono text-sm"
              />
            </label>
            <Button
              type="button"
              size="sm"
              variant="destructive"
              disabled={busy || code.length < 6}
              onClick={() => setDisableOpen(true)}
            >
              {t("disable")}
            </Button>
          </div>
        ) : null}
      </section>

      <DangerConfirm
        open={disableOpen}
        onOpenChange={setDisableOpen}
        title={t("disableTitle")}
        warnings={[t("disableWarn1"), t("disableWarn2")]}
        confirmPhrase="TOTP-OF"
        confirmPhraseLabel={t("disablePhrase")}
        confirmLabel={t("disable")}
        cancelLabel={t("cancel")}
        busy={busy}
        onConfirm={() => void disable()}
      />
    </main>
  );
}
