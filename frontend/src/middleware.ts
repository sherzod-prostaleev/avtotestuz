import createMiddleware from "next-intl/middleware";
import { NextRequest, NextResponse } from "next/server";
import { locales, defaultLocale, type Locale } from "@/i18n/config";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

const intlMiddleware = createMiddleware({ locales, defaultLocale, localePrefix: "always" });

const PROTECTED_SEGMENTS = [
  "dashboard",
  "exam-mockup",
  "tickets",
  "practice",
  "mistakes",
  "signs",
  "stats",
  "profile",
  "premium",
  "saved",
  "session",
  "checkout",
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

  if (matchesAny(pathname, PROTECTED_SEGMENTS) && !hasSession) {
    return NextResponse.redirect(new URL(`/${locale}/login`, request.url));
  }
  if (matchesAny(pathname, AUTH_SEGMENTS) && hasSession) {
    return NextResponse.redirect(new URL(`/${locale}/dashboard`, request.url));
  }

  return intlMiddleware(request);
}

export const config = {
  matcher: ["/((?!api|_next|.*\\..*).*)"],
};
