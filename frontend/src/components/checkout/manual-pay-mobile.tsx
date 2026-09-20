"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import {
  ExactAmountNotice,
  HoldNote,
  HumoPayCard,
  isManualClaimed,
  isManualPaid,
  type ManualPayInfo,
} from "@/components/checkout/manual-pay-parts";

export interface ManualPayMobileProps {
  info: ManualPayInfo;
  onClaim: () => void;
  claiming?: boolean;
  claimed?: boolean;
  className?: string;
}

/**
 * The card-transfer screen on a phone: what to send, where to send it, how long
 * the reservation lasts, and one button.
 *
 * The screen is exactly one viewport (`mobile-fit-screen`) and the button is
 * pinned to the bottom of it, with everything above in a scroller of its own.
 * Letting the column simply grow is what put "To'lov qildim" under the tab bar
 * on short phones — and a pay button nobody can reach is a lost sale, not a
 * layout nit.
 */
export function ManualPayMobile({
  info,
  onClaim,
  claiming,
  claimed,
  className = "",
}: ManualPayMobileProps) {
  const t = useTranslations("ManualPay");
  const router = useRouter();
  const paid = isManualPaid(info);
  const reported = claimed || isManualClaimed(info);

  return (
    <div className={`mobile-fit-screen flex flex-col gap-3 ${className}`}>
      <div className="flex shrink-0 items-center gap-1">
        <button
          type="button"
          onClick={() => router.back()}
          aria-label={t("title")}
          className="-ml-2 flex min-h-touch min-w-11 items-center justify-center text-muted-foreground"
        >
          <ArrowLeft aria-hidden="true" className="h-[22px] w-[22px]" />
        </button>
        <h1 className="min-w-0 flex-1 truncate font-display text-xl font-extrabold">{t("title")}</h1>
      </div>

      {/* `-mx-1 px-1`: the card's shadow would be shaved off by the scroller's
          own edges otherwise. */}
      <div
        data-testid="manual-scroll"
        className="-mx-1 flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto overscroll-contain px-1"
      >
        <HumoPayCard info={info} />
        <ExactAmountNotice amountUzs={info.amount_uzs} />
        <HoldNote untilIso={info.hold_until} />
      </div>

      {/* The artboard draws the happy path only, but these two are the sole
          signal that a claim was received — dropping them would leave a learner
          pressing the button again. Outside the scroller, so the answer cannot
          scroll away from the button that produced it. */}
      {paid ? (
        <p role="status" className="shrink-0 text-sm font-bold text-success-ink">
          {t("statusPaid")}
        </p>
      ) : reported ? (
        <p role="status" className="shrink-0 text-sm font-semibold leading-snug text-warning-ink">
          {t("statusReview")}
        </p>
      ) : null}

      <button
        type="button"
        data-testid="manual-claim"
        onClick={onClaim}
        disabled={claiming || reported || paid}
        className="btn-3d-primary inline-flex min-h-[50px] w-full shrink-0 items-center justify-center rounded-xl px-4 font-display text-lg font-extrabold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {reported ? t("claimed") : claiming ? t("claiming") : t("claim")}
      </button>

      <p className="pay-screen-optional shrink-0 text-center text-xs leading-snug text-muted-foreground">
        {t("autoActivateNote")}
      </p>
    </div>
  );
}
