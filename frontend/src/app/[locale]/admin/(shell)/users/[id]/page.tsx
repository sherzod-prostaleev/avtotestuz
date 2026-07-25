"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";

type UserDetail = {
  id: string;
  phone: string;
  phone_masked: string;
  name: string;
  region: string;
  district: string;
  locale_pref: string;
  theme_pref: string;
  role: string;
  status: string;
  referral_code?: string;
  referred_by?: string;
  vip_active: boolean;
  vip_ends_at?: string;
  streak: number;
  created_at: string;
  last_seen_at?: string;
};

type SessionRow = {
  id: string;
  created_at: string;
  expires_at: string;
  revoked_at?: string;
  active: boolean;
};

export default function AdminUserDetailPage({ params }: { params: { id: string } }) {
  const t = useTranslations("AdminUsers");
  const locale = useLocale();
  const { id } = params;
  const [user, setUser] = useState<UserDetail | null>(null);
  const [sessions, setSessions] = useState<SessionRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [reason, setReason] = useState("");

  const load = useCallback(async () => {
    setError(null);
    try {
      const [uRes, sRes] = await Promise.all([
        fetch(`/api/admin/users/${id}`, { cache: "no-store" }),
        fetch(`/api/admin/users/${id}/sessions`, { cache: "no-store" }),
      ]);
      const uJson = await uRes.json();
      const sJson = await sRes.json();
      if (!uRes.ok) {
        setError(uJson?.error?.code === "not_found" ? t("errorNotFound") : t("errorLoad"));
        setUser(null);
        return;
      }
      setUser(uJson.data as UserDetail);
      if (sRes.ok) setSessions(sJson.data as SessionRow[]);
      else setSessions([]);
    } catch {
      setError(t("errorLoad"));
    }
  }, [id, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function blockOrUnblock(block: boolean) {
    const trimmed = reason.trim();
    if (!trimmed) {
      setError(t("reasonRequired"));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/users/${id}/${block ? "block" : "unblock"}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reason: trimmed }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.message ?? t("errorAction"));
        return;
      }
      setReason("");
      await load();
    } catch {
      setError(t("errorAction"));
    } finally {
      setBusy(false);
    }
  }

  async function revokeAll() {
    if (!window.confirm(t("confirmRevokeAll"))) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/users/${id}/sessions`, { method: "POST" });
      if (!res.ok) {
        setError(t("errorAction"));
        return;
      }
      await load();
    } catch {
      setError(t("errorAction"));
    } finally {
      setBusy(false);
    }
  }

  async function revokeOne(sessionId: string) {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/users/${id}/sessions/${sessionId}/revoke`, {
        method: "POST",
      });
      if (!res.ok) {
        setError(t("errorAction"));
        return;
      }
      await load();
    } catch {
      setError(t("errorAction"));
    } finally {
      setBusy(false);
    }
  }

  if (error && !user) {
    return (
      <main className="mx-auto max-w-3xl space-y-3">
        <Link href={`/${locale}/admin/users`} className="text-sm font-semibold text-accent hover:underline">
          ← {t("back")}
        </Link>
        <p className="text-sm text-destructive">{error}</p>
      </main>
    );
  }

  if (!user) {
    return <p className="text-sm text-muted-foreground">{t("loading")}</p>;
  }

  const blocked = user.status === "blocked";

  return (
    <main className="mx-auto max-w-3xl space-y-6">
      <div>
        <Link
          href={`/${locale}/admin/users`}
          className="inline-flex items-center gap-1 text-sm font-semibold text-accent hover:underline"
        >
          <ArrowLeft aria-hidden className="h-4 w-4" /> {t("back")}
        </Link>
        <h1 className="mt-3 font-display text-2xl font-extrabold tracking-tight">
          {user.name || user.phone}
        </h1>
        <p className="mt-1 font-mono text-sm text-muted-foreground">{user.phone}</p>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <section className="space-y-2 rounded-xl border border-border bg-card p-4">
        <h2 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
          {t("profileTab")}
        </h2>
        <dl className="grid gap-2 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-xs text-muted-foreground">{t("colStatus")}</dt>
            <dd className="font-semibold uppercase">{user.status}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t("colVip")}</dt>
            <dd>
              {user.vip_active ? t("vipYes") : t("vipNo")}
              {user.vip_ends_at
                ? ` · ${new Date(user.vip_ends_at).toLocaleString(locale)}`
                : ""}
            </dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t("colStreak")}</dt>
            <dd className="tabular-nums">{user.streak}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t("locale")}</dt>
            <dd>{user.locale_pref}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t("region")}</dt>
            <dd>
              {[user.region, user.district].filter(Boolean).join(" / ") || "—"}
            </dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t("referral")}</dt>
            <dd className="font-mono text-xs">{user.referral_code || "—"}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t("colCreated")}</dt>
            <dd>{new Date(user.created_at).toLocaleString(locale)}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t("lastSeen")}</dt>
            <dd>
              {user.last_seen_at ? new Date(user.last_seen_at).toLocaleString(locale) : "—"}
            </dd>
          </div>
        </dl>
      </section>

      <section className="space-y-3 rounded-xl border border-border bg-card p-4">
        <h2 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
          {t("actionsTab")}
        </h2>
        <label className="block space-y-1.5">
          <span className="text-xs font-semibold text-muted-foreground">{t("reasonLabel")}</span>
          <input
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder={t("reasonPlaceholder")}
            className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
          />
        </label>
        <div className="flex flex-wrap gap-2">
          {blocked ? (
            <Button type="button" size="sm" disabled={busy} onClick={() => void blockOrUnblock(false)}>
              {t("unblock")}
            </Button>
          ) : (
            <Button
              type="button"
              size="sm"
              variant="destructive"
              disabled={busy}
              onClick={() => void blockOrUnblock(true)}
            >
              {t("block")}
            </Button>
          )}
          <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void revokeAll()}>
            {t("revokeAll")}
          </Button>
        </div>
      </section>

      <section className="space-y-3 rounded-xl border border-border bg-card p-4">
        <h2 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
          {t("sessionsTab")}
        </h2>
        {!sessions || sessions.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("sessionsEmpty")}</p>
        ) : (
          <ul className="space-y-2">
            {sessions.map((s) => (
              <li
                key={s.id}
                className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-border/70 px-3 py-2 text-sm"
              >
                <div>
                  <p className="font-mono text-xs">{s.id.slice(0, 8)}…</p>
                  <p className="text-xs text-muted-foreground">
                    {new Date(s.created_at).toLocaleString(locale)} ·{" "}
                    {s.active ? t("sessionActive") : t("sessionInactive")}
                  </p>
                </div>
                {s.active ? (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={busy}
                    onClick={() => void revokeOne(s.id)}
                  >
                    {t("revokeOne")}
                  </Button>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
