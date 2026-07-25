"use client";

import { ApiError, apiPost } from "@/lib/api-client";

/**
 * Pending referral code captured from a `?ref=CODE` invite link, held until we
 * can attach it to an account.
 *
 * The code arrives on a URL (/r/{code} -> /login?ref=CODE) but can only be
 * redeemed by an authenticated caller, so there is always a gap to bridge:
 * a brand-new user has to complete an OTP round trip first, and an
 * already-signed-in user gets bounced straight off /login by the middleware.
 * localStorage bridges that gap; these helpers are the only place that touches
 * the key so the capture and apply sides cannot drift apart.
 */
export const REFERRAL_CODE_STORAGE_KEY = "avtotest_pending_referral_code";

/**
 * Server error codes for which a retry can never succeed, so the stored code
 * should be dropped. Everything else (network failure, 5xx, proxy hiccup, a
 * session that is not established yet) leaves the code in place for the next
 * attempt — the earlier version of this flow deleted the code *before* issuing
 * the request, so a single transient failure silently destroyed the referral
 * and the inviter's reward with no way to recover it.
 */
const TERMINAL_ERROR_CODES = new Set([
  "referral_not_found",
  "referral_self",
  "referral_already_applied",
]);

export function storePendingReferralCode(code: string): void {
  const clean = code.trim().toUpperCase();
  if (!clean) return;
  try {
    window.localStorage.setItem(REFERRAL_CODE_STORAGE_KEY, clean);
  } catch {
    // Private browsing / storage disabled: the invite is simply lost, which is
    // no worse than not capturing it at all.
  }
}

export function readPendingReferralCode(): string | null {
  try {
    return window.localStorage.getItem(REFERRAL_CODE_STORAGE_KEY);
  } catch {
    return null;
  }
}

export function clearPendingReferralCode(): void {
  try {
    window.localStorage.removeItem(REFERRAL_CODE_STORAGE_KEY);
  } catch {
    // ignore
  }
}

/**
 * Captures `?ref=CODE` from the current URL, if present.
 *
 * Reads window.location directly rather than useSearchParams so callers do not
 * need to sit inside a Suspense boundary.
 */
export function capturePendingReferralCodeFromUrl(): void {
  if (typeof window === "undefined") return;
  const ref = new URLSearchParams(window.location.search).get("ref");
  if (ref) storePendingReferralCode(ref);
}

/**
 * Best-effort redemption of a captured code. Safe to call on every
 * authenticated page load: the backend enforces one referral per referee, and
 * a code that is genuinely unusable is discarded on the first definitive
 * answer instead of being retried forever.
 *
 * Deliberately silent — this is a background bonus, not an action the user
 * initiated, and the inviter's reward is confirmed on the referral screen.
 */
export async function applyPendingReferralCode(): Promise<void> {
  if (typeof window === "undefined") return;
  const code = readPendingReferralCode();
  if (!code) return;
  try {
    await apiPost("referral/apply", { code });
    clearPendingReferralCode();
  } catch (err) {
    if (err instanceof ApiError && TERMINAL_ERROR_CODES.has(err.code)) {
      clearPendingReferralCode();
    }
  }
}
