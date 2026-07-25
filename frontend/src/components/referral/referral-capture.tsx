"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { applyPendingReferralCode, capturePendingReferralCodeFromUrl } from "@/lib/referral-storage";

/**
 * Redeems a captured referral code for a user who is already signed in.
 *
 * Two cases the login/verify path cannot cover on its own:
 *
 *  1. An existing user opens an invite link. The middleware bounces a request
 *     for /login straight to /dashboard, so verify never runs — it forwards
 *     `?ref=` along on that redirect precisely so this component can pick it up.
 *  2. A code left in storage because an earlier apply attempt failed for a
 *     transient reason (offline, 5xx, session not established yet).
 *
 * Mounted once in the authenticated layout and renders nothing. The apply call
 * is a no-op unless a code is actually stored, so the common case costs one
 * localStorage read per navigation.
 */
export function ReferralCapture() {
  const pathname = usePathname();

  useEffect(() => {
    capturePendingReferralCodeFromUrl();
    void applyPendingReferralCode();
  }, [pathname]);

  return null;
}
