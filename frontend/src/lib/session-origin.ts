/**
 * Where the learner was standing when they opened a session.
 *
 * Exit used to be hardcoded to the dashboard (or the station home on a kiosk),
 * so picking bilet 12 out of /tickets and leaving it dropped you on the home
 * screen instead of back in the ticket list you were working through. Every
 * hub links into /session/start — tickets, practice, signs, stats, mistakes,
 * the dashboard cards, the exam picker, and their /station twins — so rather
 * than threading a `from=` param through each of those call sites (and
 * silently missing the next one someone adds), the shells record the last hub
 * the learner actually stood on and the session screens read it back.
 *
 * Stored per tab in sessionStorage: a second tab drilling into a different
 * hub must not rewrite this one's way back, and the value is meaningless
 * once the tab is gone.
 */
export const SESSION_ORIGIN_KEY = "avtotest:session-origin";

/**
 * Session-owned screens never count as an origin: recording them would make
 * "exit" point back into the very screen the learner is trying to leave.
 * Covers the learner routes and their /station twins in one test.
 */
export function isSessionOwnedPath(pathname: string): boolean {
  return pathname.includes("/session/") || pathname.includes("/memorize/");
}

/**
 * Rejects anything that is not a plain in-app absolute path. The values are
 * written by the tracker from usePathname(), never by a user, but a stored
 * value survives in the tab across code changes — so it is validated on the
 * way out rather than trusted, and a protocol-relative "//evil.example" can
 * never become a redirect target.
 */
function isSafeInternalPath(value: string): boolean {
  return value.startsWith("/") && !value.startsWith("//") && !value.includes("\\");
}

export function rememberSessionOrigin(pathname: string): void {
  if (!isSafeInternalPath(pathname) || isSessionOwnedPath(pathname)) return;
  try {
    window.sessionStorage.setItem(SESSION_ORIGIN_KEY, pathname);
  } catch {
    // Private mode or a browser with site data blocked: exit falls back to the
    // dashboard, which is exactly the behaviour that shipped before this.
  }
}

/** The remembered hub, or null when there is nothing usable to go back to. */
export function readSessionOrigin(): string | null {
  try {
    const stored = window.sessionStorage.getItem(SESSION_ORIGIN_KEY);
    if (!stored || !isSafeInternalPath(stored) || isSessionOwnedPath(stored)) return null;
    return stored;
  } catch {
    return null;
  }
}
