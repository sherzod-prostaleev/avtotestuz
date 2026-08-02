"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { type ColumnDef } from "@tanstack/react-table";
import { ArrowLeft, Copy, KeyRound, Shield } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import {
  AdminDataTable,
  type AdminColumnMeta,
} from "@/components/admin/admin-data-table";
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
  bypass_variant_progress: boolean;
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

type RefereeRow = ReferralAdmin["referees"][number];

type LedgerRow = {
  id: string;
  entry_type: string;
  amount_uzs: number;
  created_at: string;
};

export default function AdminUserDetailPage() {
  const t = useTranslations("AdminUsers");
  const tr = useTranslations("AdminReferral");
  const locale = useLocale();
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [user, setUser] = useState<UserDetail | null>(null);
  const [sessions, setSessions] = useState<SessionRow[] | null>(null);
  const [referral, setReferral] = useState<ReferralAdmin | null>(null);
  const [rateInput, setRateInput] = useState("20");
  const [balanceInput, setBalanceInput] = useState("0");
  const [balanceNote, setBalanceNote] = useState("");
  const [ledger, setLedger] = useState<LedgerRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [okMsg, setOkMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [reason, setReason] = useState("");
  const [grantDays, setGrantDays] = useState(30);
  const [grantNote, setGrantNote] = useState("");
  const [tab, setTab] = useState<Tab>("profile");
  const [blockOpen, setBlockOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

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

  // useCallback (not a plain declaration) so the sessions column defs below stay
  // referentially stable across renders.
  const revokeOne = useCallback(
    async (sessionId: string) => {
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
    },
    [id, t, load],
  );

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

  async function setBypassVariantProgress(enabled: boolean) {
    setBusy(true);
    setError(null);
    setOkMsg(null);
    try {
      const res = await fetch(`/api/admin/users/${id}/bypass-variant-progress`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.message ?? t("errorAction"));
        return;
      }
      setOkMsg(enabled ? t("bypassOnDone") : t("bypassOffDone"));
      await load();
    } catch {
      setError(t("errorAction"));
    } finally {
      setBusy(false);
    }
  }

  async function hardDeleteUser(confirmation: string) {
    if (!user) return;
    setBusy(true);
    setError(null);
    setOkMsg(null);
    try {
      const res = await fetch(`/api/admin/users/${id}`, {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: confirmation }),
      });
      if (!res.ok) {
        setError(t("deleteError"));
        return;
      }
      setDeleteOpen(false);
      router.replace(`/${locale}/admin/users`);
    } catch {
      setError(t("deleteError"));
    } finally {
      setBusy(false);
    }
  }

  const sessionColumns = useMemo<ColumnDef<SessionRow>[]>(
    () => [
      {
        accessorKey: "id",
        header: t("colSession"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => <p className="font-mono text-xs">{row.original.id.slice(0, 8)}…</p>,
      },
      {
        accessorKey: "created_at",
        header: t("colCreated"),
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground">
            {new Date(row.original.created_at).toLocaleString(locale)}
          </span>
        ),
      },
      {
        accessorKey: "active",
        header: t("colStatus"),
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground">
            {row.original.active ? t("sessionActive") : t("sessionInactive")}
          </span>
        ),
      },
      {
        // Deliberately not `hideOnCard`: revoking a leaked session is incident
        // response, and the operator doing it is often holding a phone.
        id: "actions",
        header: t("colActions"),
        cell: ({ row }) =>
          row.original.active ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => void revokeOne(row.original.id)}
            >
              {t("revokeOne")}
            </Button>
          ) : null,
      },
    ],
    [t, locale, busy, revokeOne],
  );

  const entitlementColumns = useMemo<ColumnDef<EntitlementRow>[]>(
    () => [
      {
        accessorKey: "source",
        header: t("colSource"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <>
            <p className="font-semibold">{row.original.source}</p>
            {row.original.note ? (
              <p className="text-[11px] text-muted-foreground">{row.original.note}</p>
            ) : null}
          </>
        ),
      },
      {
        accessorKey: "starts_at",
        header: t("colStarts"),
        cell: ({ row }) => (
          <span className="text-xs">{new Date(row.original.starts_at).toLocaleString(locale)}</span>
        ),
      },
      {
        accessorKey: "ends_at",
        header: t("colEnds"),
        cell: ({ row }) => (
          <span className="text-xs">{new Date(row.original.ends_at).toLocaleString(locale)}</span>
        ),
      },
      {
        accessorKey: "active",
        header: t("colActive"),
        cell: ({ row }) => (
          <span className="text-xs font-bold uppercase">
            {row.original.active ? t("entitlementActive") : t("entitlementInactive")}
          </span>
        ),
      },
    ],
    [t, locale],
  );

  const ledgerColumns = useMemo<ColumnDef<LedgerRow>[]>(
    () => [
      {
        accessorKey: "entry_type",
        header: tr("colEntry"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">{row.original.entry_type}</span>
        ),
      },
      {
        accessorKey: "amount_uzs",
        header: tr("colAmount"),
        meta: { numeric: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <span
            className={`font-mono text-xs font-bold ${row.original.amount_uzs >= 0 ? "text-success" : "text-destructive"}`}
          >
            {row.original.amount_uzs >= 0 ? "+" : ""}
            {row.original.amount_uzs.toLocaleString()}
          </span>
        ),
      },
      {
        accessorKey: "created_at",
        header: tr("colCreated"),
        cell: ({ row }) => (
          <span className="text-[11px] text-muted-foreground">
            {new Date(row.original.created_at).toLocaleString(locale)}
          </span>
        ),
      },
    ],
    [tr, locale],
  );

  const refereeColumns = useMemo<ColumnDef<RefereeRow>[]>(
    () => [
      {
        accessorKey: "referee_id",
        header: tr("colUser"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <div>
            <p className="font-semibold">{row.original.referee_name || row.original.referee_phone}</p>
            <p className="text-xs text-muted-foreground">
              {row.original.referee_phone} · {row.original.status}
            </p>
          </div>
        ),
      },
      {
        accessorKey: "commission_uzs",
        header: tr("colCommission"),
        meta: { numeric: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <p className="font-mono text-sm">{row.original.commission_uzs.toLocaleString()} so&apos;m</p>
        ),
      },
    ],
    [tr],
  );

  if (error && !user) {
    return (
      <main className="mx-auto max-w-4xl space-y-3">
        <Link href={`/${locale}/admin/users`} className="text-sm font-semibold text-accent-ink hover:underline">
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
            className="inline-flex items-center gap-1 text-sm font-semibold text-accent-ink hover:underline"
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
                    blocked ? "bg-destructive/15 text-danger-ink" : "bg-success/15 text-success-ink"
                  }`}
                >
                  {user.status}
                </span>
                <span
                  className={`inline-flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-extrabold uppercase ${
                    user.vip_active ? "bg-accent/20 text-accent-ink" : "bg-muted text-muted-foreground"
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
              className={`min-h-[44px] rounded-lg px-3 py-1.5 text-xs font-bold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                tab === tb.id ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:bg-foreground/[0.04]"
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
            <AdminDataTable
              data={sessions ?? []}
              columns={sessionColumns}
              emptyTitle={t("sessionsEmpty")}
              getRowId={(row) => row.id}
              maxHeight={320}
            />
          </section>
        ) : null}

        {tab === "billing" ? (
          <div className="space-y-4">
            <PermissionGate permission="users.entitlements.grant" mode="hide">
              <section className="space-y-3 rounded-2xl border border-accent/25 bg-gradient-to-br from-accent/10 via-card/80 to-card p-4">
                <h2 className="text-xs font-extrabold uppercase tracking-wider text-accent-ink">
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
              <AdminDataTable
                data={user.entitlements ?? []}
                columns={entitlementColumns}
                emptyTitle={t("entitlementsEmpty")}
                getRowId={(row) => row.id}
                maxHeight={320}
              />
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
                    <p className="text-lg font-bold text-accent-ink">{referral.commission_percent}%</p>
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
                  <AdminDataTable
                    data={ledger}
                    columns={ledgerColumns}
                    emptyTitle={tr("ledgerEmpty")}
                    getRowId={(row) => row.id}
                    maxHeight={320}
                  />
                </div>
                <div>
                  <h3 className="mb-2 text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
                    {tr("refereesTitle")}
                  </h3>
                  <AdminDataTable
                    data={referral.referees}
                    columns={refereeColumns}
                    emptyTitle={tr("refereesEmpty")}
                    getRowId={(row) => row.referee_id}
                    maxHeight={320}
                  />
                </div>
              </>
            )}
          </section>
        ) : null}

        {tab === "activity" || tab === "learning" || tab === "notes" ? (
          <AdminEmptyState title={tabs.find((x) => x.id === tab)?.label || ""} description={t("comingTab")} />
        ) : null}

        {tab === "actions" ? (
          <div className="space-y-4">
            <PermissionGate permission="users.write" mode="hide">
              <section className="space-y-3 rounded-2xl border border-border/80 bg-card/70 p-4">
                <h2 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
                  {t("bypassTitle")}
                </h2>
                <p className="text-xs text-muted-foreground">{t("bypassHint")}</p>
                <p className="text-sm">
                  {t("bypassStatus")}:{" "}
                  <span className="font-semibold">
                    {user.bypass_variant_progress ? t("bypassOn") : t("bypassOff")}
                  </span>
                </p>
                <Button
                  type="button"
                  size="sm"
                  variant={user.bypass_variant_progress ? "outline" : "game"}
                  disabled={busy}
                  onClick={() => void setBypassVariantProgress(!user.bypass_variant_progress)}
                >
                  {user.bypass_variant_progress ? t("bypassDisable") : t("bypassEnable")}
                </Button>
              </section>
            </PermissionGate>
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
            <PermissionGate permission="users.hard_delete" mode="hide">
              <section className="space-y-3 rounded-2xl border border-destructive/45 bg-destructive/5 p-4">
                <h2 className="text-xs font-extrabold uppercase tracking-wider text-danger-ink">
                  {t("deleteZoneTitle")}
                </h2>
                <p className="text-sm font-semibold">{t("deleteTitle")}</p>
                <p className="text-xs text-muted-foreground">{t("deleteHint")}</p>
                <Button
                  type="button"
                  size="sm"
                  variant="destructive"
                  disabled={busy}
                  onClick={() => setDeleteOpen(true)}
                >
                  {t("deleteButton")}
                </Button>
              </section>
            </PermissionGate>
          </div>
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
        <DangerConfirm
          open={deleteOpen}
          onOpenChange={setDeleteOpen}
          title={t("deleteConfirmTitle")}
          warnings={[t("deleteWarnProfile"), t("deleteWarnRelated"), t("deleteWarnPermanent")]}
          confirmPhrase={user.phone}
          confirmAlternatives={["DELETE"]}
          confirmPhraseLabel={t("deletePhrase")}
          confirmLabel={t("deleteButton")}
          cancelLabel={t("cancel")}
          busy={busy}
          onConfirm={(confirmation) => void hardDeleteUser(confirmation)}
        />
      </main>
    </PermissionGate>
  );
}
