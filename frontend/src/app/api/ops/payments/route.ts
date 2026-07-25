import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";

export const runtime = "nodejs";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 },
  );
}

export async function GET(request: Request) {
  const token = request.headers.get("x-ops-token") ?? "";
  const url = new URL(request.url);
  const qs = url.searchParams.toString();
  const path = qs ? `/ops/payments?${qs}` : "/ops/payments";
  try {
    const backendRes = await backendFetch(path, {
      method: "GET",
      headers: { "Content-Type": "application/json", "X-Ops-Token": token },
    });
    const data = await readBackendJson(backendRes);
    return NextResponse.json(data, { status: backendRes.status });
  } catch {
    return unavailableResponse();
  }
}
