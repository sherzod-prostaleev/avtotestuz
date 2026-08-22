"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

/**
 * What a classroom PC has reported about itself.
 *
 * The agent runs with no console window, so this and the kiosk screen are the
 * only places its state is visible. A school with thirty machines cannot be
 * asked to walk to each one and open a log file — and when that was the only
 * option, the one time it mattered the evidence had already been reinstalled
 * away.
 */
export type StationSummary = {
  last_phase?: string;
  last_code?: string;
  last_problem?: string;
  clock_offset_seconds?: number;
  last_diag_at?: string;
};

export type DiagRow = {
  created_at: string;
  station_id?: string;
  hwid_hash: string;
  label: string;
  agent_version: string;
  phase: string;
  code: string;
  problem: string;
  detail: string;
  clock_offset_seconds?: number;
  os: string;
  log_tail: string;
};

/** Colour by whether a human has to do something, not by severity in the abstract. */
function toneFor(phase?: string): string {
  if (phase === "blocked") return "text-destructive";
  if (phase === "waiting") return "text-warning";
  if (phase === "ready") return "text-success";
  return "text-muted-foreground";
}

function ClockNote({ seconds }: { seconds?: number }) {
  const t = useTranslations("AdminB2B");
  // Two minutes is the backend's tolerance (stationClockSkew); past it every
  // signature this PC makes looks replayed, which surfaces as an opaque
  // "unauthorized" that reads exactly like a revoked station.
  if (seconds === undefined || Math.abs(seconds) <= 120) return null;
  return (
    <p className="text-xs text-destructive">
      {t("stationClockSkew", { minutes: Math.round(Math.abs(seconds) / 60) })}
    </p>
  );
}

export function StationHealth({ station }: { station: StationSummary }) {
  const t = useTranslations("AdminB2B");
  if (!station.last_phase) {
    return <p className="text-xs text-muted-foreground">{t("stationNoReport")}</p>;
  }
  return (
    <div className="space-y-0.5">
      <p className={`text-xs font-semibold ${toneFor(station.last_phase)}`}>
        {t(`stationPhase_${station.last_phase}` as "stationPhase_ready")}
        {station.last_code ? <span className="ml-1 font-mono">({station.last_code})</span> : null}
      </p>
      {station.last_problem ? (
        <p className="max-w-xl text-xs text-muted-foreground">{station.last_problem}</p>
      ) : null}
      <ClockNote seconds={station.clock_offset_seconds} />
    </div>
  );
}

/**
 * Reports from machines in this school that never became stations.
 *
 * This is the half that could not exist before: a PC blocked at enrolment has
 * no station token, so it had no way to tell anyone why. Those are exactly the
 * failures a school phones about — the machine already registered elsewhere,
 * the school with no free seats, the installer key that expired.
 */
export function EnrollFailures({ orgId }: { orgId: string }) {
  const t = useTranslations("AdminB2B");
  const [rows, setRows] = useState<DiagRow[]>([]);
  const [open, setOpen] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(async () => {
    // Swallowed on purpose. This panel is a diagnostic aid bolted onto a page
    // whose real job is managing licences and seats; a diagnostics endpoint
    // that is unreachable, slow or not yet deployed must never be the reason
    // an operator cannot revoke a station.
    try {
      const res = await fetch(`/api/admin/b2b/orgs/${orgId}/enroll-failures`, {
        credentials: "include",
      });
      if (res.ok) {
        const body = (await res.json()) as { data?: unknown };
        // Shape-checked, not cast. The same assumption -- "the envelope will
        // contain what I expect" -- is what made every classroom PC render
        // "this page is for classroom computers only" against a perfectly good
        // /me response earlier in this file's history.
        setRows(Array.isArray(body.data) ? (body.data as DiagRow[]) : []);
      }
    } catch {
      // Nothing to show; the rest of the page carries on.
    } finally {
      setLoaded(true);
    }
  }, [orgId]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!loaded || rows.length === 0) return null;

  return (
    <section className="space-y-2 rounded-xl border border-destructive/40 bg-card p-4">
      <h2 className="text-sm font-bold">{t("enrollFailuresTitle")}</h2>
      <p className="text-xs text-muted-foreground">{t("enrollFailuresHint")}</p>
      <ul className="space-y-3">
        {rows.map((r, i) => {
          const key = `${r.hwid_hash}-${r.created_at}-${i}`;
          return (
            <li key={key} className="space-y-1 border-t border-border pt-2 text-sm first:border-0 first:pt-0">
              <p className="font-semibold">
                {r.label || t("stationUnnamed")}
                <span className="ml-2 font-mono text-xs text-muted-foreground">
                  {r.hwid_hash.slice(0, 12)}… {r.agent_version ? `· v${r.agent_version}` : null}
                </span>
              </p>
              <p className="text-xs text-destructive">
                {r.problem || r.code}
                {r.code ? <span className="ml-1 font-mono">({r.code})</span> : null}
              </p>
              <p className="text-xs text-muted-foreground">
                {new Date(r.created_at).toLocaleString()} · {r.os}
              </p>
              <ClockNote seconds={r.clock_offset_seconds} />
              {r.log_tail ? (
                <>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => setOpen(open === key ? null : key)}
                  >
                    {open === key ? t("hideLog") : t("showLog")}
                  </Button>
                  {open === key ? (
                    <pre className="max-h-80 overflow-auto rounded-lg bg-muted p-3 text-[11px] leading-snug">
                      {r.log_tail}
                    </pre>
                  ) : null}
                </>
              ) : null}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
