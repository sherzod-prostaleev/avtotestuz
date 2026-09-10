"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useLocale } from "next-intl";
import { useMeQuery } from "@/hooks/use-me";

/**
 * Redirects a learner who still has a temporary password. The app shell stays
 * painted while /me resolves so dashboard navigation is not a full-screen wait.
 *
 * Deliberately no 401 branch. This used to own one, and it was the app's only
 * one — which is exactly why an expired session went unnoticed: /me is fetched
 * once per tab, so a session that died later never reached it. SessionExpiredGate
 * now hears every 401 from the transport, and it signs the browser out before
 * redirecting. A second redirect from here would race it and, because it left
 * the cookies in place, the middleware's cookie-presence check could bounce
 * /login straight back to /dashboard.
 */
export function MustChangePasswordGate({ children }: { children: React.ReactNode }) {
  const locale = useLocale();
  const router = useRouter();
  const meQuery = useMeQuery();

  useEffect(() => {
    if (meQuery.data?.profile?.must_change_password) {
      router.replace(`/${locale}/change-password`);
    }
  }, [locale, meQuery.data, router]);

  return <>{children}</>;
}
