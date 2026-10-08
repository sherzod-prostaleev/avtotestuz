/**
 * Where /tg may send the learner after sign-in. Only same-locale app paths:
 * anything else (other origins, protocol-relative, backslash tricks, /tg
 * itself) falls back to the dashboard.
 */
export function safeNextPath(raw: string | null, locale: string): string {
  const fallback = `/${locale}/dashboard`;
  // new URL() would happily resolve "http://x/l/foo" or "l/foo" against the
  // dummy base; only a leading slash is an app path.
  if (!raw || !raw.startsWith("/") || raw.includes("\\")) return fallback;
  // Normalise first: "/l/x/../tg" and "/l/%2e%2e/tg" would pass a prefix check
  // yet resolve to /l/tg, looping the learner back into this page.
  let url: URL;
  try {
    url = new URL(raw, "http://x");
  } catch {
    return fallback;
  }
  if (url.origin !== "http://x") return fallback;
  const path = url.pathname;
  if (!path.startsWith(`/${locale}/`) || raw.includes("//") || path.includes("//")) return fallback;
  const tg = `/${locale}/tg`;
  if (path === tg || path.startsWith(`${tg}/`)) return fallback;
  return path + url.search;
}

/**
 * The deep link's raw `next`, or null where the post-sign-in redirect does not
 * honour it. Only a live Mini App (a WebApp object) does: on the website, and
 * while the SDK is loading or has failed, sign-in lands on the dashboard.
 */
export function miniAppNext(webApp: object | null, raw: string | null): string | null {
  return webApp ? raw : null;
}

/**
 * The `?next=…` the login ↔ register cross links carry: exactly when
 * {@link miniAppNext} would let the redirect use it, and never for a target
 * that already falls back to the dashboard.
 */
export function carryNextQuery(webApp: object | null, raw: string | null, locale: string): string {
  const next = miniAppNext(webApp, raw);
  if (next === null || safeNextPath(next, locale) === `/${locale}/dashboard`) return "";
  return `?next=${encodeURIComponent(next)}`;
}
