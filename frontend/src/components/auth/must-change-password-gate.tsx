"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useLocale } from "next-intl";
import { apiGet, ApiError } from "@/lib/api-client";

type MeResponse = {
  profile: { must_change_password?: boolean };
};

/**
 * Blocks the learner app shell until a temporary password is replaced.
 * change-password lives outside (app), so this only runs for normal routes.
 */
export function MustChangePasswordGate({ children }: { children: React.ReactNode }) {
  const locale = useLocale();
  const pathname = usePathname();
  const router = useRouter();
  const [allowed, setAllowed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const me = await apiGet<MeResponse>("me");
        if (cancelled) return;
        if (me.profile.must_change_password) {
          router.replace(`/${locale}/change-password`);
          return;
        }
        setAllowed(true);
      } catch (err) {
        if (cancelled) return;
        // Unauthenticated users are handled by middleware; keep shell usable
        // if /me briefly fails so we don't blank the whole app.
        if (err instanceof ApiError && err.status === 401) {
          router.replace(`/${locale}/login`);
          return;
        }
        setAllowed(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [locale, pathname, router]);

  if (!allowed) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        …
      </div>
    );
  }

  return <>{children}</>;
}
