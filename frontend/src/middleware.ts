import createMiddleware from "next-intl/middleware";
import { NextRequest, NextResponse } from "next/server";
import { locales, type Locale } from "@/i18n/config";
import { routing } from "@/i18n/routing";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { ADMIN_AUTH_COOKIE, ADMIN_REFRESH_COOKIE } from "@/lib/admin-auth-cookies";

const intlMiddleware = createMiddleware(routing);

const PROTECTED_SEGMENTS = [
  "dashboard",
  "exam-mockup",
  "tickets",
  "practice",
  "mistakes",
  "signs",
  "leaderboard",
  "arena",
  "stats",
  "profile",
  "premium",
  "saved",
  "session",
  "checkout",
  "teacher",
];
const AUTH_SEGMENTS = ["login"];

function matchesAny(pathname: string, segments: string[]): boolean {
  return segments.some((seg) => pathname === `/${seg}` || pathname.startsWith(`/${seg}/`));
}

export default function middleware(request: NextRequest) {
  const segments = request.nextUrl.pathname.split("/").filter(Boolean);
  const hasLocalePrefix = locales.includes(segments[0] as Locale);

  if (!hasLocalePrefix) {
    return intlMiddleware(request);
  }

  const locale = segments[0];
  const pathname = "/" + segments.slice(1).join("/");
  const hasSession = Boolean(request.cookies.get(AUTH_COOKIE) ?? request.cookies.get(REFRESH_COOKIE));
  const hasAdminSession = Boolean(
    request.cookies.get(ADMIN_AUTH_COOKIE) ?? request.cookies.get(ADMIN_REFRESH_COOKIE),
  );

  // Learner "settings" is the profile page (nav label: Profil va sozlamalar).
  if (pathname === "/settings" || pathname.startsWith("/settings/")) {
    return NextResponse.redirect(new URL(`/${locale}/profile`, request.url));
  }

  // Admin shell — separate from learner cookies.
  if (pathname === "/admin/login") {
    if (hasAdminSession) {
      return NextResponse.redirect(new URL(`/${locale}/admin`, request.url));
    }
    return intlMiddleware(request);
  }
  if (pathname === "/admin" || pathname.startsWith("/admin/")) {
    if (!hasAdminSession) {
      return NextResponse.redirect(new URL(`/${locale}/admin/login`, request.url));
    }
    return intlMiddleware(request);
  }

  if (matchesAny(pathname, PROTECTED_SEGMENTS) && !hasSession) {
    return NextResponse.redirect(new URL(`/${locale}/login`, request.url));
  }
  if (matchesAny(pathname, AUTH_SEGMENTS) && hasSession) {
    // Carry `?ref=` across: an invite link (/r/{code} -> /login?ref=CODE)
    // opened by someone who is already signed in gets bounced here, and
    // dropping the query would silently discard the referral. ReferralCapture
    // in the authenticated layout redeems it. Only this one param is forwarded
    // so the redirect target stays predictable.
    const target = new URL(`/${locale}/dashboard`, request.url);
    const ref = request.nextUrl.searchParams.get("ref");
    if (ref) target.searchParams.set("ref", ref);
    return NextResponse.redirect(target);
  }

  return intlMiddleware(request);
}

export const config = {
  matcher: ["/((?!api|_next|.*\\..*).*)"],
};
