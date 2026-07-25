import { NextResponse } from "next/server";
import { backendAdminFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import {
  ADMIN_AUTH_COOKIE,
  ADMIN_REFRESH_COOKIE,
  setAdminAuthCookies,
  clearAdminAuthCookies,
} from "@/lib/admin-auth-cookies";
import { readCookie } from "@/lib/auth-cookies";

export const runtime = "nodejs";

type AdminRefreshPayload = {
  data?: {
    access_token?: string;
  };
};

function unavailable() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 },
  );
}

async function refreshAdmin(request: Request): Promise<string | null> {
  const refresh = readCookie(request, ADMIN_REFRESH_COOKIE);
  if (!refresh) return null;
  const backendRes = await backendAdminFetch("/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refresh }),
  });
  if (!backendRes.ok) return null;
  const data = await readBackendJson<AdminRefreshPayload>(backendRes);
  const access = data.data?.access_token;
  return typeof access === "string" ? access : null;
}

export async function GET(request: Request) {
  let access = readCookie(request, ADMIN_AUTH_COOKIE) ?? "";
  try {
    let backendRes = await backendAdminFetch("/me", {
      method: "GET",
      headers: {
        Authorization: access ? `Bearer ${access}` : "",
        "Content-Type": "application/json",
      },
    });
    if (backendRes.status === 401) {
      const rotated = await refreshAdmin(request);
      if (!rotated) {
        const res = NextResponse.json(
          { error: { code: "unauthorized", message: "admin session expired" } },
          { status: 401 },
        );
        clearAdminAuthCookies(res);
        return res;
      }
      access = rotated;
      backendRes = await backendAdminFetch("/me", {
        method: "GET",
        headers: {
          Authorization: `Bearer ${access}`,
          "Content-Type": "application/json",
        },
      });
      const data = await readBackendJson(backendRes);
      const res = NextResponse.json(data, { status: backendRes.status });
      const refresh = readCookie(request, ADMIN_REFRESH_COOKIE);
      if (backendRes.ok && refresh) {
        setAdminAuthCookies(res, { accessToken: access, refreshToken: refresh });
      }
      return res;
    }
    const data = await readBackendJson(backendRes);
    return NextResponse.json(data, { status: backendRes.status });
  } catch {
    return unavailable();
  }
}
