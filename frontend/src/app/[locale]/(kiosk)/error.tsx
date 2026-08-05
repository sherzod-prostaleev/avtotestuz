"use client";

import { useEffect } from "react";
import Link from "next/link";
import { AlertTriangle, Home } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

// Kiosk-scoped so Next.js resolves this boundary instead of the shared
// [locale]/error.tsx. That shared boundary offers only a `reset()` retry
// with no link today, but it is the same class of surface as not-found.tsx
// and must not gain a route out of the kiosk later. This one offers a
// single way back to /station instead of retrying in place, since a
// walk-up student has no way to judge whether retrying is safe.
export default function KioskError({
  error,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations("Errors");
  const locale = useLocale();

  useEffect(() => {
    // Keep the technical detail in developer tooling; users receive stable,
    // localized recovery copy instead of raw backend/runtime errors.
    console.error(error);
  }, [error]);

  return (
    <main className="grid min-h-[70vh] place-items-center px-6 py-12" role="alert">
      <section className="w-full max-w-lg rounded-3xl border bg-card p-8 text-center shadow-sm">
        <AlertTriangle className="mx-auto h-12 w-12 text-amber-600" aria-hidden="true" />
        <h1 className="mt-5 font-display text-3xl font-extrabold">{t("genericTitle")}</h1>
        <p className="mt-3 text-muted-foreground">{t("genericMessage")}</p>
        <Link
          href={`/${locale}/station`}
          className="mt-6 inline-flex min-h-11 items-center justify-center rounded-xl border-b-4 border-accent-shadow bg-accent px-5 text-sm font-bold tracking-wide text-accent-foreground shadow-3d transition-all duration-150 hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:shadow-none"
        >
          <Home className="mr-2 h-4 w-4" aria-hidden="true" />
          {t("backToStation")}
        </Link>
      </section>
    </main>
  );
}
