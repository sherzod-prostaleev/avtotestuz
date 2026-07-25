import { NextResponse } from "next/server";
import { backendRootFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";

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
 * Aggregates public API liveness + readiness for the thin ops health stub
 * (M3 monitoring precursor — no admin RBAC yet).
 */
export async function GET() {
  const [live, ready] = await Promise.all([probe("/healthz"), probe("/readyz")]);
  const overallOk = live.ok && ready.ok;
  return NextResponse.json(
    {
      data: {
        status: overallOk ? "ok" : "degraded",
        checked_at: new Date().toISOString(),
        live,
        ready,
      },
    },
    { status: overallOk ? 200 : 503 }
  );
}
