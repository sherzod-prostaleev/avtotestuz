"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiDelete, apiGet, apiPost } from "@/lib/api-client";
import { Button } from "@/components/ui/button";

type EnrollWindow = {
  id: string;
  code: string;
  max_uses: number;
  used_count: number;
  expires_at: string;
};

// Keys rather than translated strings, so the fetch/action callbacks below
// never need to close over `t`. Closing over `t` would make `load`'s
// identity depend on `useTranslations`'s return value; that identity is not
// guaranteed stable across renders, and `load` feeds a mount effect, so an
// unstable `load` would refire the fetch on every render.
type ErrorKey = "errorLoad" | "enrollError";

/** Minutes left until iso, floored at 0. */
function minutesLeft(iso: string): number {
  return Math.max(0, Math.floor((new Date(iso).getTime() - Date.now()) / 60_000));
}

export default function EnrollWindowPanel({ orgId }: { orgId: string }) {
  const t = useTranslations("Teacher");
  const [win, setWin] = useState<EnrollWindow | null>(null);
  const [busy, setBusy] = useState(false);
  const [errorKey, setErrorKey] = useState<ErrorKey | null>(null);
  const [minutes, setMinutes] = useState(0);

  const load = useCallback(async () => {
    try {
      setWin(await apiGet<EnrollWindow | null>(`me/teacher/orgs/${orgId}/enroll-window`));
    } catch {
      setErrorKey("errorLoad");
    }
  }, [orgId]);

  useEffect(() => {
    void load();
  }, [load]);

  // Recompute the countdown once a minute, without re-fetching: the window's
  // expiry is fixed at creation, so only the clock moves.
  useEffect(() => {
    if (!win) return;
    setMinutes(minutesLeft(win.expires_at));
    const id = setInterval(() => setMinutes(minutesLeft(win.expires_at)), 60_000);
    return () => clearInterval(id);
  }, [win]);

  const open = async () => {
    setBusy(true);
    setErrorKey(null);
    try {
      setWin(
        await apiPost<EnrollWindow>(`me/teacher/orgs/${orgId}/enroll-window`, { ttl_minutes: 120 }),
      );
    } catch {
      setErrorKey("enrollError");
    } finally {
      setBusy(false);
    }
  };

  const close = async () => {
    if (!win) return;
    setBusy(true);
    setErrorKey(null);
    try {
      await apiDelete(`me/teacher/orgs/${orgId}/enroll-window/${win.id}`);
      setWin(null);
    } catch {
      setErrorKey("enrollError");
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="rounded-lg border p-4">
      <h2 className="mb-2 text-lg font-semibold">{t("enrollTitle")}</h2>

      {win ? (
        <div className="space-y-3">
          <p className="select-all font-mono text-3xl tracking-widest">{win.code}</p>
          <p className="text-sm opacity-80">
            {t("enrollUsed", { used: win.used_count, max: win.max_uses })}
          </p>
          <p className="text-sm opacity-80">
            {t("enrollExpires", { minutes })}
          </p>
          <p className="text-sm opacity-70">{t("enrollHint", { max: win.max_uses })}</p>
          <Button variant="outline" onClick={close} disabled={busy}>
            {t("enrollClose")}
          </Button>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-sm opacity-80">{t("enrollNone")}</p>
          <Button onClick={open} disabled={busy}>
            {t("enrollOpen")}
          </Button>
        </div>
      )}

      {errorKey ? <p className="mt-3 text-sm text-red-500">{t(errorKey)}</p> : null}
    </section>
  );
}
