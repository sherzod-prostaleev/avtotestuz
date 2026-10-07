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
// unconditional because browsers reject SameSite=None without it.
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
