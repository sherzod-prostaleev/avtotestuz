/**
 * Optional Sentry browser init (U-41).
 * Empty NEXT_PUBLIC_SENTRY_DSN → no-op (no SDK network).
 * Honest: error capture only — no pager / on-call product.
 */

let started = false;

export function sentryDsn(): string {
  return (process.env.NEXT_PUBLIC_SENTRY_DSN ?? "").trim();
}

export function isSentryEnabled(): boolean {
  return sentryDsn().length > 0;
}

/** Call once from a client Providers tree. Safe to call repeatedly. */
export async function initSentry(): Promise<boolean> {
  if (started) return isSentryEnabled();
  const dsn = sentryDsn();
  if (!dsn) {
    started = true;
    return false;
  }
  const Sentry = await import("@sentry/browser");
  Sentry.init({
    dsn,
    tracesSampleRate: 0,
  });
  started = true;
  return true;
}

/** Test helper — resets the once-guard between vitest cases. */
export function resetSentryForTests(): void {
  started = false;
}
