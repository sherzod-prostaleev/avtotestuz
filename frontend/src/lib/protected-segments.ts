// Split out from src/proxy.ts so it can be imported by component tests
// without pulling in next-intl/middleware and next/server (which don't
// resolve under Vitest's jsdom environment). The kiosk-path tests import
// this module directly to assert every route the classroom kiosk can reach
// falls outside PROTECTED_SEGMENTS — a cookie-less kiosk browser bounces to
// /login the moment it matches one of these.
export const PROTECTED_SEGMENTS = [
  "dashboard",
  // The exam chooser, not the kiosk's /station/exam — matchesAny only covers
  // "/exam" and "/exam/...", so the station route stays login-free.
  "exam",
  "exam-mockup",
  "tickets",
  "practice",
  "mistakes",
  "notifications",
  "signs",
  "leaderboard",
  "arena",
  "stats",
  "support",
  "profile",
  "premium",
  "saved",
  "session",
  "checkout",
  "change-password",
];

export function matchesAny(pathname: string, segments: string[]): boolean {
  return segments.some((seg) => pathname === `/${seg}` || pathname.startsWith(`/${seg}/`));
}

// /checkout/done sits under a protected segment but must open without a
// session: Payme/Click return a Mini App payer there in an external browser
// that never had our cookies. The bot username rides as ONE path segment
// (/checkout/done/<bot>) because Payme embeds the return URL raw in its
// ';'-joined key=value payload, so no '?' or '=' can be used. Anything deeper
// or any sibling (/checkout/pending, /checkout/done/a/b) stays protected.
const PUBLIC_CHECKOUT_DONE = /^\/checkout\/done(?:\/[^/]+)?$/;

/** The proxy's login gate: a protected segment that is not a public exception. */
export function isProtectedPath(pathname: string): boolean {
  return matchesAny(pathname, PROTECTED_SEGMENTS) && !PUBLIC_CHECKOUT_DONE.test(pathname);
}
