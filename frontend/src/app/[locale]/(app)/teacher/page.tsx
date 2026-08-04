"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, Download, Users } from "lucide-react";
import { apiDelete, apiGet, apiPatch, apiPost } from "@/lib/api-client";
import { Button } from "@/components/ui/button";

type OrgSummary = {
  id: string;
  name: string;
  status: string;
  my_role: string;
  member_count: number;
  active_seats: number;
  seats_used?: number;
};

type Member = {
  profile_id: string;
  phone_masked: string;
  name: string;
  role: string;
  sessions_count?: number;
  readiness_pct?: number;
  streak_current?: number;
  has_b2b_vip?: boolean;
};

type OrgDetail = {
  org: OrgSummary;
  members: Member[];
  licenses: { id: string; seats: number; ends_at: string; active: boolean; note: string }[];
  seats_used?: number;
};

type OrgStats = {
  members_total: number;
  owners: number;
  teachers: number;
  students: number;
  active_seats: number;
  seats_used: number;
  seats_remaining: number;
  avg_readiness_pct: number;
  sessions_finished_7d: number;
  sessions_finished_30d: number;
  active_members_7d: number;
  pending_invites: number;
  license_expiring_soon?: boolean;
  license_ends_at?: string;
};

type Station = {
  id: string;
  label: string;
  status: string;
  last_seen_at?: string;
};

type InviteRow = {
  id: string;
  token?: string;
  phone_masked: string;
  role: string;
  expires_at: string;
};

