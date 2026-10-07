import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { clearAuthCookies, cookieModeFor, readCookie, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { rejectCrossSite } from "@/lib/same-origin";

// Cookies are cleared unconditionally — logout must never leave the client
// "logged in" locally just because the backend call failed or the refresh
// token was already gone (mirrors the Flutter-era logout() precedent: it
// clears tokens on both a thrown exception and a Result.err).
export async function POST(request: Request) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;

  const refreshToken = readCookie(request, REFRESH_COOKIE);
  if (refreshToken) {
    try {
      await backendFetch("/auth/logout", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });
    } catch {
      // Ignored deliberately — cookies are cleared below regardless.
    }
  }

  const response = NextResponse.json({ data: { ok: true } }, { status: 200 });
  clearAuthCookies(response, cookieModeFor(request));
  return response;
}
