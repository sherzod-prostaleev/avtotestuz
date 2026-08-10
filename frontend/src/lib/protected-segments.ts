// Split out from src/proxy.ts so it can be imported by component tests
// without pulling in next-intl/middleware and next/server (which don't
// resolve under Vitest's jsdom environment). The kiosk-path tests import
// this module directly to assert every route the classroom kiosk can reach
// falls outside PROTECTED_SEGMENTS — a cookie-less kiosk browser bounces to
// /login the moment it matches one of these.
export const PROTECTED_SEGMENTS = [
  "dashboard",
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
