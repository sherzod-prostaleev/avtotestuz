/**
 * Shared localStorage key for a pending referral code captured from a
 * `?ref=CODE` invite link on the login page (see (auth)/login/page.tsx) and
 * later applied after a successful signup/login (see
 * (auth)/login/verify/page.tsx).
 */
export const REFERRAL_CODE_STORAGE_KEY = "avtotest_pending_referral_code";
