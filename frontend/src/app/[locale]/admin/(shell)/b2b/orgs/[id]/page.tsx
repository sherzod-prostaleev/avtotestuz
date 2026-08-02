"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { DangerConfirm } from "@/components/admin/danger-confirm";
import { PermissionGate } from "@/components/admin/permission-gate";

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
  const router = useRouter();
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
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

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
    if (!res.ok) {
      setError(t("errorCreate"));
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
    if (!res.ok) {
      setError(t("errorCreate"));
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
        note: t("licenseAuditNote"),
      }),
    });
    if (!res.ok) {
      setError(t("errorCreate"));
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
      setError(t("errorStationCode"));
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
    if (!res.ok) {
      setError(t("errorRevokeStation"));
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
    if (!res.ok) {
      setError(t("errorStatus"));
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
    if (!res.ok) {
      setError(t("errorPartnerPromo"));
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
      body: JSON.stringify({ days: Number(grantDays), note: t("homeGrantAuditNote") }),
    });
    if (!res.ok) {
      setError(t("errorGrant"));
      return;
    }
    await load();
  }

  async function removeMember(profileId: string) {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/members/${profileId}`, {
      method: "DELETE",
    });
    if (!res.ok) {
      setError(t("errorRemove"));
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
    if (!res.ok) {
      setError(t("errorRole"));
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

  async function hardDeleteOrg(confirmation: string) {
    if (!data) return;
    setDeleting(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/b2b/orgs/${params.id}`, {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: confirmation }),
      });
      if (!res.ok) {
        setError(t("deleteError"));
        return;
      }
      setDeleteOpen(false);
      router.replace(`/${locale}/admin/b2b/orgs`);
    } catch {
      setError(t("deleteError"));
    } finally {
      setDeleting(false);
    }
  }

  function roleLabel(role: string) {
    if (role === "owner") return t("roleOwner");
    if (role === "teacher") return t("roleTeacher");
    return t("roleStudent");
  }

  const stations = data?.stations ?? [];
  const activeStations = stations.filter((s) => s.status === "active");

  return (
    <main className="mx-auto max-w-2xl space-y-4">
      <Link href={`/${locale}/admin/b2b/orgs`} className="text-sm font-semibold text-accent-ink">
        ← {t("back")}
      </Link>
      {!data ? (
        error ? (
          <p className="text-sm text-destructive">{error}</p>
        ) : (
          <p className="text-sm text-muted-foreground">{t("loading")}</p>
        )
      ) : (
        <>
          <header>
            <h1 className="font-display text-2xl font-extrabold">{data.org.name}</h1>
            <p className="text-sm text-muted-foreground">
              {data.org.status === "active" ? t("statusActive") : t("statusSuspended")} ·{" "}
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

          <section className="space-y-2 rounded-xl border border-accent/30 bg-accent/5 p-4">
            <h2 className="text-sm font-bold">{t("multiPcTitle")}</h2>
            <p className="text-xs text-muted-foreground">{t("multiPcHint")}</p>
            <ol className="list-decimal space-y-1 pl-5 text-xs text-muted-foreground">
              <li>{t("multiPcStepLicense")}</li>
              <li>{t("multiPcStepCode")}</li>
              <li>{t("multiPcStepLearner")}</li>
              <li>{t("multiPcStepSession")}</li>
            </ol>
          </section>

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
            <div className="grid gap-2 sm:grid-cols-3">
              <label className="space-y-1 text-xs font-semibold text-muted-foreground">
                <span>{t("licensePcLimit")}</span>
                <input
                  type="number"
                  min={1}
                  value={seats}
                  onChange={(e) => setSeats(e.target.value)}
                  className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
                />
              </label>
              <label className="space-y-1 text-xs font-semibold text-muted-foreground">
                <span>{t("licenseHomeLimit")}</span>
                <input
                  type="number"
                  min={0}
                  value={homeSeats}
                  onChange={(e) => setHomeSeats(e.target.value)}
                  className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
                />
              </label>
              <label className="space-y-1 text-xs font-semibold text-muted-foreground">
                <span>{t("licenseDays")}</span>
                <input
                  type="number"
                  min={1}
                  value={days}
                  onChange={(e) => setDays(e.target.value)}
                  className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
                />
              </label>
            </div>
            <div>
              <Button type="button" size="sm" onClick={() => void addLicense()}>
                {t("addLicense")}
              </Button>
            </div>
            <ul className="space-y-1 text-xs">
              {data.licenses.map((l) => (
                <li key={l.id} className="font-mono">
                  {t("licenseSummary", {
                    pcs: l.seats,
                    homes: l.home_seats,
                    status: l.active ? t("licenseActive") : t("licenseEnded"),
                    date: new Date(l.ends_at).toLocaleDateString(locale),
                  })}
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
                      {s.label} · {s.status === "active" ? t("stationActive") : t("stationRevoked")}
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
              <label className="space-y-1 text-xs font-semibold text-muted-foreground">
                <span>{t("promoPercent")}</span>
                <input
                  type="number"
                  min={1}
                  max={100}
                  value={promoValue}
                  onChange={(e) => setPromoValue(e.target.value)}
                  className="h-10 w-24 rounded-xl border border-border bg-background px-3 text-sm"
                />
              </label>
              <Button type="button" size="sm" onClick={() => void createPartnerPromo()}>
                {t("createPartnerPromo")}
              </Button>
            </div>
          </section>

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold">{t("membersTitle")}</h2>
            <p className="text-xs text-muted-foreground">{t("membersHint")}</p>
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
                      {m.phone_masked} · {roleLabel(m.role)}
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-1">
                    <select
                      value={m.role}
                      onChange={(e) => void changeRole(m.profile_id, e.target.value)}
                      className="h-8 rounded-lg border border-border bg-background px-1 text-xs"
                    >
                      <option value="student">{t("roleStudent")}</option>
                      <option value="teacher">{t("roleTeacher")}</option>
                      <option value="owner">{t("roleOwner")}</option>
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
            <p className="text-xs text-muted-foreground">{t("homeGrantHint")}</p>
          </section>

          <PermissionGate permission="b2b.orgs.hard_delete" mode="hide">
            <section className="space-y-3 rounded-xl border border-destructive/45 bg-destructive/5 p-4">
              <h2 className="text-xs font-extrabold uppercase tracking-wider text-danger-ink">
                {t("deleteZoneTitle")}
              </h2>
              <p className="text-sm font-semibold">{t("deleteTitle")}</p>
              <p className="text-xs text-muted-foreground">{t("deleteHint")}</p>
              <Button type="button" size="sm" variant="destructive" onClick={() => setDeleteOpen(true)}>
                {t("deleteButton")}
              </Button>
            </section>
          </PermissionGate>

          <DangerConfirm
            open={deleteOpen}
            onOpenChange={setDeleteOpen}
            title={t("deleteConfirmTitle")}
            warnings={[
              t("deleteWarnStations", { count: stations.length }),
              t("deleteWarnMembers", { count: data.members.length }),
              t("deleteWarnRelated"),
            ]}
            confirmPhrase={data.org.name}
            confirmPhraseLabel={t("deletePhrase")}
            confirmLabel={t("deleteButton")}
            cancelLabel={t("cancel")}
            busy={deleting}
            onConfirm={(confirmation) => void hardDeleteOrg(confirmation)}
          />
        </>
      )}
    </main>
  );
}
