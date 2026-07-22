import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import { buildClientIPAssertionHeaders } from "@/lib/client-ip-assertion";

export const runtime = "nodejs";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
}

export async function POST(request: Request) {
  const body = await request.text();

  try {
    const backendRes = await backendFetch("/auth/otp/request", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...buildClientIPAssertionHeaders(request),
      },
      body,
    });
    const data = await readBackendJson(backendRes);
    return NextResponse.json(data, { status: backendRes.status });
  } catch {
    return unavailableResponse();
  }
}
