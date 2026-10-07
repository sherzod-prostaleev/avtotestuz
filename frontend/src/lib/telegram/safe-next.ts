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
