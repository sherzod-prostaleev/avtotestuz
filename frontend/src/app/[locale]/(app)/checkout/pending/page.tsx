"use client";

import { useEffect } from "react";
import { useTranslations, useLocale } from "next-intl";
import { useRouter } from "next/navigation";
import { apiGet } from "@/lib/api-client";
import { Card } from "@/components/ui/card";
import { Loader2 } from "lucide-react";

interface EntitlementDTO {
  active: boolean;
  until: string | null;
  proration?: {
    applied: boolean;
    granted_days: number;
    tariff_days: number;
    reason: string;
  } | null;
}

export default function CheckoutPendingPage() {
  const t = useTranslations("Premium");
  const locale = useLocale();
  const router = useRouter();

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    let stopped = false;
    let attempt = 0;
    const schedule = () => {
      if (stopped) return;
      const delay = document.visibilityState === "hidden" || !navigator.onLine
        ? 30_000
        : Math.min(3_000 * 2 ** attempt, 30_000);
      timer = setTimeout(() => void checkStatus(), delay);
    };
    const checkStatus = async () => {
      if (stopped) return;
      if (document.visibilityState === "hidden" || !navigator.onLine) {
        schedule();
        return;
      }
      try {
        const ent = await apiGet<EntitlementDTO>("me/entitlement");
        attempt = 0;
        if (ent.active) {
          stopped = true;
          const params = new URLSearchParams();
          if (ent.proration?.applied) {
            params.set("prorated", "1");
            params.set("granted", String(ent.proration.granted_days));
            params.set("tariff", String(ent.proration.tariff_days));
          }
          const qs = params.toString();
          router.push(`/${locale}/checkout/success${qs ? `?${qs}` : ""}`);
          return;
        }
      } catch {
        attempt++;
      }
      schedule();
    };

    const wake = () => {
      if (document.visibilityState === "hidden" || !navigator.onLine) return;
      if (timer) clearTimeout(timer);
      attempt = 0;
      void checkStatus();
    };

    void checkStatus();
    document.addEventListener("visibilitychange", wake);
    window.addEventListener("online", wake);

    return () => {
      stopped = true;
      if (timer) clearTimeout(timer);
      document.removeEventListener("visibilitychange", wake);
      window.removeEventListener("online", wake);
    };
  }, [locale, router]);

  return (
    <main className="mx-auto flex min-h-[70vh] max-w-md flex-col items-center justify-center p-4 text-center">
      <Card className="flex w-full flex-col items-center p-6 sm:p-8">
        <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full border-2 border-accent/40 bg-accent/10 text-accent">
          <Loader2 className="h-8 w-8 animate-spin" />
        </div>

        <h1 className="font-display text-2xl font-extrabold tracking-tight text-foreground sm:text-3xl">
          {t("checkoutPendingTitle")}
        </h1>
        <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
          {t("checkoutPendingSubtitle")}
        </p>
      </Card>
    </main>
  );
}
