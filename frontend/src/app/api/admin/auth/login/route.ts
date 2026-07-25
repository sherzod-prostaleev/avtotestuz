import { NextResponse } from "next/server";
import { backendAdminFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import { setAdminAuthCookies, clearAdminAuthCookies } from "@/lib/admin-auth-cookies";

export const runtime = "nodejs";

type AdminLoginPayload = {
  data?: {
    tokens?: {
      access_token?: string;
      refresh_token?: string;
    };
  };
};

function unavailable() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 },
  );
}

export async function POST(request: Request) {
  let body: string;
  try {
    body = await request.text();
  } catch {
    return unavailable();
  }
  try {
    const backendRes = await backendAdminFetch("/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
    });
    const data = await readBackendJson<AdminLoginPayload>(backendRes);
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
    return unavailable();
  }
}