export default function TeacherPage() {
  const t = useTranslations("Teacher");
  const locale = useLocale();
  const [orgs, setOrgs] = useState<OrgSummary[] | null>(null);
  const [selected, setSelected] = useState<OrgDetail | null>(null);
  const [stats, setStats] = useState<OrgStats | null>(null);
  const [invites, setInvites] = useState<InviteRow[]>([]);
  const [phone, setPhone] = useState("");
  const [role, setRole] = useState("student");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [stations, setStations] = useState<Station[]>([]);

  const openOrg = useCallback(
    async (id: string) => {
      setError(null);
      try {
        const [detail, st, inv, stns] = await Promise.all([
          apiGet<OrgDetail>(`me/teacher/orgs/${id}`),
          apiGet<OrgStats>(`me/teacher/orgs/${id}/stats`),
          apiGet<InviteRow[]>(`me/teacher/orgs/${id}/invites`),
          apiGet<Station[]>(`me/teacher/orgs/${id}/stations`),
        ]);
        setSelected(detail);
        setStats(st);
        setInvites(inv);
        setStations(stns);
      } catch {
        setError(t("errorLoad"));
      }
    },
    [t],
  );

  const loadOrgs = useCallback(async () => {
    setError(null);
    try {
      const data = await apiGet<OrgSummary[]>("me/teacher/orgs");
      setOrgs(data);
      if (data.length === 1) {
        await openOrg(data[0].id);
      }
    } catch {
      setError(t("errorLoad"));
      setOrgs([]);
    }
  }, [t, openOrg]);

  useEffect(() => {
    void loadOrgs();
  }, [loadOrgs]);

  async function refreshSelected() {
    if (!selected) return;
    await openOrg(selected.org.id);
  }

  async function invite() {
    if (!selected || !phone.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await apiPost(`me/teacher/orgs/${selected.org.id}/invites`, { phone: phone.trim(), role });
      setPhone("");
      await refreshSelected();
    } catch {
      setError(t("errorInvite"));
    } finally {
      setBusy(false);
    }
  }

  async function removeMember(profileId: string) {
    if (!selected) return;
    setBusy(true);
    setError(null);
    try {
      await apiDelete(`me/teacher/orgs/${selected.org.id}/members/${profileId}`);
      await refreshSelected();
    } catch {
      setError(t("errorRemove"));
    } finally {
      setBusy(false);
    }
  }

  async function changeRole(profileId: string, next: string) {
    if (!selected) return;
    setBusy(true);
    setError(null);
    try {
      await apiPatch(`me/teacher/orgs/${selected.org.id}/members/${profileId}`, { role: next });
      await refreshSelected();
    } catch {
      setError(t("errorRole"));
    } finally {
      setBusy(false);
    }
  }

  async function downloadCSV() {
    if (!selected) return;
    setError(null);
    try {
      const res = await fetch(`/api/proxy/me/teacher/orgs/${selected.org.id}/export.csv`);
      if (!res.ok) {
        setError(t("errorExport"));
        return;
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "org-members.csv";
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      setError(t("errorExport"));
    }
  }

  async function revokeStation(stationId: string) {
    if (!selected) return;
    setBusy(true);
    setError(null);
    try {
      await apiDelete(`me/teacher/orgs/${selected.org.id}/stations/${stationId}`);
      await refreshSelected();
    } catch {
      setError(t("errorRevokeStation"));
    } finally {
      setBusy(false);
    }
  }

  const isOwner = selected?.org.my_role === "owner";

  function roleLabel(value: string) {
    if (value === "owner") return t("roleOwner");
    if (value === "teacher") return t("roleTeacher");
    return t("roleStudent");
  }

  return (
    <main className="mx-auto max-w-2xl space-y-4 px-4 py-8">
      <Link
        href={`/${locale}/profile`}
        className="inline-flex items-center gap-2 text-sm font-bold text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden /> {t("back")}
      </Link>
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("note")}</p>
      </header>

      <section className="space-y-2 rounded-xl border border-accent/30 bg-accent/5 p-4">
        <h2 className="text-sm font-bold">{t("multiPcTitle")}</h2>
        <p className="text-xs text-muted-foreground">{t("multiPcHint")}</p>
      </section>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!orgs ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : orgs.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("empty")}</p>
      ) : (
        <ul className="space-y-2">
          {orgs.map((o) => (
            <li key={o.id}>
              <button
                type="button"
                onClick={() => void openOrg(o.id)}
                className={`w-full rounded-xl border px-4 py-3 text-left transition-colors ${
                  selected?.org.id === o.id
                    ? "border-accent bg-accent/10"
                    : "border-border bg-card hover:border-accent/40"
                }`}
              >
                <p className="font-semibold">{o.name}</p>
                <p className="text-xs text-muted-foreground">
                  {roleLabel(o.my_role)} · {t("membersSeats", { members: o.member_count, seats: o.active_seats })}
                </p>
              </button>
            </li>
          ))}
        </ul>
      )}

      {selected ? (
        <section className="space-y-4 rounded-xl border border-border bg-card p-4">
          {stats ? (
            <div className="grid grid-cols-2 gap-2 text-xs sm:grid-cols-3">
              <p>
                <span className="text-muted-foreground">{t("statMembers")}</span>{" "}
                <strong>{stats.members_total}</strong>
              </p>
              <p>
                <span className="text-muted-foreground">{t("statSeats")}</span>{" "}
                <strong>
                  {stats.seats_used}/{stats.active_seats}
                </strong>
              </p>
              <p>
                <span className="text-muted-foreground">{t("statReadiness")}</span>{" "}
                <strong>{stats.avg_readiness_pct}%</strong>
              </p>
              <p>
                <span className="text-muted-foreground">{t("statActive7d")}</span>{" "}
                <strong>{stats.active_members_7d}</strong>
              </p>
              <p>
                <span className="text-muted-foreground">{t("statSessions7d")}</span>{" "}
                <strong>{stats.sessions_finished_7d}</strong>
              </p>
              <p>
                <span className="text-muted-foreground">{t("statPending")}</span>{" "}
                <strong>{stats.pending_invites}</strong>
              </p>
              {stats.license_expiring_soon ? (
                <p className="col-span-full text-amber-700 dark:text-amber-400">{t("licenseExpiringSoon")}</p>
              ) : null}
            </div>
          ) : null}

          <div className="space-y-2 border-t border-border pt-3">
            <h3 className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t("stationsTitle")}
            </h3>
            <p className="text-xs text-muted-foreground">{t("stationsHint")}</p>
            <ul className="space-y-2">
              {stations.map((s) => (
                <li key={s.id} className="flex flex-wrap items-center justify-between gap-2 text-sm">
                  <div>
                    <p className="font-semibold">
                      {s.label} · {s.status === "active" ? t("stationActive") : t("stationRevoked")}
                    </p>
                  </div>
                  {s.status === "active" ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={busy}
                      onClick={() => void revokeStation(s.id)}
                    >
                      {t("revokeStation")}
                    </Button>
                  ) : null}
                </li>
              ))}
            </ul>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <div className="flex items-center gap-2">
              <Users className="h-4 w-4 text-muted-foreground" aria-hidden />
              <h2 className="text-sm font-bold">{t("membersTitle")}</h2>
            </div>
            <Button type="button" size="sm" variant="outline" onClick={() => void downloadCSV()}>
              <Download className="mr-1 h-3.5 w-3.5" aria-hidden />
              {t("exportCsv")}
            </Button>
          </div>

          <div className="flex flex-wrap gap-2">
            <input
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder={t("phonePlaceholder")}
              className="h-10 flex-1 rounded-xl border border-border bg-background px-3 text-sm"
            />
            <select
              value={role}
              onChange={(e) => setRole(e.target.value)}
              className="h-10 rounded-xl border border-border bg-background px-2 text-sm"
            >
              <option value="student">{t("roleStudent")}</option>
              <option value="teacher">{t("roleTeacher")}</option>
              {isOwner ? <option value="owner">{t("roleOwner")}</option> : null}
            </select>
            <Button type="button" size="sm" disabled={busy} onClick={() => void invite()}>
              {t("invite")}
            </Button>
          </div>

          <ul className="space-y-2">
            {selected.members.map((m) => (
              <li key={m.profile_id} className="flex flex-wrap items-center justify-between gap-2 text-sm">
                <div>
                  <p className="font-semibold">{m.name || m.phone_masked}</p>
                  <p className="font-mono text-xs text-muted-foreground">
                    {m.phone_masked} · {roleLabel(m.role)}
                    {typeof m.readiness_pct === "number" ? ` · ${m.readiness_pct}%` : ""}
                    {m.has_b2b_vip ? ` · ${t("vipLabel")}` : ""}
                  </p>
                </div>
                <div className="flex flex-wrap gap-1">
                  {isOwner && m.role !== "owner" ? (
                    <select
                      value={m.role}
                      disabled={busy}
                      onChange={(e) => void changeRole(m.profile_id, e.target.value)}
                      className="h-8 rounded-lg border border-border bg-background px-1 text-xs"
                    >
                      <option value="student">{t("roleStudent")}</option>
                      <option value="teacher">{t("roleTeacher")}</option>
                      <option value="owner">{t("roleOwner")}</option>
                    </select>
                  ) : null}
                  {(isOwner || m.role === "student") &&
                  !(m.role === "owner" && selected.members.filter((x) => x.role === "owner").length <= 1) ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={busy}
                      onClick={() => void removeMember(m.profile_id)}
                    >
                      {t("remove")}
                    </Button>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>

          {invites.length > 0 ? (
            <div>
              <h3 className="mb-1 text-xs font-bold uppercase tracking-wider text-muted-foreground">
                {t("pendingInvites")}
              </h3>
              <ul className="space-y-1 text-xs font-mono text-muted-foreground">
                {invites.map((i) => (
                  <li key={i.id}>
                    {i.phone_masked} · {roleLabel(i.role)} · {new Date(i.expires_at).toLocaleDateString(locale)}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}

          <div>
            <h3 className="mb-1 text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t("licensesTitle")}
            </h3>
            {selected.licenses.length === 0 ? (
              <p className="text-xs text-muted-foreground">{t("noLicenses")}</p>
            ) : (
              <ul className="space-y-1 text-xs font-mono text-muted-foreground">
                {selected.licenses.map((l) => (
                  <li key={l.id}>
                    {t("licenseSummary", {
                      pcs: l.seats,
                      status: l.active ? t("licenseActive") : t("licenseEnded"),
                      date: new Date(l.ends_at).toLocaleDateString(locale),
                    })}
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>
      ) : null}
    </main>
  );
}
