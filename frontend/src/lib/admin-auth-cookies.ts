import type { NextResponse } from "next/server";

/** Separate from learner `at`/`rt` cookies (blast-radius isolation). */
export const ADMIN_AUTH_COOKIE = "aat";
export const ADMIN_REFRESH_COOKIE = "art";

/**
 * Holds the scoped TOTP-enrollment token that `POST /admin/v1/auth/login`
 * hands back (403 `totp_setup_required`) to an admin that ADMIN_TOTP_ENFORCE
 * refuses a session to. It is a credential, so it lives in an httpOnly cookie
 * exactly like `aat`/`art` — the browser never reads it — and it is deliberately
 * NOT named `aat`/`art`: `proxy.ts` treats those two as "has an admin session"
 * and would let an un-enrolled admin through to the admin shell.
 */
export const ADMIN_TOTP_ENROLL_COOKIE = "ate";

const AT_MAX_AGE = 900;
const RT_MAX_AGE = 60 * 60 * 24 * 30;

/** Matches backend `totpEnrollTTL` (15 min); the login response restates it. */
export const ADMIN_ENROLL_MAX_AGE = 900;

const baseOptions = {
  httpOnly: true,
  sameSite: "lax" as const,
  secure: process.env.NODE_ENV === "production",
  path: "/",
};

/**
 * Path-scoping the enrollment cookie to the TOTP enrollment endpoints means the
 * browser never even transmits it anywhere else, so no other BFF route can send
 * it upstream by accident. `adminProxy` enforces the same allowlist server-side.
 */
const enrollOptions = { ...baseOptions, path: "/api/admin/security/totp" };

export function setAdminAuthCookies(
  res: NextResponse,
  tokens: { accessToken: string; refreshToken: string },
): void {
  res.cookies.set(ADMIN_AUTH_COOKIE, tokens.accessToken, { ...baseOptions, maxAge: AT_MAX_AGE });
  res.cookies.set(ADMIN_REFRESH_COOKIE, tokens.refreshToken, { ...baseOptions, maxAge: RT_MAX_AGE });
}

export function clearAdminAuthCookies(res: NextResponse): void {
  res.cookies.set(ADMIN_AUTH_COOKIE, "", { ...baseOptions, maxAge: 0 });
  res.cookies.set(ADMIN_REFRESH_COOKIE, "", { ...baseOptions, maxAge: 0 });
}

/**
 * Stores the enrollment token for at most as long as the backend will honour
 * it, so a spent flow cannot leave a live credential sitting in the browser.
 */
export function setAdminEnrollCookie(res: NextResponse, token: string, expiresIn?: number): void {
  const maxAge =
    typeof expiresIn === "number" && Number.isFinite(expiresIn) && expiresIn > 0
      ? Math.min(Math.floor(expiresIn), ADMIN_ENROLL_MAX_AGE)
      : ADMIN_ENROLL_MAX_AGE;
  res.cookies.set(ADMIN_TOTP_ENROLL_COOKIE, token, { ...enrollOptions, maxAge });
}

export function clearAdminEnrollCookie(res: NextResponse): void {
  res.cookies.set(ADMIN_TOTP_ENROLL_COOKIE, "", { ...enrollOptions, maxAge: 0 });
}
