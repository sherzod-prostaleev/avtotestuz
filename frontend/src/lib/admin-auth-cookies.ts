import type { NextResponse } from "next/server";

/** Separate from learner `at`/`rt` cookies (blast-radius isolation). */
export const ADMIN_AUTH_COOKIE = "aat";
export const ADMIN_REFRESH_COOKIE = "art";

const AT_MAX_AGE = 900;
const RT_MAX_AGE = 60 * 60 * 24 * 30;

const baseOptions = {
  httpOnly: true,
  sameSite: "lax" as const,
  secure: process.env.NODE_ENV === "production",
  path: "/",
};

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
