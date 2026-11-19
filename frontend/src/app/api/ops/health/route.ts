import { NextResponse } from "next/server";
import { backendRootFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import { ADMIN_AUTH_COOKIE, ADMIN_REFRESH_COOKIE } from "@/lib/admin-auth-cookies";
import { readCookie } from "@/lib/auth-cookies";

export const runtime = "nodejs";

type ProbeResult = {
  ok: boolean;
  status: number;
  data: unknown;
  error?: string;
};

async function probe(path: string): Promise<ProbeResult> {
  try {
    const res = await backendRootFetch(path, {
      method: "GET",
      headers: { Accept: "application/json" },
      cache: "no-store",
    });
    const data = await readBackendJson(res);
    return { ok: res.ok, status: res.status, data };
  } catch {
    return {
      ok: false,
      status: 502,
      data: null,
      error: "unreachable",
    };
  }
}

/**
 * Aggregates API liveness + readiness + process metrics for admin ops monitoring.
 * Requires an active admin session cookie so internal metrics and live readiness
 * probes cannot be triggered anonymously from the public internet.
 */
export async function GET(request?: Request) {
  if (request) {
    const hasAdminSession = Boolean(
      readCookie(request, ADMIN_AUTH_COOKIE) || readCookie(request, ADMIN_REFRESH_COOKIE),
    );
    if (!hasAdminSession) {
      return NextResponse.json(
        { error: { code: "unauthorized", message: "admin authentication required" } },
        { status: 401 },
      );
    }
  }
  const [live, ready, metrics] = await Promise.all([
    probe("/healthz"),
    probe("/readyz"),
    probe("/metrics"),
  ]);
  const overallOk = live.ok && ready.ok;
  return NextResponse.json(
    {
      data: {
        status: overallOk ? "ok" : "degraded",
        checked_at: new Date().toISOString(),
        live,
        ready,
        metrics,
      },
    },
    { status: overallOk ? 200 : 503 }
  );
}
