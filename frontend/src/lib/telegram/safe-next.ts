/**
 * Where /tg may send the learner after sign-in. Only same-locale app paths:
 * anything else (other origins, protocol-relative, backslash tricks, /tg
 * itself) falls back to the dashboard.
 */
export function safeNextPath(raw: string | null, locale: string): string {
  const fallback = `/${locale}/dashboard`;
  if (!raw) return fallback;
  if (!raw.startsWith(`/${locale}/`) || raw.includes("//") || raw.includes("\\")) return fallback;
  if (raw === `/${locale}/tg` || raw.startsWith(`/${locale}/tg?`) || raw.startsWith(`/${locale}/tg/`)) return fallback;
  return raw;
}
