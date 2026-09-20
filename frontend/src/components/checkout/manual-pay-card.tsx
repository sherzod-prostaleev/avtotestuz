"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  ExactAmountNotice,
  HoldNote,
  HumoPayCard,
  isManualClaimed,
  isManualPaid,
  type ManualPayInfo,
} from "@/components/checkout/manual-pay-parts";

/**
 * The card-transfer screen from `md` up. Same four blocks as the phone body in
 * `manual-pay-mobile.tsx` — card, exact sum, window, claim — laid out for a
 * column that may simply grow, since nothing here has to fit a fold.
 */
export function ManualPayCard({
  info,
  onClaim,
  claiming,
  claimed,
}: {
  info: ManualPayInfo;
  onClaim: () => void;
  claiming?: boolean;
  claimed?: boolean;
}) {
  const t = useTranslations("ManualPay");
  const paid = isManualPaid(info);
  const reported = claimed || isManualClaimed(info);

  return (
    <div className="mx-auto w-full max-w-md space-y-4">
      <HumoPayCard info={info} />
      <ExactAmountNotice amountUzs={info.amount_uzs} />
      <HoldNote untilIso={info.hold_until} />

      {paid ? (
        <p role="status" className="rounded-xl border border-success/40 bg-success/10 px-3 py-2 text-sm font-semibold text-success-ink">
          {t("statusPaid")}
        </p>
      ) : reported ? (
        <p role="status" className="rounded-xl border border-warning/40 bg-warning/10 px-3 py-2 text-sm text-warning-ink">
          {t("statusReview")}
        </p>
      ) : null}

      {!paid && (
        <Button type="button" className="w-full" disabled={claiming || reported} onClick={onClaim}>
          {reported ? t("claimed") : claiming ? t("claiming") : t("claim")}
        </Button>
      )}
    </div>
  );
}
