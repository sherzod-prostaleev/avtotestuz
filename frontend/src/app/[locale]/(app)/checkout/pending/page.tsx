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
}

export default function CheckoutPendingPage() {
  const t = useTranslations("Premium");
  const locale = useLocale();
  const router = useRouter();

  useEffect(() => {
    let timer: ReturnType<typeof setInterval>;
    const checkStatus = async () => {
      try {
        const ent = await apiGet<EntitlementDTO>("me/entitlement");
        if (ent.active) {
          router.push(`/${locale}/checkout/success`);
        }
      } catch {
        // continue polling
      }
    };

    void checkStatus();
    timer = setInterval(() => {
      void checkStatus();
    }, 3000);

    return () => clearInterval(timer);
  }, [locale, router]);

  return (
    <main className="mx-auto flex min-h-[70vh] max-w-md flex-col items-center justify-center p-4 text-center">
      <Card className="w-full p-6 sm:p-8 flex flex-col items-center">
        <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-accent/10 border-2 border-accent/40 text-accent">
          <Loader2 className="h-8 w-8 animate-spin" />
        </div>

        <h1 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl text-foreground">
          {t("checkoutPendingTitle")}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground leading-relaxed">
          {t("checkoutPendingSubtitle")}
        </p>
      </Card>
    </main>
  );
}
