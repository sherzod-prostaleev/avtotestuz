"use client";

import { useEffect, useState } from "react";

/**
 * The classroom agent's own view of itself, served from 127.0.0.1 by
 * backend/station/internal/proxy at /__station/status.
 *
 * This is the only diagnostic surface a school has. The agent now runs with no
 * console window at all, so nothing it logs is visible on the machine, and the
 * page it serves is the sole place a problem can be reported. Everything here
 * is written by the agent already translated into Uzbek — the failures worth
 * showing ("this PC's clock is wrong", "this PC belongs to another school")
 * belong in the language of the person standing next to the machine.
 */
export type StationStatus = {
  version: string;
  phase: "starting" | "waiting" | "ready" | "blocked";
  org: string;
  station_id: string;
  label: string;
  enrolled: boolean;
  token_ok: boolean;
  problem: string;
  action: string;
  detail: string;
  code: string;
  log_path: string;
  listen_addr: string;
  update_state: string;
};

/**
 * useStationStatus polls the local agent while the kiosk cannot reach the API.
 *
 * Deliberately independent of apiGet: that goes through the agent to
 * drivergo.uz, which is exactly the path that is broken whenever this hook
 * matters. This one never leaves the machine.
 */
export function useStationStatus(active: boolean): StationStatus | null {
  const [status, setStatus] = useState<StationStatus | null>(null);

  useEffect(() => {
    if (!active) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const poll = async () => {
      try {
        const res = await fetch("/__station/status", { cache: "no-store" });
        if (res.ok) {
          const body = (await res.json()) as StationStatus;
          if (!cancelled) setStatus(body);
        }
      } catch {
        // The agent is gone entirely (someone ended the process, or it never
        // started). Leaving status null makes the page fall back to its
        // generic waiting screen, which is the honest answer: with no agent
        // there is nothing on this machine that knows anything.
      }
      if (!cancelled) timer = setTimeout(() => void poll(), 3000);
    };

    void poll();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [active]);

  return status;
}

/**
 * StationDiagnostics renders what the agent knows, for a screen someone is
 * standing in front of wondering why the lesson has not started.
 *
 * The raw error is shown rather than hidden. A photograph of this screen is
 * what a driving school actually sends to support, and a screenshot that says
 * only "connecting…" is worth nothing.
 */
export function StationDiagnostics({ status }: { status: StationStatus | null }) {
  if (!status) return null;
  const blocked = status.phase === "blocked";

  return (
    <div className="mx-auto w-full max-w-xl space-y-4 text-left">
      {status.problem ? (
        <div
          className={`rounded-2xl border p-4 ${
            blocked ? "border-destructive/40 bg-destructive/10" : "border-border bg-muted/40"
          }`}
        >
          <p className="font-semibold">{status.problem}</p>
          {status.action ? (
            <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{status.action}</p>
          ) : null}
        </div>
      ) : null}

      {status.update_state ? (
        <p className="text-sm text-muted-foreground">{status.update_state}</p>
      ) : null}

      <dl className="grid grid-cols-[auto,1fr] gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <dt>Dastur versiyasi</dt>
        <dd className="font-mono">{status.version}</dd>
        {status.org ? (
          <>
            <dt>Avtomaktab</dt>
            <dd className="truncate">{status.org}</dd>
          </>
        ) : null}
        {status.label ? (
          <>
            <dt>Kompyuter</dt>
            <dd className="truncate">{status.label}</dd>
          </>
        ) : null}
        {status.station_id ? (
          <>
            <dt>Stansiya ID</dt>
            <dd className="truncate font-mono">{status.station_id}</dd>
          </>
        ) : null}
        {status.log_path ? (
          <>
            <dt>Jurnal fayli</dt>
            <dd className="break-all font-mono">{status.log_path}</dd>
          </>
        ) : null}
      </dl>

      {status.detail ? (
        <details className="text-xs text-muted-foreground">
          <summary className="cursor-pointer">Texnik tafsilot (yordam uchun)</summary>
          <p className="mt-2 break-all font-mono">{status.detail}</p>
        </details>
      ) : null}
    </div>
  );
}
