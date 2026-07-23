import { NextResponse } from "next/server";
import { setAuthCookies, clearAuthCookies, readCookie, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { refreshOnce } from "@/lib/refresh-lock";
import { callBackendRefresh } from "@/lib/backend-refresh";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
}

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

  let tokens: { accessToken: string; refreshToken: string } | null;
  try {
    tokens = await refreshOnce(refreshToken, callBackendRefresh);
  } catch {
    // A transient upstream failure must not log the user out or erase the
    // still-valid refresh cookie. The client can retry once service returns.
    return unavailableResponse();
  }
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
