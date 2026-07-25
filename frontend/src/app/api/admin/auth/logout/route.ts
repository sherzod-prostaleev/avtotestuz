import { NextResponse } from "next/server";
import { backendAdminFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import { clearAdminAuthCookies, ADMIN_REFRESH_COOKIE } from "@/lib/admin-auth-cookies";
import { readCookie } from "@/lib/auth-cookies";

export const runtime = "nodejs";

export async function POST(request: Request) {
  const refresh = readCookie(request, ADMIN_REFRESH_COOKIE) ?? "";
  try {
    await backendAdminFetch("/auth/logout", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refresh }),
    });
  } catch {
    /* still clear cookies */
  }
  const res = NextResponse.json({ data: { status: "ok" } });
  clearAdminAuthCookies(res);
  return res;
}
