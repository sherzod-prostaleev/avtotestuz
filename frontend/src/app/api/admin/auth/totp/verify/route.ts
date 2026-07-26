import { NextResponse } from "next/server";
import { backendAdminFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import { setAdminAuthCookies, clearAdminAuthCookies } from "@/lib/admin-auth-cookies";

export const runtime = "nodejs";

type Payload = {
  data?: {
    tokens?: {
      access_token?: string;
      refresh_token?: string;
    };
  };
};

export async function POST(request: Request) {
  let body: string;
  try {
    body = await request.text();
  } catch {
    return NextResponse.json(
      { error: { code: "network_error", message: "service temporarily unavailable" } },
      { status: 502 },
    );
  }
  try {
    const backendRes = await backendAdminFetch("/auth/totp/verify", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
    });
    const data = await readBackendJson<Payload>(backendRes);
    const res = NextResponse.json(data, { status: backendRes.status });
    const tokens = data.data?.tokens;
    if (
      backendRes.ok &&
      tokens &&
      typeof tokens.access_token === "string" &&
      typeof tokens.refresh_token === "string"
    ) {
      setAdminAuthCookies(res, {
        accessToken: tokens.access_token,
        refreshToken: tokens.refresh_token,
      });
    } else {
      clearAdminAuthCookies(res);
    }
    return res;
  } catch {
    return NextResponse.json(
      { error: { code: "network_error", message: "service temporarily unavailable" } },
      { status: 502 },
    );
  }
}
