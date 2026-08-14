"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useLocale } from "next-intl";
import { ApiError } from "@/lib/api-client";
import { useMeQuery } from "@/hooks/use-me";

/**
 * Redirects a learner who still has a temporary password. The app shell stays
 * painted while /me resolves so dashboard navigation is not a full-screen wait.
 */
export function MustChangePasswordGate({ children }: { children: React.ReactNode }) {
  const locale = useLocale();
  const pathname = usePathname();
  const router = useRouter();
  const meQuery = useMeQuery();

  useEffect(() => {
    if (meQuery.data?.profile?.must_change_password) {
      router.replace(`/${locale}/change-password`);
    }
  }, [locale, meQuery.data, router]);

  useEffect(() => {
    const err = meQuery.error;
    if (err instanceof ApiError && err.status === 401) {
      router.replace(`/${locale}/login`);
    }
  }, [locale, meQuery.error, pathname, router]);

  return <>{children}</>;
}
