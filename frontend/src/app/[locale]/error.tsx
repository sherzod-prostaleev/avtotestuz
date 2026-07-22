"use client";

import { useEffect } from "react";
import { AlertTriangle, RotateCcw } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

export default function ErrorBoundary({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations("Errors");

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
        <Button className="mt-6 min-h-11" onClick={reset}>
          <RotateCcw className="mr-2 h-4 w-4" aria-hidden="true" />
          {t("retry")}
        </Button>
      </section>
    </main>
  );
}
