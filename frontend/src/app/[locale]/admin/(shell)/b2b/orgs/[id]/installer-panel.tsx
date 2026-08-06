"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

type InstallerKey = {
  code: string;
  max_uses: number;
  used_count: number;
  expires_at: string;
};

const KIOSK_LOCALES = ["uz-Latn", "uz-Cyrl", "ru"] as const;

// One installer key per school: a POST is idempotent (returns the existing
// key), rotate revokes it and issues a fresh one. The download is a plain
// <a download> so the browser streams the .exe and honours the backend's
// Content-Disposition filename — fetch()-ing it into memory would lose both.
export default function InstallerPanel({ orgId }: { orgId: string }) {
  const t = useTranslations("AdminB2B");
  const locale = useLocale();
  const [key, setKey] = useState<InstallerKey | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [dlLocale, setDlLocale] = useState<string>(locale);

  const load = useCallback(async () => {
    setError(null);
    try {
      const res = await fetch(`/api/admin/b2b/orgs/${orgId}/installer`, { cache: "no-store" });
      const body = await res.json();
      if (!res.ok) {
        setError(t("installerError"));
        return;
      }
      setKey(body.data as InstallerKey | null);
    } catch {
      setError(t("installerError"));
    } finally {
      setLoaded(true);
    }
  }, [orgId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function openKey() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/b2b/orgs/${orgId}/installer`, { method: "POST" });
      const body = await res.json();
      if (!res.ok) {
        setError(t("installerError"));
        return;
      }
      setKey(body.data as InstallerKey);
    } catch {
      setError(t("installerError"));
    } finally {
      setBusy(false);
    }
  }

  async function rotateKey() {
    if (!window.confirm(t("installerRotateConfirm"))) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/b2b/orgs/${orgId}/installer/rotate`, { method: "POST" });
      const body = await res.json();
      if (!res.ok) {
        setError(t("installerError"));
        return;
      }
      setKey(body.data as InstallerKey);
    } catch {
      setError(t("installerError"));
    } finally {
      setBusy(false);
    }
  }

  const downloadHref = `/api/admin/b2b/orgs/${orgId}/installer.exe?locale=${dlLocale}`;

  return (
    <section className="space-y-2 rounded-xl border border-border bg-card p-4">
      <h2 className="text-sm font-bold">{t("installerTitle")}</h2>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!loaded ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : key ? (
        <>
          <p className="break-all rounded-lg border border-border bg-background p-3 font-mono text-lg font-bold">
            {key.code}
          </p>
          <p className="text-xs text-muted-foreground">
            {t("installerUsed", { used: key.used_count, max: key.max_uses })}
          </p>
          <p className="text-xs text-muted-foreground">
            {t("installerExpires", { date: new Date(key.expires_at).toLocaleDateString(locale) })}
          </p>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">{t("installerNone")}</p>
      )}

      <label className="block space-y-1 text-xs font-semibold text-muted-foreground">
        <span>{t("installerLocale")}</span>
        <select
          value={dlLocale}
          onChange={(e) => setDlLocale(e.target.value)}
          className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
        >
          {KIOSK_LOCALES.map((l) => (
            <option key={l} value={l}>
              {l}
            </option>
          ))}
        </select>
      </label>

      <p className="text-xs text-muted-foreground">{t("installerHint")}</p>

      <div className="flex flex-wrap gap-2">
        <a href={downloadHref} download>
          <Button type="button" as="span">
            {t("installerDownload")}
          </Button>
        </a>
        {key ? (
          <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void rotateKey()}>
            {t("installerRotate")}
          </Button>
        ) : (
          <Button type="button" size="sm" disabled={busy || !loaded} onClick={() => void openKey()}>
            {t("installerOpen")}
          </Button>
        )}
      </div>
    </section>
  );
}
