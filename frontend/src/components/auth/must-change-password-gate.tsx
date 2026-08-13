"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { LoaderCircle } from "lucide-react";
import { apiGet, ApiError } from "@/lib/api-client";

export const ME_CHECK_TIMEOUT_MS = 8_000;

type MeResponse = {
  profile?: { must_change_password?: boolean };
};

/**
 * Blocks the learner app shell until a temporary password is replaced.
 * change-password lives outside (app), so this only runs for normal routes.
 */
export function MustChangePasswordGate({ children }: { children: React.ReactNode }) {
  const t = useTranslations("Errors");
  const locale = useLocale();
  const pathname = usePathname();
  const router = useRouter();
  const [allowed, setAllowed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), ME_CHECK_TIMEOUT_MS);

    (async () => {
      try {
        const me = await apiGet<MeResponse>("me", { signal: controller.signal });
        if (cancelled) return;
        if (me.profile?.must_change_password) {
          router.replace(`/${locale}/change-password`);
          return;
        }
        setAllowed(true);
      } catch (err) {
        if (cancelled) return;
        // Unauthenticated users are handled by middleware; keep shell usable
        // if /me briefly fails or times out so we don't blank the whole app.
        if (err instanceof ApiError && err.status === 401) {
          router.replace(`/${locale}/login`);
          return;
        }
        setAllowed(true);
      }
    })();

    return () => {
      cancelled = true;
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [locale, pathname, router]);

  if (!allowed) {
    return (
      <div
        className="flex min-h-screen flex-col items-center justify-center gap-3 bg-background px-6"
        role="status"
        aria-live="polite"
      >
        <LoaderCircle className="h-8 w-8 animate-spin text-accent" aria-hidden="true" />
        <p className="text-sm text-muted-foreground">{t("checkingSession")}</p>
      </div>
    );
  }

  return <>{children}</>;
}
