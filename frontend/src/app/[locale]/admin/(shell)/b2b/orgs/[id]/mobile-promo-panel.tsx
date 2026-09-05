"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Smartphone } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PermissionGate } from "@/components/admin/permission-gate";
import { MobilePromoBanner, type MobilePromo } from "@/components/station/mobile-promo";

/**
 * One school's kiosk advert: the link, the switch, and a preview of exactly
 * what its classrooms will show.
 *
 * The preview renders the same component the kiosk does, from the same
 * server-generated QR, so "what I saved" and "what the classroom sees" cannot
 * drift apart -- and an operator can point a phone at their own screen to
 * check the link before a single PC shows it.
 */
export default function MobilePromoPanel({ orgId }: { orgId: string }) {
  const t = useTranslations("MobilePromo");
  const [saved, setSaved] = useState<MobilePromo | null>(null);
  const [url, setURL] = useState("");
  const [enabled, setEnabled] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);
  const endpoint = `/api/admin/b2b/orgs/${orgId}/mobile-promo`;

  const load = useCallback(async () => {
    setError("");
    try {
      const res = await fetch(endpoint, { cache: "no-store" });
      if (!res.ok) throw new Error("load");
      const { data } = await res.json();
      setSaved(data);
      setURL(data.url);
      setEnabled(data.enabled);
    } catch {
      setError(t("loadError"));
    }
  }, [endpoint, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function save(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setSuccess(false);
    try {
      const res = await fetch(endpoint, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled, url }),
      });
      const body = await res.json();
      if (!res.ok) {
        setError(t(body?.error?.code === "invalid_url" ? "invalidURL" : "saveError"));
        return;
      }
      setSaved(body.data);
      setSuccess(true);
    } catch {
      setError(t("saveError"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="space-y-4 rounded-2xl border border-border bg-card p-5">
      <h2 className="flex items-center gap-2 text-base font-bold">
        <Smartphone aria-hidden="true" className="h-5 w-5 text-accent" />
        {t("adminTitle")}
      </h2>
      <p className="text-sm text-muted-foreground">{t("adminHint")}</p>
      {error ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
      {!saved ? (
        <Button onClick={() => void load()} disabled={!error}>
          {t(error ? "retry" : "loading")}
        </Button>
      ) : (
        <>
          {/* Same permission as minting the school's installer key: both decide
              what a paying school's classrooms put in front of students. */}
          <PermissionGate permission="users.entitlements.grant" mode="hide">
            <form onSubmit={save} className="space-y-4">
              <div className="space-y-2">
                <label htmlFor="mobile-promo-url" className="block text-sm font-medium">
                  {t("urlLabel")}
                </label>
                {/* type="text", not type="url": the browser's own URL validation
                    rejects shapes the backend accepts and, worse, says nothing
                    about why. The exact bytes typed here are what the QR
                    encodes, so nothing may normalise them on the way through. */}
                <input
                  id="mobile-promo-url"
                  type="text"
                  inputMode="url"
                  value={url}
                  onChange={(e) => {
                    setURL(e.target.value);
                    setSuccess(false);
                  }}
                  required={enabled}
                  maxLength={512}
                  disabled={busy}
                  spellCheck={false}
                  autoCapitalize="none"
                  autoComplete="off"
                  placeholder="https://drivergo.uz/r/REF-XXXXXX"
                  aria-describedby="mobile-promo-url-hint"
                  className="w-full min-w-0 rounded-xl border border-border bg-background px-3 py-2.5 text-sm"
                />
                <p id="mobile-promo-url-hint" className="text-xs text-muted-foreground">
                  {t("urlHint")}
                </p>
              </div>
              <label className="flex cursor-pointer items-center gap-3 rounded-xl bg-muted/50 p-3 text-sm font-medium">
                <input
                  type="checkbox"
                  role="switch"
                  checked={enabled}
                  onChange={(e) => {
                    setEnabled(e.target.checked);
                    setSuccess(false);
                  }}
                  disabled={busy}
                  className="h-5 w-5 accent-accent"
                />
                {t("enable")}
              </label>
              <Button type="submit" disabled={busy}>
                {t(busy ? "saving" : "save")}
              </Button>
              {success ? (
                <p role="status" className="text-sm text-success">
                  {t("saved")}
                </p>
              ) : null}
            </form>
          </PermissionGate>
          <div className="space-y-3 border-t border-border pt-4">
            <p className="text-xs font-semibold text-muted-foreground">
              {t(saved.enabled ? "livePreview" : "disabled")}
            </p>
            {saved.url ? <p className="break-all font-mono text-xs">{saved.url}</p> : null}
            {/* Forced enabled so a switched-off school can still be shown what
                it would publish before publishing it. */}
            {saved.url ? <MobilePromoBanner promo={{ ...saved, enabled: true }} /> : null}
          </div>
        </>
      )}
    </section>
  );
}
