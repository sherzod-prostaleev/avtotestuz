import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import { setAuthCookies } from "@/lib/auth-cookies";
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
  let backendRes: Response;
  let data: unknown;

  try {
    backendRes = await backendFetch("/auth/set-password", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...buildClientIPAssertionHeaders(request),
      },
      body,
    });
    data = await readBackendJson(backendRes);
  } catch {
    return unavailableResponse();
  }

  if (!backendRes.ok) {
    return NextResponse.json(data, { status: backendRes.status });
  }

  let tokens: { accessToken: string; refreshToken: string };
  try {
    tokens = extractTokenPair(data);
  } catch {
    return unavailableResponse();
  }

  const response = NextResponse.json({ data: { ok: true } }, { status: 200 });
  setAuthCookies(response, tokens);
  return response;
}
