import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { clearAuthCookies, cookieModeFor, readCookieValues, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { rejectCrossSite } from "@/lib/same-origin";

// Cookies are cleared unconditionally — logout must never leave the client
// "logged in" locally just because the backend call failed or the refresh
// token was already gone (mirrors the Flutter-era logout() precedent: it
// clears tokens on both a thrown exception and a Result.err).
export async function POST(request: Request) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;

  // Usually one value; a Telegram WebView that also holds the website's lax
  // pair sends two "rt" cookies, and both sessions must end. Capped so a
  // crafted Cookie header cannot fan out into many backend calls.
  const refreshTokens = [...new Set(readCookieValues(request, REFRESH_COOKIE).filter(Boolean))].slice(0, 2);
  await Promise.all(
    refreshTokens.map(async (refreshToken) => {
      try {
        await backendFetch("/auth/logout", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh_token: refreshToken }),
        });
      } catch {
        // Ignored deliberately — cookies are cleared below regardless.
      }
    })
  );

  const response = NextResponse.json({ data: { ok: true } }, { status: 200 });
  clearAuthCookies(response, cookieModeFor(request), { endSitePair: true });
  return response;
}
