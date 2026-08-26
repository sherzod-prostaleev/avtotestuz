"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { DangerConfirm } from "@/components/admin/danger-confirm";
import { PermissionGate } from "@/components/admin/permission-gate";
import InstallerPanel from "./installer-panel";
import { EnrollFailures, StationHealth } from "./station-health";

type Station = {
  id: string;
  label: string;
  status: string;
  activated_at: string;
  last_seen_at: string;
  // What the PC itself last reported. Before this existed, thirty rows all
  // said "active" and a school with a broken machine had no way to say which.
  agent_version?: string;
  last_phase?: string;
  last_code?: string;
  last_problem?: string;
  clock_offset_seconds?: number;
  last_diag_at?: string;
};

type Detail = {
  org: { id: string; name: string; status: string; active_seats?: number };
  licenses: {
    id: string;
    seats: number;
    ends_at: string;
    active: boolean;
    note: string;
  }[];
  stations?: Station[];
  seats_used?: number;
};

type OrgStats = {
  active_seats: number;
  seats_used: number;
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
  const [seats, setSeats] = useState("30");
  const [days, setDays] = useState("365");
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

  async function addLicense() {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/licenses`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        seats: Number(seats),
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

  // The way back from revoke. The PC keeps its station id and its sealed key
  // across a revoke, so restoring the row is all it takes -- the agent on that
  // machine recovers by itself at its next token renewal, within two minutes,
  // with nobody going to the classroom. Before this existed, a school whose
  // PCs had been revoked needed an engineer with database access.
  //
  // A full licence is the one refusal worth naming separately: "could not
  // reactivate" would send an operator looking for a fault, when the answer is
  // to free a seat or buy one.
  async function reactivateStation(stationId: string) {
    setError(null);
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/stations/${stationId}/reactivate`, {
      method: "POST",
    });
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      setError(
        body?.error?.code === "seats_exhausted"
          ? t("errorReactivateSeats")
          : t("errorReactivateStation"),
      );
      return;
    }
    await load();
  }

  // Separate from revokeStation on purpose: revoke frees the seat and leaves
  // the PC in the list, purge erases the row and the shadow profile it
  // practised under. A confirm() because there is nothing to undo it with.
  async function deleteStation(stationId: string, label: string) {
    setError(null);
    if (!window.confirm(t("deleteStationConfirm", { label }))) return;
    const res = await fetch(`/api/admin/b2b/orgs/${params.id}/stations/${stationId}/purge`, {
      method: "DELETE",
    });
    if (!res.ok) {
      setError(t("errorDeleteStation"));
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
              {data.org.status === "active" ? t("statusActive") : t("statusSuspended")}
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
                {t("statStations")}:{" "}
                <strong>
                  {stats.seats_used}/{stats.active_seats}
                </strong>
              </p>
              {stats.license_expiring_soon ? (
                <p className="col-span-full text-amber-700 dark:text-amber-400">{t("licenseExpiringSoon")}</p>
              ) : null}
            </section>
          ) : null}

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold">{t("licenseTitle")}</h2>
            <p className="text-xs text-muted-foreground">{t("licenseHint")}</p>
            <div className="grid gap-2 sm:grid-cols-2">
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
                    status: l.active ? t("licenseActive") : t("licenseEnded"),
                    date: new Date(l.ends_at).toLocaleDateString(locale),
                  })}
                </li>
              ))}
            </ul>
          </section>

          <InstallerPanel orgId={params.id} />

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold">{t("stationsTitle")}</h2>
            <p className="text-xs text-muted-foreground">{t("stationsHint")}</p>
            <p className="text-xs text-muted-foreground">
              {t("stationsUsed", { used: activeStations.length })} / {data.org.active_seats ?? 0}
            </p>
            <ul className="space-y-2">
              {stations.map((s) => (
                <li key={s.id} className="flex flex-wrap items-start justify-between gap-2 text-sm">
                  <div className="min-w-0 space-y-1">
                    <p className="font-semibold">
                      {s.label} · {s.status === "active" ? t("stationActive") : t("stationRevoked")}
                      {s.agent_version ? (
                        <span className="ml-2 font-mono text-xs text-muted-foreground">
                          v{s.agent_version}
                        </span>
                      ) : null}
                    </p>
                    <StationHealth station={s} />
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {s.status === "active" ? (
                      <Button type="button" size="sm" variant="outline" onClick={() => void revokeStation(s.id)}>
                        {t("revokeStation")}
                      </Button>
                    ) : (
                      <Button type="button" size="sm" onClick={() => void reactivateStation(s.id)}>
                        {t("reactivateStation")}
                      </Button>
                    )}
                    <Button
                      type="button"
                      size="sm"
                      variant="destructive"
                      onClick={() => void deleteStation(s.id, s.label)}
                    >
                      {t("deleteStation")}
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          </section>

          <EnrollFailures orgId={params.id} />

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
