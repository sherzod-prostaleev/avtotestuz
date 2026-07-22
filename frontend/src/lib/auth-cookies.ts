import type { NextResponse } from "next/server";

export const AUTH_COOKIE = "at";
export const REFRESH_COOKIE = "rt";

const AT_MAX_AGE = 900; // 15 minutes, matches backend access-token TTL
const RT_MAX_AGE = 60 * 60 * 24 * 30; // 30 days, matches backend rotating refresh TTL

const baseOptions = {
  httpOnly: true,
  sameSite: "lax" as const,
  secure: process.env.NODE_ENV === "production",
  path: "/",
};

export function setAuthCookies(
  res: NextResponse,
  tokens: { accessToken: string; refreshToken: string }
): void {
  res.cookies.set(AUTH_COOKIE, tokens.accessToken, { ...baseOptions, maxAge: AT_MAX_AGE });
  res.cookies.set(REFRESH_COOKIE, tokens.refreshToken, { ...baseOptions, maxAge: RT_MAX_AGE });
}

export function clearAuthCookies(res: NextResponse): void {
  res.cookies.set(AUTH_COOKIE, "", { ...baseOptions, maxAge: 0 });
  res.cookies.set(REFRESH_COOKIE, "", { ...baseOptions, maxAge: 0 });
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
