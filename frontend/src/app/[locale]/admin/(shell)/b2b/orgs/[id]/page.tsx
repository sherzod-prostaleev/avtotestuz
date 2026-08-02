"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

type Station = {
  id: string;
  fingerprint: string;
  label: string;
  status: string;
  activated_at: string;
  last_seen_at: string;
};

type Detail = {
  org: { id: string; name: string; status: string; members?: number; active_seats?: number };
  members: { profile_id: string; phone_masked: string; name: string; role: string }[];
  licenses: {
    id: string;
    seats: number;
    home_seats: number;
    ends_at: string;
    active: boolean;
    note: string;
  }[];
  stations?: Station[];
  seats_used?: number;
  home_seats?: number;
  home_seats_used?: number;
};

type OrgStats = {
  members_total: number;
  active_seats: number;
  seats_used: number;
  home_seats?: number;
  home_seats_used?: number;
  avg_readiness_pct: number;
  sessions_finished_7d: number;
  pending_invites: number;
  license_expiring_soon?: boolean;
  license_ends_at?: string;
};

export default function AdminB2BOrgDetailPage() {
  const t = useTranslations("AdminB2B");
  const locale = useLocale();
  const params = useParams<{ id: string }>();
  const [data, setData] = useState<Detail | null>(null);
  const [stats, setStats] = useState<OrgStats | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [profileID, setProfileID] = useState("");
  const [invitePhone, setInvitePhone] = useState("");
  const [seats, setSeats] = useState("30");
  const [homeSeats, setHomeSeats] = useState("0");
  const [days, setDays] = useState("365");
  const [grantDays, setGrantDays] = useState("30");
  const [stationLabel, setStationLabel] = useState("");
  const [lastCode, setLastCode] = useState<string | null>(null);
  const [promoCode, setPromoCode] = useState("");
  const [promoValue, setPromoValue] = useState("20");

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
      body: JSON.stringify({
        seats: Number(seats),
        home_seats: Number(homeSeats) || 0,
        days: Number(days),
        note: "admin classroom",
      }),
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorCreate"));
      return;
    }
    await load();
  }

  async function createStationCode() {
    setError(null);
    setLastCode(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/station-codes`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ label: stationLabel || "PC", ttl_hours: 168 }),
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorStationCode"));
      return;
    }
    setLastCode(json.data?.code ?? null);
    setStationLabel("");
    await load();
  }

  async function revokeStation(stationId: string) {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/stations/${stationId}`, {
      method: "DELETE",
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorRevokeStation"));
      return;
    }
    await load();
  }

  async function setStatus(status: string) {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/status`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ status }),
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorStatus"));
      return;
    }
    await load();
  }

  async function createPartnerPromo() {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/partner-promos`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        code: promoCode.trim(),
        kind: "percent",
        value: Number(promoValue) || 20,
        valid_days: 90,
      }),
    });
    const json = await res.json();
    if (!res.ok) {
      setError(json?.error?.message ?? t("errorPartnerPromo"));
      return;
    }
    setPromoCode("");
    await load();
  }

  async function grantHome(profileId: string) {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/members/${profileId}/grant`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ days: Number(grantDays), note: "home seat" }),
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

  const stations = data?.stations ?? [];
  const activeStations = stations.filter((s) => s.status === "active");

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
              {typeof data.seats_used === "number"
                ? ` · ${t("stationsUsed", { used: data.seats_used })}`
                : ""}
            </p>
            <div className="mt-2 flex flex-wrap gap-2">
              {data.org.status === "active" ? (
                <Button type="button" size="sm" variant="outline" onClick={() => void setStatus("suspended")}>
                  {t("suspend")}
                </Button>
              ) : (
                <Button type="button" size="sm" onClick={() => void setStatus("active")}>
                  {t("activateOrg")}
                </Button>
              )}
            </div>
          </header>

          {error ? <p className="text-sm text-destructive">{error}</p> : null}

          {stats ? (
            <section className="grid grid-cols-2 gap-2 rounded-xl border border-border bg-card p-4 text-xs sm:grid-cols-3">
              <p>
                {t("statMembers")}: <strong>{stats.members_total}</strong>
              </p>
              <p>
                {t("statStations")}:{" "}
                <strong>
                  {stats.seats_used}/{stats.active_seats}
                </strong>
              </p>
              <p>
                {t("statHomeSeats")}:{" "}
                <strong>
                  {stats.home_seats_used ?? data.home_seats_used ?? 0}/{stats.home_seats ?? data.home_seats ?? 0}
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
              {stats.license_expiring_soon ? (
                <p className="col-span-full text-amber-700 dark:text-amber-400">{t("licenseExpiringSoon")}</p>
              ) : null}
              <Button type="button" size="sm" variant="outline" onClick={() => void downloadCSV()}>
                {t("exportCsv")}
              </Button>
            </section>
          ) : null}

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold">{t("licenseTitle")}</h2>
            <p className="text-xs text-muted-foreground">{t("licenseHint")}</p>
            <div className="flex flex-wrap gap-2">
              <input
                value={seats}
                onChange={(e) => setSeats(e.target.value)}
                className="h-10 w-24 rounded-xl border border-border bg-background px-3 text-sm"
                placeholder={t("seatsPlaceholder")}
              />
              <input
                value={homeSeats}
                onChange={(e) => setHomeSeats(e.target.value)}
                className="h-10 w-28 rounded-xl border border-border bg-background px-3 text-sm"
                placeholder={t("homeSeatsPlaceholder")}
              />
              <input
                value={days}
                onChange={(e) => setDays(e.target.value)}
                className="h-10 w-24 rounded-xl border border-border bg-background px-3 text-sm"
                placeholder={t("daysPlaceholder")}
              />
              <Button type="button" size="sm" onClick={() => void addLicense()}>
                {t("addLicense")}
              </Button>
            </div>
            <ul className="space-y-1 text-xs">
              {data.licenses.map((l) => (
                <li key={l.id} className="font-mono">
                  {l.seats} stations · {l.home_seats} home · {l.active ? "active" : "ended"} ·{" "}
                  {new Date(l.ends_at).toLocaleDateString()}
                </li>
              ))}
            </ul>
          </section>

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold">{t("stationsTitle")}</h2>
            <p className="text-xs text-muted-foreground">{t("stationsHint")}</p>
            <div className="flex flex-wrap gap-2">
              <input
                value={stationLabel}
                onChange={(e) => setStationLabel(e.target.value)}
                placeholder={t("stationLabelPlaceholder")}
                className="h-10 flex-1 rounded-xl border border-border bg-background px-3 text-sm"
              />
              <Button type="button" size="sm" onClick={() => void createStationCode()}>
                {t("createStationCode")}
              </Button>
            </div>
            {lastCode ? (
              <p className="rounded-lg bg-accent/10 px-3 py-2 font-mono text-sm font-bold">
                {t("stationCodeReady")}: {lastCode}
              </p>
            ) : null}
            <p className="text-xs text-muted-foreground">
              {t("stationsUsed", { used: activeStations.length })} / {data.org.active_seats ?? 0}
            </p>
            <ul className="space-y-2">
              {stations.map((s) => (
                <li key={s.id} className="flex flex-wrap items-center justify-between gap-2 text-sm">
                  <div>
                    <p className="font-semibold">
                      {s.label} · {s.status}
                    </p>
                    <p className="font-mono text-xs text-muted-foreground">{s.fingerprint}</p>
                  </div>
                  {s.status === "active" ? (
                    <Button type="button" size="sm" variant="outline" onClick={() => void revokeStation(s.id)}>
                      {t("revokeStation")}
                    </Button>
                  ) : null}
                </li>
              ))}
            </ul>
          </section>

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold">{t("partnerPromoTitle")}</h2>
            <p className="text-xs text-muted-foreground">{t("partnerPromoHint")}</p>
            <div className="flex flex-wrap gap-2">
              <input
                value={promoCode}
                onChange={(e) => setPromoCode(e.target.value)}
                placeholder={t("partnerPromoCode")}
                className="h-10 flex-1 rounded-xl border border-border bg-background px-3 font-mono text-sm"
              />
              <input
                value={promoValue}
                onChange={(e) => setPromoValue(e.target.value)}
                className="h-10 w-20 rounded-xl border border-border bg-background px-3 text-sm"
                placeholder="%"
              />
              <Button type="button" size="sm" onClick={() => void createPartnerPromo()}>
                {t("createPartnerPromo")}
              </Button>
            </div>
          </section>

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold">{t("membersTitle")}</h2>
            <p className="text-xs text-muted-foreground">{t("homeGrantHint")}</p>
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
                    <Button type="button" size="sm" variant="outline" onClick={() => void grantHome(m.profile_id)}>
                      {t("grantHome")}
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
