import type { NextResponse } from "next/server";
import { readCookie } from "@/lib/auth-cookies";

/**
 * The browser half of a website Telegram login request: a random secret the
 * backend stores only as a digest. Status and complete need it next to the
 * token, so a login link forwarded to (or opened by) someone else can never
 * sign anyone in except the browser that started it. HttpOnly — page script
 * never sees it — and scoped to the three BFF routes that use it.
 */
export const TELEGRAM_LOGIN_COOKIE = "tgl";
/** The status poll carries the token in this header (never the URL). */
export const TELEGRAM_LOGIN_TOKEN_HEADER = "x-telegram-login-token";
const TELEGRAM_LOGIN_PATH = "/api/auth/telegram-login";
// The request lives 5 minutes; an approval near the end may be completed for
// two more (backend telegramLoginCompleteGrace).
const TELEGRAM_LOGIN_MAX_AGE = 7 * 60;

const options = {
  httpOnly: true,
  sameSite: "lax" as const,
  secure: process.env.NODE_ENV === "production",
  path: TELEGRAM_LOGIN_PATH,
};

export function setTelegramLoginCookie(res: NextResponse, secret: string): void {
  res.cookies.set(TELEGRAM_LOGIN_COOKIE, secret, { ...options, maxAge: TELEGRAM_LOGIN_MAX_AGE });
}

export function clearTelegramLoginCookie(res: NextResponse): void {
  res.cookies.set(TELEGRAM_LOGIN_COOKIE, "", { ...options, maxAge: 0 });
}

export function readTelegramLoginSecret(request: Request): string {
  return readCookie(request, TELEGRAM_LOGIN_COOKIE) ?? "";
}

/** The deep-link token is opaque base64url; anything else is not ours. */
export function isLoginToken(value: unknown): value is string {
  return typeof value === "string" && /^[A-Za-z0-9_-]{20,64}$/.test(value);
}
