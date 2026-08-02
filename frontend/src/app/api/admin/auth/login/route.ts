import { NextResponse } from "next/server";
import { backendAdminFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import {
  clearAdminAuthCookies,
  clearAdminEnrollCookie,
  setAdminAuthCookies,
  setAdminEnrollCookie,
} from "@/lib/admin-auth-cookies";

export const runtime = "nodejs";

type AdminLoginPayload = {
  data?: {
    tokens?: {
      access_token?: string;
      refresh_token?: string;
    };
    totp_setup_required?: boolean;
    enrollment_token?: string;
    expires_in?: number;
  };
  error?: { code?: string };
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

    // ADMIN_TOTP_ENFORCE refused a session and handed back a scoped
    // enrollment token instead. It is a credential, so it goes into an
    // httpOnly cookie and is stripped from the body: the page only needs to
    // know it must show the enrollment step, and anything reachable from JS
    // is reachable from an XSS payload.
    const setup = data.data;
    if (
      backendRes.status === 403 &&
      data.error?.code === "totp_setup_required" &&
      typeof setup?.enrollment_token === "string" &&
      setup.enrollment_token
    ) {
      const { enrollment_token: enrollToken, ...safeData } = setup;
      const res = NextResponse.json({ ...data, data: safeData }, { status: 403 });
      clearAdminAuthCookies(res);
      setAdminEnrollCookie(res, enrollToken, setup.expires_in);
      return res;
    }

    const res = NextResponse.json(data, { status: backendRes.status });
    // Any other outcome ends whatever enrollment attempt was in flight.
    clearAdminEnrollCookie(res);
    const tokens = setup?.tokens;
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
