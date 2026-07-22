import { NextResponse } from "next/server";
import { setAuthCookies, clearAuthCookies, readCookie, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { refreshOnce } from "@/lib/refresh-lock";
import { callBackendRefresh } from "@/lib/backend-refresh";

export async function POST(request: Request) {
  const refreshToken = readCookie(request, REFRESH_COOKIE);
  if (!refreshToken) {
    const response = NextResponse.json(
      { error: { code: "invalid_refresh", message: "no refresh token" } },
      { status: 401 }
    );
    clearAuthCookies(response);
    return response;
  }

  const tokens = await refreshOnce(refreshToken, callBackendRefresh);
  if (!tokens) {
    const response = NextResponse.json(
      { error: { code: "invalid_refresh", message: "refresh failed" } },
      { status: 401 }
    );
    clearAuthCookies(response);
    return response;
  }

  const response = NextResponse.json({ data: { ok: true } }, { status: 200 });
  setAuthCookies(response, tokens);
  return response;
}
