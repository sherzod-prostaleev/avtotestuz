import { NextResponse } from "next/server";

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

function forbidden() {
  return NextResponse.json(
    { error: { code: "cross_site", message: "cross-site request refused" } },
    { status: 403 }
  );
}

/**
 * CSRF guard for cookie-authenticated BFF writes. The site's lax cookies were
 * the only CSRF defence; Telegram-mode cookies are SameSite=None, so another
 * Mini App framed inside web.telegram.org could otherwise POST with them.
 * nginx forwards the public Host ($host), which is what a same-origin browser
 * request's Origin carries; the dev/e2e server sees "localhost:PORT" in both.
 */
export function rejectCrossSite(request: Request): NextResponse | null {
  if (SAFE_METHODS.has(request.method.toUpperCase())) return null;
  const origin = request.headers.get("origin");
  if (origin !== null) {
    let originHost: string;
    try {
      originHost = new URL(origin).host;
    } catch {
      return forbidden(); // "null" from sandboxed frames, or garbage
    }
    // URL already lowercases the origin's host; nginx's $host is lowercase
    // too, but the dev server passes the browser's Host through verbatim.
    const host = request.headers.get("host")?.toLowerCase();
    return host && originHost === host ? null : forbidden();
  }
  // Browsers always send Origin on cross-site POSTs; a missing Origin plus an
  // explicit cross-site fetch-metadata header is still refused. Non-browser
  // callers cannot attach a victim's cookies, so they pass.
  return request.headers.get("sec-fetch-site") === "cross-site" ? forbidden() : null;
}
