import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";

export const runtime = "nodejs";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
}

export async function PATCH(
  request: Request,
  context: { params: { provider: string } }
) {
  const token = request.headers.get("x-ops-token") ?? "";
  const provider = context.params.provider;
  if (provider !== "payme" && provider !== "click") {
    return NextResponse.json(
      { error: { code: "invalid_provider", message: "provider must be payme or click" } },
      { status: 400 }
    );
  }
  const body = await request.text();
  try {
    const backendRes = await backendFetch(`/ops/payment-providers/${provider}`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "X-Ops-Token": token,
      },
      body,
    });
    const data = await readBackendJson(backendRes);
    return NextResponse.json(data, { status: backendRes.status });
  } catch {
    return unavailableResponse();
  }
}
