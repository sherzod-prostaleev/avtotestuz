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

// Domain error codes the backend's b2b installer endpoints can return (see
// writeInstallerErr in backend/internal/admin/b2b_installer.go). Each gets its
// own message so an operator knows what actually happened instead of a
// generic "action failed" -- most importantly rotated_no_seats, where the
// mutation (revoking the live key) already succeeded and only the
// replacement failed to mint.
const INSTALLER_ERROR_CODE_KEYS = {
  rotated_no_seats: "installerErrorRotatedNoSeats",
  no_license: "installerErrorNoLicense",
  org_suspended: "installerErrorOrgSuspended",
  seats_exhausted: "installerErrorSeatsExhausted",
} as const;

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

  const errorMessage = useCallback(
    (code: unknown) => {
      const key = typeof code === "string" ? INSTALLER_ERROR_CODE_KEYS[code as keyof typeof INSTALLER_ERROR_CODE_KEYS] : undefined;
      return key ? t(key) : t("installerError");
    },
    [t],
  );

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
        setError(errorMessage(body?.error?.code));
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
        setError(errorMessage(body?.error?.code));
        if (body?.error?.code === "rotated_no_seats") {
          // The backend's emergency stop already revoked the live key even
          // though it couldn't mint a replacement -- the code we're still
          // showing is dead. Stop rendering it as if it were live.
          setKey(null);
        }
        return;
      }
      setKey(body.data as InstallerKey);
    } catch {
      setError(t("installerError"));
    } finally {
      setBusy(false);
    }
  }

  // No locale on this URL on purpose. The kiosk carries its own language
  // switcher (see (kiosk)/kiosk-chrome.tsx), so the student at the PC picks
  // the language -- an admin choosing it here would strand anyone whose
  // language differs from whatever was selected at download time, on a screen
  // with no settings page to correct it. The agent's embedded locale is only
  // the first URL it opens.
  const downloadHref = `/api/admin/b2b/orgs/${orgId}/installer.exe`;

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

      <p className="text-xs text-muted-foreground">{t("installerHint")}</p>

      <div className="flex flex-wrap gap-2">
        {key ? (
          <a href={downloadHref} download>
            <Button type="button" as="span">
              {t("installerDownload")}
            </Button>
          </a>
        ) : null}
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
