"use client";

import { useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { useLocale } from "next-intl";
import { useQueryClient } from "@tanstack/react-query";
import { onSessionExpired } from "@/lib/session-expiry";
import { clearSessionHistoryCache } from "@/hooks/use-session-history";
import { mistakesCountStore, mockEligibilityStore, savedQuestionsStore } from "@/lib/dashboard-stores";

/**
 * Turns the first unauthorized answer into a trip to the login screen.
 *
 * Mounted on the learner shells only — (app) and (session). The kiosk reaches
 * the same screens through /station/... with a station token and no learner
 * cookie, and the public site fetches grading-neutral content anonymously;
 * neither has a login screen to be sent to, so neither subscribes.
 *
 * Renders nothing and never blocks paint: the shell stays up during the one
 * navigation, exactly like MustChangePasswordGate.
 */
export function SessionExpiredGate() {
  const locale = useLocale();
  const router = useRouter();
  const queryClient = useQueryClient();
  // A dashboard fires half a dozen requests at once and every one of them
  // comes back 401. Only the first may act.
  const handledRef = useRef(false);

  useEffect(() => {
    return onSessionExpired(() => {
      if (handledRef.current) return;
      handledRef.current = true;
      void endSession();
    });

    async function endSession() {
      try {
        // The middleware only checks that the `rt` cookie EXISTS, so a
        // lingering one bounces /login straight back to /dashboard and the
        // learner ping-pongs. /api/auth/logout drops both cookies whatever
        // the backend answers, which makes the redirect below terminal.
        await fetch("/api/auth/logout", { method: "POST" });
      } catch {
        /* best-effort: the redirect matters more than the round trip */
      }

      // Everything below is per-learner and outlives a client-side
      // navigation. On a shared classroom PC the next person to sign in
      // would otherwise inherit this one's streak, mistakes and history.
      queryClient.clear();
      clearSessionHistoryCache();
      mistakesCountStore.reset();
      savedQuestionsStore.reset();
      mockEligibilityStore.reset();
      try {
        if ("serviceWorker" in navigator) {
          const registration = await navigator.serviceWorker.getRegistration("/");
          registration?.active?.postMessage({ type: "CLEAR_PRIVATE_CACHES" });
        }
      } catch {
        /* best-effort */
      }

      // `expired=1` is what tells the login screen to explain itself. Without
      // it the learner is dropped on a sign-in form with no idea why.
      router.replace(`/${locale}/login?expired=1`);
    }
  }, [locale, queryClient, router]);

  return null;
}
