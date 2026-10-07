import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import { cookieModeFor, setAuthCookies } from "@/lib/auth-cookies";
import { rejectCrossSite } from "@/lib/same-origin";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
}

export async function POST(request: Request) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;

  const body = await request.text();
  let backendRes: Response;
  let data: unknown;

  try {
    backendRes = await backendFetch("/auth/otp/verify", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
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
  // OTP carries no Telegram launch data; keep whatever jar the caller has.
  setAuthCookies(response, tokens, cookieModeFor(request));
  return response;
}
