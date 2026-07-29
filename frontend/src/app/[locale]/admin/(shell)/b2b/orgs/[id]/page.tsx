"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

type Detail = {
  org: { id: string; name: string; status: string; members?: number; active_seats?: number };
  members: { profile_id: string; phone_masked: string; name: string; role: string }[];
  licenses: { id: string; seats: number; ends_at: string; active: boolean; note: string }[];
  seats_used?: number;
};

type OrgStats = {
  members_total: number;
  active_seats: number;
  seats_used: number;
  avg_readiness_pct: number;
  sessions_finished_7d: number;
  pending_invites: number;
};

export default function AdminB2BOrgDetailPage({ params }: { params: { id: string } }) {
  const t = useTranslations("AdminB2B");
  const locale = useLocale();
  const [data, setData] = useState<Detail | null>(null);
  const [stats, setStats] = useState<OrgStats | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [profileID, setProfileID] = useState("");
  const [invitePhone, setInvitePhone] = useState("");
  const [seats, setSeats] = useState("10");
  const [days, setDays] = useState("30");
  const [grantDays, setGrantDays] = useState("30");

  const load = useCallback(async () => {
    setError(null);
    try {
      const [res, statsRes] = await Promise.all([
        fetch(`/api/admin/b2b/orgs/${params.id}`, { cache: "no-store" }),
        fetch(`/api/admin/b2b/orgs/${params.id}/stats`, { cache: "no-store" }),
      ]);
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenRead") : t("errorLoad"));
        setData(null);
        return;
      }
      setData(json.data as Detail);
      if (statsRes.ok) {
        const sj = await statsRes.json();
        setStats(sj.data as OrgStats);
      }
    } catch {
      setError(t("errorLoad"));
      setData(null);
    }
  }, [params.id, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function addMember() {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/members`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ profile_id: profileID, role: "student" }),
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorCreate"));
      return;
    }
    setProfileID("");
    await load();
  }

  async function inviteByPhone() {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/invites`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ phone: invitePhone, role: "student" }),
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorCreate"));
      return;
    }
    setInvitePhone("");
    await load();
  }

  async function addLicense() {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/licenses`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ seats: Number(seats), days: Number(days), note: "admin" }),
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorCreate"));
      return;
    }
    await load();
  }

  async function grant(profileId: string) {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/members/${profileId}/grant`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ days: Number(grantDays), note: "admin ui" }),
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorGrant"));
      return;
    }
    await load();
  }

  async function removeMember(profileId: string) {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/members/${profileId}`, {
      method: "DELETE",
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorRemove"));
      return;
    }
    await load();
  }

  async function changeRole(profileId: string, role: string) {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/members/${profileId}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ role }),
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorRole"));
      return;
    }
    await load();
  }

  async function downloadCSV() {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/export-csv`);
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
  }

  return (
    <main className="mx-auto max-w-2xl space-y-4">
      <Link href={`/${locale}/admin/b2b/orgs`} className="text-sm font-semibold text-accent-ink">
        ← {t("back")}
      </Link>
      {!data ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : (
        <>
          <header>
            <h1 className="font-display text-2xl font-extrabold">{data.org.name}</h1>
            <p className="text-sm text-muted-foreground">
              {data.org.status} ·{" "}
              {t("membersSeats", {
                members: data.members.length,
                seats: data.org.active_seats ?? 0,
              })}
              {typeof data.seats_used === "number" ? ` · used ${data.seats_used}` : ""}
            </p>
          </header>

          {error ? <p className="text-sm text-destructive">{error}</p> : null}

          {stats ? (
            <section className="grid grid-cols-2 gap-2 rounded-xl border border-border bg-card p-4 text-xs sm:grid-cols-3">
              <p>
                {t("statMembers")}: <strong>{stats.members_total}</strong>
              </p>
              <p>
                {t("statSeats")}:{" "}
                <strong>
                  {stats.seats_used}/{stats.active_seats}
                </strong>
              </p>
              <p>
                {t("statReadiness")}: <strong>{stats.avg_readiness_pct}%</strong>
              </p>
              <p>
                {t("statSessions7d")}: <strong>{stats.sessions_finished_7d}</strong>
              </p>
              <p>
                {t("statPending")}: <strong>{stats.pending_invites}</strong>
              </p>
              <Button type="button" size="sm" variant="outline" onClick={() => void downloadCSV()}>
                {t("exportCsv")}
              </Button>
            </section>
          ) : null}

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold">{t("licenseTitle")}</h2>
            <div className="flex flex-wrap gap-2">
              <input
                value={seats}
                onChange={(e) => setSeats(e.target.value)}
                className="h-10 w-24 rounded-xl border border-border bg-background px-3 text-sm"
                placeholder="seats"
              />
              <input
                value={days}
                onChange={(e) => setDays(e.target.value)}
                className="h-10 w-24 rounded-xl border border-border bg-background px-3 text-sm"
                placeholder="days"
              />
              <Button type="button" size="sm" onClick={() => void addLicense()}>
                {t("addLicense")}
              </Button>
            </div>
            <ul className="space-y-1 text-xs">
              {data.licenses.map((l) => (
                <li key={l.id} className="font-mono">
                  {l.seats} seats · {l.active ? "active" : "ended"} · {new Date(l.ends_at).toLocaleDateString()}
                </li>
              ))}
            </ul>
          </section>

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold">{t("membersTitle")}</h2>
            <div className="flex flex-wrap gap-2">
              <input
                value={profileID}
                onChange={(e) => setProfileID(e.target.value)}
                placeholder={t("profilePlaceholder")}
                className="h-10 flex-1 rounded-xl border border-border bg-background px-3 text-sm"
              />
              <Button type="button" size="sm" onClick={() => void addMember()}>
                {t("addMember")}
              </Button>
            </div>
            <div className="flex flex-wrap gap-2">
              <input
                value={invitePhone}
                onChange={(e) => setInvitePhone(e.target.value)}
                placeholder={t("phonePlaceholder")}
                className="h-10 flex-1 rounded-xl border border-border bg-background px-3 text-sm"
              />
              <Button type="button" size="sm" onClick={() => void inviteByPhone()}>
                {t("invite")}
              </Button>
            </div>
            <div className="flex items-center gap-2 text-xs">
              <span>{t("grantDays")}</span>
              <input
                value={grantDays}
                onChange={(e) => setGrantDays(e.target.value)}
                className="h-8 w-20 rounded-lg border border-border bg-background px-2"
              />
            </div>
            <ul className="space-y-2">
              {data.members.map((m) => (
                <li key={m.profile_id} className="flex flex-wrap items-center justify-between gap-2 text-sm">
                  <div>
                    <p className="font-semibold">{m.name || m.phone_masked}</p>
                    <p className="font-mono text-xs text-muted-foreground">
                      {m.phone_masked} · {m.role}
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-1">
                    <select
                      value={m.role}
                      onChange={(e) => void changeRole(m.profile_id, e.target.value)}
                      className="h-8 rounded-lg border border-border bg-background px-1 text-xs"
                    >
                      <option value="student">student</option>
                      <option value="teacher">teacher</option>
                      <option value="owner">owner</option>
                    </select>
                    <Button type="button" size="sm" variant="outline" onClick={() => void grant(m.profile_id)}>
                      {t("grant")}
                    </Button>
                    <Button type="button" size="sm" variant="outline" onClick={() => void removeMember(m.profile_id)}>
                      {t("remove")}
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          </section>
        </>
      )}
    </main>
  );
}
