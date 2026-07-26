"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowLeft, Copy, KeyRound, Shield } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { AdminTooltip } from "@/components/admin/admin-tooltip";
import { DangerConfirm } from "@/components/admin/danger-confirm";
import { PermissionGate } from "@/components/admin/permission-gate";

type EntitlementRow = {
  id: string;
  source: string;
  starts_at: string;
  ends_at: string;
  active: boolean;
  note?: string;
};

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
  has_password: boolean;
  streak: number;
  created_at: string;
  last_seen_at?: string;
  entitlements: EntitlementRow[];
};

type SessionRow = {
  id: string;
  created_at: string;
  expires_at: string;
  revoked_at?: string;
  active: boolean;
};

const GRANT_PRESETS = [7, 30, 90, 365] as const;
type Tab = "profile" | "security" | "billing" | "referral" | "activity" | "learning" | "notes" | "actions";

type ReferralAdmin = {
  total_invited: number;
  total_rewarded: number;
  earned_uzs: number;
  balance_uzs: number;
  commission_percent: number;
  referees: Array<{
    referee_id: string;
    referee_phone: string;
    referee_name: string;
    status: string;
    commission_uzs: number;
  }>;
};

export default function AdminUserDetailPage({ params }: { params: { id: string } }) {
  const t = useTranslations("AdminUsers");
  const tr = useTranslations("AdminReferral");
  const locale = useLocale();
  const { id } = params;
  const [user, setUser] = useState<UserDetail | null>(null);
  const [sessions, setSessions] = useState<SessionRow[] | null>(null);
  const [referral, setReferral] = useState<ReferralAdmin | null>(null);
  const [rateInput, setRateInput] = useState("20");
  const [balanceInput, setBalanceInput] = useState("0");
  const [balanceNote, setBalanceNote] = useState("");
  const [ledger, setLedger] = useState<
    Array<{ id: string; entry_type: string; amount_uzs: number; created_at: string }>
  >([]);
  const [error, setError] = useState<string | null>(null);
  const [okMsg, setOkMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [reason, setReason] = useState("");
  const [grantDays, setGrantDays] = useState(30);
  const [grantNote, setGrantNote] = useState("");
  const [tab, setTab] = useState<Tab>("profile");
  const [blockOpen, setBlockOpen] = useState(false);

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
      const [rRes, lRes] = await Promise.all([
        fetch(`/api/admin/users/${id}/referral`, { cache: "no-store" }),
        fetch(`/api/admin/users/${id}/referral/ledger`, { cache: "no-store" }),
      ]);
      if (rRes.ok) {
        const rJson = await rRes.json();
        const data = rJson.data as ReferralAdmin;
        setReferral(data);
        setRateInput(String(data.commission_percent));
        setBalanceInput(String(data.balance_uzs));
      }
      if (lRes.ok) {
        const lJson = await lRes.json();
        setLedger((lJson.data?.entries ?? []) as typeof ledger);
      }
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
    setOkMsg(null);
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
      setBlockOpen(false);
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
    setOkMsg(null);
    try {
      const res = await fetch(`/api/admin/users/${id}/sessions`, { method: "POST" });
      if (!res.ok) {
        setError(t("errorAction"));
        return;
      }
      setOkMsg(t("forceReloginDone"));
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

  async function grantVip() {
    if (grantDays < 1 || grantDays > 3660) {
      setError(t("grantDaysInvalid"));
      return;
    }
    setBusy(true);
    setError(null);
    setOkMsg(null);
    try {
      const res = await fetch(`/api/admin/users/${id}/grant`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          days: grantDays,
          note: grantNote.trim() || undefined,
        }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.message ?? t("errorAction"));
        return;
      }
      const until = json?.data?.until as string | undefined;
      setOkMsg(
        t("grantSuccess", {
          days: grantDays,
          until: until ? new Date(until).toLocaleString(locale) : "—",
        }),
      );
      setGrantNote("");
      await load();
    } catch {
      setError(t("errorAction"));
    } finally {
      setBusy(false);
    }
  }

  if (error && !user) {
    return (
      <main className="mx-auto max-w-4xl space-y-3">
        <Link href={`/${locale}/admin/users`} className="text-sm font-semibold text-accent hover:underline">
          ← {t("back")}
        </Link>
        <AdminErrorState message={error} />
      </main>
    );
  }

  if (!user) {
    return <AdminSkeleton rows={6} />;
  }

  const blocked = user.status === "blocked";
  const tabs: { id: Tab; label: string }[] = [
    { id: "profile", label: t("profileTab") },
    { id: "security", label: t("tabSecurity") },
    { id: "billing", label: t("tabBilling") },
    { id: "referral", label: tr("userTab") },
    { id: "activity", label: t("tabActivity") },
    { id: "learning", label: t("tabLearning") },
    { id: "notes", label: t("tabNotes") },
    { id: "actions", label: t("actionsTab") },
  ];

  return (
    <PermissionGate permission="users.read">
      <main className="mx-auto max-w-4xl space-y-6">
        <div>
          <Link
            href={`/${locale}/admin/users`}
            className="inline-flex items-center gap-1 text-sm font-semibold text-accent hover:underline"
          >
            <ArrowLeft aria-hidden className="h-4 w-4" /> {t("back")}
          </Link>
          <AdminPageHeader
            badge={t("controlBadge")}
            title={user.name || user.phone}
            description={user.id}
            actions={
              <div className="flex flex-wrap gap-2">
                <span
                  className={`rounded-lg px-2 py-1 text-[11px] font-extrabold uppercase ${
                    blocked ? "bg-destructive/15 text-destructive" : "bg-emerald-500/15 text-emerald-400"
                  }`}
                >
                  {user.status}
                </span>
                <span
                  className={`inline-flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-extrabold uppercase ${
                    user.vip_active ? "bg-accent/20 text-accent" : "bg-muted text-muted-foreground"
                  }`}
                >
                  <Shield aria-hidden className="h-3.5 w-3.5" />
                  {user.vip_active ? t("vipYes") : t("vipNo")}
                </span>
              </div>
            }
          />
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <p className="font-mono text-base font-semibold tracking-tight">{user.phone}</p>
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => void navigator.clipboard.writeText(user.phone)}
            >
              <Copy aria-hidden className="mr-1 h-3.5 w-3.5" />
              {t("copyPhone")}
            </Button>
          </div>
        </div>

        {error ? <AdminErrorState message={error} /> : null}
        {okMsg ? (
          <p className="rounded-xl border border-accent/30 bg-accent/10 px-3 py-2 text-sm">{okMsg}</p>
        ) : null}

        <div className="flex flex-wrap gap-1 border-b border-border/70 pb-2" role="tablist">
          {tabs.map((tb) => (
            <button
              key={tb.id}
              type="button"
              role="tab"
              aria-selected={tab === tb.id}
              className={`rounded-lg px-3 py-1.5 text-xs font-bold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                tab === tb.id ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:bg-white/[0.04]"
              }`}
              onClick={() => setTab(tb.id)}
            >
              {tb.label}
            </button>
          ))}
        </div>

        {tab === "profile" ? (
          <section className="space-y-3 rounded-2xl border border-border/80 bg-card/70 p-4">
            <dl className="grid gap-3 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-xs text-muted-foreground">{t("colVip")}</dt>
                <dd>
                  {user.vip_active ? t("vipYes") : t("vipNo")}
                  {user.vip_ends_at ? ` · ${new Date(user.vip_ends_at).toLocaleString(locale)}` : ""}
                </dd>
              </div>
              <div>
                <dt className="flex items-center gap-1 text-xs text-muted-foreground">
                  {t("colPassword")}
                  <AdminTooltip
                    content={{
                      what: t("passwordNote"),
                      why: t("forceReloginHint"),
                      risks: t("passwordNote"),
                      recommend: t("revokeAll"),
                    }}
                  />
                </dt>
                <dd className="flex items-center gap-1.5">
                  <KeyRound aria-hidden className="h-3.5 w-3.5 text-muted-foreground" />
                  {user.has_password ? t("passwordSet") : t("passwordUnset")}
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
                <dd>{[user.region, user.district].filter(Boolean).join(" / ") || "—"}</dd>
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
        ) : null}

        {tab === "security" ? (
          <section className="space-y-3 rounded-2xl border border-border/80 bg-card/70 p-4">
            <p className="text-xs text-muted-foreground">{t("forceReloginHint")}</p>
            <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void revokeAll()}>
              {t("revokeAll")}
            </Button>
            <h3 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
              {t("sessionsTab")}
            </h3>
            {!sessions?.length ? (
              <p className="text-sm text-muted-foreground">{t("sessionsEmpty")}</p>
            ) : (
              <ul className="space-y-2">
                {sessions.map((s) => (
                  <li
                    key={s.id}
                    className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-border/70 px-3 py-2 text-sm"
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
        ) : null}

        {tab === "billing" ? (
          <div className="space-y-4">
            <PermissionGate permission="users.entitlements.grant" mode="hide">
              <section className="space-y-3 rounded-2xl border border-accent/25 bg-gradient-to-br from-accent/10 via-card/80 to-card p-4">
                <h2 className="text-xs font-extrabold uppercase tracking-wider text-accent">
                  {t("grantVipTitle")}
                </h2>
                <p className="text-xs text-muted-foreground">{t("grantVipHint")}</p>
                <div className="flex flex-wrap gap-2">
                  {GRANT_PRESETS.map((d) => (
                    <Button
                      key={d}
                      type="button"
                      size="sm"
                      variant={grantDays === d ? "game" : "outline"}
                      disabled={busy}
                      onClick={() => setGrantDays(d)}
                    >
                      {t("grantPreset", { days: d })}
                    </Button>
                  ))}
                </div>
                <div className="grid gap-3 sm:grid-cols-[140px_1fr_auto]">
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold text-muted-foreground">{t("grantDays")}</span>
                    <input
                      type="number"
                      min={1}
                      max={3660}
                      value={grantDays}
                      onChange={(e) => setGrantDays(Number(e.target.value) || 0)}
                      className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm tabular-nums"
                    />
                  </label>
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold text-muted-foreground">{t("grantNote")}</span>
                    <input
                      value={grantNote}
                      onChange={(e) => setGrantNote(e.target.value)}
                      placeholder={t("grantNotePlaceholder")}
                      className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
                    />
                  </label>
                  <div className="flex items-end">
                    <Button type="button" disabled={busy} onClick={() => void grantVip()}>
                      {t("grantSubmit")}
                    </Button>
                  </div>
                </div>
              </section>
            </PermissionGate>
            <section className="space-y-3 rounded-2xl border border-border/80 bg-card/70 p-4">
              <h2 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
                {t("entitlementsTab")}
              </h2>
              {!user.entitlements?.length ? (
                <p className="text-sm text-muted-foreground">{t("entitlementsEmpty")}</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[520px] text-left text-sm">
                    <thead className="text-[11px] uppercase tracking-wider text-muted-foreground">
                      <tr>
                        <th className="pb-2 pr-3 font-bold">{t("colSource")}</th>
                        <th className="pb-2 pr-3 font-bold">{t("colStarts")}</th>
                        <th className="pb-2 pr-3 font-bold">{t("colEnds")}</th>
                        <th className="pb-2 font-bold">{t("colActive")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {user.entitlements.map((e) => (
                        <tr key={e.id} className="border-t border-border/50">
                          <td className="py-2 pr-3">
                            <p className="font-semibold">{e.source}</p>
                            {e.note ? <p className="text-[11px] text-muted-foreground">{e.note}</p> : null}
                          </td>
                          <td className="py-2 pr-3 text-xs">
                            {new Date(e.starts_at).toLocaleString(locale)}
                          </td>
                          <td className="py-2 pr-3 text-xs">
                            {new Date(e.ends_at).toLocaleString(locale)}
                          </td>
                          <td className="py-2 text-xs font-bold uppercase">
                            {e.active ? t("entitlementActive") : t("entitlementInactive")}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>
          </div>
        ) : null}

        {tab === "referral" ? (
          <section className="space-y-4 rounded-2xl border border-border/80 bg-card/70 p-4">
            {!referral ? (
              <p className="text-sm text-muted-foreground">{tr("errorLoad")}</p>
            ) : (
              <>
                <div className="grid gap-3 sm:grid-cols-5">
                  <div className="rounded-xl border border-accent/30 bg-accent/5 p-3">
                    <p className="text-[11px] text-muted-foreground">{tr("statPercent")}</p>
                    <p className="text-lg font-bold text-accent">{referral.commission_percent}%</p>
                  </div>
                  <div className="rounded-xl border border-border p-3">
                    <p className="text-[11px] text-muted-foreground">{tr("statInvited")}</p>
                    <p className="text-lg font-bold">{referral.total_invited}</p>
                  </div>
                  <div className="rounded-xl border border-border p-3">
                    <p className="text-[11px] text-muted-foreground">{tr("statRewarded")}</p>
                    <p className="text-lg font-bold">{referral.total_rewarded}</p>
                  </div>
                  <div className="rounded-xl border border-border p-3">
                    <p className="text-[11px] text-muted-foreground">{tr("statEarned")}</p>
                    <p className="text-lg font-bold">{referral.earned_uzs.toLocaleString()}</p>
                  </div>
                  <div className="rounded-xl border border-border p-3">
                    <p className="text-[11px] text-muted-foreground">{tr("statBalance")}</p>
                    <p className="text-lg font-bold">{referral.balance_uzs.toLocaleString()}</p>
                  </div>
                </div>
                <PermissionGate permission="referral.rates.manage" mode="hide">
                  <div className="space-y-3 rounded-xl border border-border/70 p-3">
                    <div className="flex flex-wrap items-end gap-2">
                      <label className="space-y-1">
                        <span className="text-xs text-muted-foreground">{tr("rateLabel")}</span>
                        <input
                          type="number"
                          min={0}
                          max={100}
                          value={rateInput}
                          onChange={(e) => setRateInput(e.target.value)}
                          className="h-10 w-28 rounded-xl border border-border bg-background px-3 text-sm"
                        />
                      </label>
                      <Button
                        type="button"
                        size="sm"
                        variant="game"
                        disabled={busy}
                        onClick={() => {
                          void (async () => {
                            setBusy(true);
                            setError(null);
                            try {
                              const res = await fetch(`/api/admin/users/${id}/referral/rate`, {
                                method: "PATCH",
                                headers: { "Content-Type": "application/json" },
                                body: JSON.stringify({ percent: Number(rateInput) }),
                              });
                              if (!res.ok) {
                                setError(tr("errorAction"));
                                return;
                              }
                              setOkMsg(tr("rateSaved"));
                              await load();
                            } catch {
                              setError(tr("errorAction"));
                            } finally {
                              setBusy(false);
                            }
                          })();
                        }}
                      >
                        {tr("saveRate")}
                      </Button>
                    </div>
                    <div className="flex flex-wrap items-end gap-2">
                      <label className="space-y-1">
                        <span className="text-xs text-muted-foreground">{tr("balanceLabel")}</span>
                        <input
                          type="number"
                          min={0}
                          value={balanceInput}
                          onChange={(e) => setBalanceInput(e.target.value)}
                          className="h-10 w-40 rounded-xl border border-border bg-background px-3 text-sm font-mono"
                        />
                      </label>
                      <label className="min-w-[180px] flex-1 space-y-1">
                        <span className="text-xs text-muted-foreground">{tr("balanceNote")}</span>
                        <input
                          type="text"
                          value={balanceNote}
                          onChange={(e) => setBalanceNote(e.target.value)}
                          placeholder={tr("balanceNotePlaceholder")}
                          className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
                        />
                      </label>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={busy}
                        onClick={() => {
                          void (async () => {
                            setBusy(true);
                            setError(null);
                            try {
                              const res = await fetch(`/api/admin/users/${id}/referral/balance`, {
                                method: "PUT",
                                headers: { "Content-Type": "application/json" },
                                body: JSON.stringify({
                                  balance_uzs: Number(balanceInput),
                                  note: balanceNote,
                                }),
                              });
                              if (!res.ok) {
                                setError(tr("errorAction"));
                                return;
                              }
                              setOkMsg(tr("balanceSaved"));
                              setBalanceNote("");
                              await load();
                            } catch {
                              setError(tr("errorAction"));
                            } finally {
                              setBusy(false);
                            }
                          })();
                        }}
                      >
                        {tr("saveBalance")}
                      </Button>
                    </div>
                  </div>
                </PermissionGate>
                <div>
                  <h3 className="mb-2 text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
                    {tr("ledgerTitle")}
                  </h3>
                  {!ledger.length ? (
                    <p className="text-sm text-muted-foreground">{tr("ledgerEmpty")}</p>
                  ) : (
                    <ul className="max-h-64 space-y-1 overflow-y-auto text-sm">
                      {ledger.map((e) => (
                        <li
                          key={e.id}
                          className="flex flex-wrap justify-between gap-2 rounded-lg border border-border/50 px-2 py-1.5"
                        >
                          <span className="font-mono text-xs text-muted-foreground">{e.entry_type}</span>
                          <span className={`font-mono text-xs font-bold ${e.amount_uzs >= 0 ? "text-emerald-500" : "text-destructive"}`}>
                            {e.amount_uzs >= 0 ? "+" : ""}
                            {e.amount_uzs.toLocaleString()}
                          </span>
                          <span className="w-full text-[11px] text-muted-foreground">
                            {new Date(e.created_at).toLocaleString(locale)}
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
                <div>
                  <h3 className="mb-2 text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
                    {tr("refereesTitle")}
                  </h3>
                  {!referral.referees.length ? (
                    <p className="text-sm text-muted-foreground">{tr("refereesEmpty")}</p>
                  ) : (
                    <ul className="space-y-2">
                      {referral.referees.map((r) => (
                        <li
                          key={r.referee_id}
                          className="flex flex-wrap justify-between gap-2 rounded-xl border border-border/70 px-3 py-2 text-sm"
                        >
                          <div>
                            <p className="font-semibold">{r.referee_name || r.referee_phone}</p>
                            <p className="text-xs text-muted-foreground">
                              {r.referee_phone} · {r.status}
                            </p>
                          </div>
                          <p className="font-mono text-sm">{r.commission_uzs.toLocaleString()} so&apos;m</p>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </>
            )}
          </section>
        ) : null}

        {tab === "activity" || tab === "learning" || tab === "notes" ? (
          <AdminEmptyState title={tabs.find((x) => x.id === tab)?.label || ""} description={t("comingTab")} />
        ) : null}

        {tab === "actions" ? (
          <section className="space-y-3 rounded-2xl border border-border/80 bg-card/70 p-4">
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
                  onClick={() => setBlockOpen(true)}
                >
                  {t("block")}
                </Button>
              )}
              <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void revokeAll()}>
                {t("revokeAll")}
              </Button>
            </div>
          </section>
        ) : null}

        <DangerConfirm
          open={blockOpen}
          onOpenChange={setBlockOpen}
          title={t("blockConfirmTitle")}
          warnings={[t("blockWarn1"), t("blockWarn2")]}
          confirmPhrase={t("confirmBlock")}
          confirmPhraseLabel={t("blockPhrase")}
          confirmLabel={t("block")}
          cancelLabel={t("cancel")}
          busy={busy}
          onConfirm={() => void blockOrUnblock(true)}
        />
      </main>
    </PermissionGate>
  );
}
