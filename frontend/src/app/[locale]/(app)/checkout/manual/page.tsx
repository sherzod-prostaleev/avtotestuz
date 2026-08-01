"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter, useSearchParams } from "next/navigation";
import { apiGet, apiPost } from "@/lib/api-client";
import { ManualPayCard, ManualPayInfo } from "@/components/checkout/manual-pay-card";
import { Button } from "@/components/ui/button";

export default function ManualCheckoutPage() {
  const t = useTranslations("ManualPay");
  const locale = useLocale();
  const router = useRouter();
  const params = useSearchParams();
  const paymentId = params.get("payment_id") ?? "";
  const [info, setInfo] = useState<ManualPayInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [claiming, setClaiming] = useState(false);
  const [claimed, setClaimed] = useState(false);

  const load = useCallback(async () => {
    if (!paymentId) return false;
    try {
      const data = await apiGet<ManualPayInfo>(`me/payments/${paymentId}/manual`);
      setInfo(data);
      setError(null);
      if (data.payment_status === "paid" || data.manual_state === "consumed") {
        router.replace(`/${locale}/checkout/success`);
      }
      return true;
    } catch {
      setError(t("loadError"));
      return false;
    }
  }, [paymentId, locale, router, t]);

  useEffect(() => {
    let stopped = false;
    let attempt = 0;
    let timer: number | undefined;
    const poll = async () => {
      if (stopped) return;
      if (document.visibilityState !== "hidden" && navigator.onLine) {
        const ok = await load();
        if (ok) {
          attempt = 0;
        } else {
          attempt++;
        }
      }
      if (!stopped) {
        timer = window.setTimeout(() => void poll(), Math.min(4_000 * 2 ** attempt, 30_000));
      }
    };
    const wake = () => {
      if (document.visibilityState === "hidden" || !navigator.onLine) return;
      if (timer) window.clearTimeout(timer);
      attempt = 0;
      void poll();
    };
    void poll();
    document.addEventListener("visibilitychange", wake);
    window.addEventListener("online", wake);
    return () => {
      stopped = true;
      if (timer) window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", wake);
      window.removeEventListener("online", wake);
    };
  }, [load]);

  async function claim() {
    if (!paymentId) return;
    setClaiming(true);
    try {
      const data = await apiPost<ManualPayInfo>(`me/payments/${paymentId}/manual/claim`, {});
      setInfo(data);
      setClaimed(true);
    } catch {
      setError(t("claimError"));
    } finally {
      setClaiming(false);
    }
  }

  if (!paymentId) {
    return (
      <main className="mx-auto max-w-lg px-4 py-10">
        <p className="text-sm text-muted-foreground">{t("missingPayment")}</p>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-lg space-y-6 px-4 py-8">
      <div>
        <h1 className="font-display text-2xl font-bold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>
      {error ? <p className="text-sm text-destructive">{error}</p> : null}
      {info ? (
        <ManualPayCard info={info} onClaim={() => void claim()} claiming={claiming} claimed={claimed} />
      ) : (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      )}
      <Button type="button" variant="outline" className="w-full" onClick={() => void load()}>
        {t("refresh")}
      </Button>
    </main>
  );
}
