"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { XCircle, RefreshCw } from "lucide-react";

export default function CheckoutFailurePage() {
  const t = useTranslations("Premium");
  const locale = useLocale();

  return (
    <main className="mx-auto flex min-h-[70vh] max-w-md flex-col items-center justify-center p-4 text-center">
      <Card className="w-full p-6 sm:p-8 flex flex-col items-center">
        <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-destructive/10 border-2 border-destructive/40 text-destructive">
          <XCircle className="h-8 w-8" />
        </div>

        <h1 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl text-foreground">
          {t("checkoutFailureTitle")}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground leading-relaxed">
          {t("checkoutFailureSubtitle")}
        </p>

        <div className="mt-6 flex w-full flex-col gap-2">
          <Link href={`/${locale}/premium`} className="w-full">
            <Button as="span" variant="gold" size="lg" className="w-full gap-2">
              <RefreshCw className="h-4 w-4" /> {t("checkoutTryAgain")}
            </Button>
          </Link>
          <Link href={`/${locale}/dashboard`} className="w-full">
            <Button as="span" variant="outline" size="lg" className="w-full">
              {t("backHome")}
            </Button>
          </Link>
        </div>
      </Card>
    </main>
  );
}
