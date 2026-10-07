import type { NextResponse } from "next/server";

export const AUTH_COOKIE = "at";
export const REFRESH_COOKIE = "rt";
/**
 * Marks a cookie jar that belongs to the Telegram Mini App. Its only job is to
 * tell refresh/logout which attributes to re-issue: a partitioned cookie can
 * only be replaced or deleted by a Set-Cookie that is itself Partitioned.
 * The website never receives it, so the site's lax cookies never change.
 */
export const TG_MODE_COOKIE = "tgp";

export type CookieMode = "site" | "telegram";

const AT_MAX_AGE = 900; // 15 minutes, matches backend access-token TTL
const RT_MAX_AGE = 60 * 60 * 24 * 30; // 30 days, matches backend rotating refresh TTL

const siteOptions = {
  httpOnly: true,
  sameSite: "lax" as const,
  secure: process.env.NODE_ENV === "production",
  path: "/",
};

// web.telegram.org runs the Mini App in a cross-site iframe, where lax cookies
// are never sent. Partitioned (CHIPS) keys them to the telegram.org top level,
// so they cannot be replayed from any other site; the Origin guard in
// same-origin.ts covers other Mini Apps inside the same top level. Secure is
// unconditional because browsers reject SameSite=None without it — so a Mini
// App under development must be served over https (a tunnel) or localhost;
// on a plain-http LAN IP the browser silently drops these cookies.
const telegramOptions = {
  httpOnly: true,
  sameSite: "none" as const,
  secure: true,
  partitioned: true,
  path: "/",
};

function optionsFor(mode: CookieMode) {
  return mode === "telegram" ? telegramOptions : siteOptions;
}

export function cookieModeFor(request: Request): CookieMode {
  return readCookie(request, TG_MODE_COOKIE) === "1" ? "telegram" : "site";
}

/**
 * Login and register switch to Telegram mode when the Mini App sends its
 * launch data along (the backend then links the account). Any other body —
 * including one that is not JSON at all — keeps the website's cookies.
 */
export function modeForBody(body: string): CookieMode {
  let parsed: unknown;
  try {
    parsed = JSON.parse(body);
  } catch {
    return "site";
  }
  if (!parsed || typeof parsed !== "object") return "site";
  const initData = (parsed as { tg_init_data?: unknown }).tg_init_data;
  return typeof initData === "string" && initData !== "" ? "telegram" : "site";
}

/**
 * In Telegram's Android WebView drivergo.uz is the top level, so the jar may
 * already hold the website's unpartitioned lax at/rt (an in-app browser login,
 * an OTP sign-in). CHIPS keeps those beside the new partitioned pair and the
 * browser sends both under the same names; the BFF would then keep reading and
 * rotating the stale lax rt while it lingers revoked-but-present, and the next
 * replay trips refresh-token reuse detection, revoking the whole session.
 * Telegram-mode issuance therefore expires the lax pair in the same response.
 *
 * ResponseCookies keys by name, so it cannot hold a lax and a partitioned "at"
 * at once; the expiries are raw headers. Its set() rewrites every Set-Cookie
 * header from its own map, which drops them — hence this runs after the last
 * set() and re-applies after every later one (each telegram setAuthCookies
 * call does). The expiries go first: WebKit ignores Partitioned and treats
 * both as one cookie, so an expiry after the set would delete the new pair.
 * In the web.telegram.org iframe the lax expiry is refused as a cross-site
 * write, which is harmless: lax cookies are never sent there either.
 */
function expireSiteSessionFirst(res: NextResponse): void {
  const attrs = `Path=/; Max-Age=0; HttpOnly; SameSite=Lax${siteOptions.secure ? "; Secure" : ""}`;
  const issued = res.headers.getSetCookie();
  res.headers.delete("set-cookie");
  for (const name of [AUTH_COOKIE, REFRESH_COOKIE]) {
    res.headers.append("set-cookie", `${name}=; ${attrs}`);
  }
  for (const cookie of issued) res.headers.append("set-cookie", cookie);
}

export function setAuthCookies(
  res: NextResponse,
  tokens: { accessToken: string; refreshToken: string },
  mode: CookieMode = "site"
): void {
  const options = optionsFor(mode);
  res.cookies.set(AUTH_COOKIE, tokens.accessToken, { ...options, maxAge: AT_MAX_AGE });
  res.cookies.set(REFRESH_COOKIE, tokens.refreshToken, { ...options, maxAge: RT_MAX_AGE });
  if (mode === "telegram") {
    res.cookies.set(TG_MODE_COOKIE, "1", { ...options, maxAge: RT_MAX_AGE });
    expireSiteSessionFirst(res);
  }
}

export function clearAuthCookies(res: NextResponse, mode: CookieMode = "site"): void {
  const options = optionsFor(mode);
  res.cookies.set(AUTH_COOKIE, "", { ...options, maxAge: 0 });
  res.cookies.set(REFRESH_COOKIE, "", { ...options, maxAge: 0 });
  if (mode === "telegram") {
    res.cookies.set(TG_MODE_COOKIE, "", { ...options, maxAge: 0 });
  }
}

// Reads a cookie directly from the request's Cookie header rather than via
// next/headers' cookies() — that API depends on Next's request-scoped
// AsyncLocalStorage context, which doesn't exist when a Route Handler is
// unit-tested by importing and calling it directly. Reading the raw header
// works identically in production (Route Handlers always receive the real
// Cookie header) and needs no Next-runtime context to test.
export function readCookie(request: Request, name: string): string | undefined {
  const header = request.headers.get("cookie");
  if (!header) return undefined;
  const match = header
    .split(";")
    .map((c) => c.trim())
    .find((c) => c.startsWith(`${name}=`));
  if (!match) return undefined;

  try {
    return decodeURIComponent(match.slice(name.length + 1));
  } catch {
    // A malformed attacker-controlled Cookie header must behave like a
    // missing cookie instead of crashing every BFF request with a 500.
    return undefined;
  }
}
