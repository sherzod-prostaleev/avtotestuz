"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Users, Copy, Check, Gift, Wallet, Percent } from "lucide-react";

export interface ReferralResponse {
  referral_code: string;
  invite_url: string;
  total_invited: number;
  total_rewarded: number;
  earned_uzs: number;
  available_balance_uzs: number;
  commission_percent: number;
  bonus_days_earned?: number;
}

function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

export function ReferralCard() {
  const t = useTranslations("Referral");

  const [data, setData] = useState<ReferralResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [copied, setCopied] = useState(false);

  const [inputCode, setInputCode] = useState("");
  const [applying, setApplying] = useState(false);
  const [applyMessage, setApplyMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const [payoutAmount, setPayoutAmount] = useState("");
  const [cardNumber, setCardNumber] = useState("");
  const [cardNetwork, setCardNetwork] = useState<"uzcard" | "humo">("uzcard");
  const [payoutLoading, setPayoutLoading] = useState(false);
  const [payoutMessage, setPayoutMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const loadReferralData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const res = await apiGet<ReferralResponse>("me/referral");
      setData(res);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadReferralData();
  }, [loadReferralData]);

  const handleCopyLink = async () => {
    if (!data?.invite_url) return;
    try {
      await navigator.clipboard.writeText(data.invite_url);
      setCopied(true);
      setTimeout(() => setCopied(false), 3000);
    } catch {
      // ignore
    }
  };

  const handleApplyCode = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputCode.trim()) return;
    setApplying(true);
    setApplyMessage(null);
    try {
      await apiPost<{ applied: boolean }>("referral/apply", { code: inputCode.trim() });
      setApplyMessage({ type: "success", text: t("applySuccess") });
      setInputCode("");
      void loadReferralData();
    } catch (err: unknown) {
      let errMsg = t("applyError");
      if (err instanceof ApiError) {
        switch (err.code) {
          case "referral_self":
            errMsg = t("errorSelf");
            break;
          case "referral_not_found":
            errMsg = t("errorNotFound");
            break;
          case "referral_already_applied":
            errMsg = t("errorAlreadyApplied");
            break;
          case "referral_not_eligible_paid":
            errMsg = t("errorNotEligiblePaid");
            break;
          case "referral_window_closed":
            errMsg = t("errorWindowClosed");
            break;
          default:
            errMsg = t("applyError");
        }
      }
      setApplyMessage({ type: "error", text: errMsg });
    } finally {
      setApplying(false);
    }
  };

  const handlePayout = async (e: React.FormEvent) => {
    e.preventDefault();
    const amount = Number(payoutAmount.replace(/\s/g, ""));
    if (!Number.isFinite(amount) || amount <= 0) {
      setPayoutMessage({ type: "error", text: t("payoutInvalidAmount") });
      return;
    }
    setPayoutLoading(true);
    setPayoutMessage(null);
    try {
      await apiPost("me/referral/payout", {
        amount_uzs: amount,
        card_number: cardNumber,
        card_network: cardNetwork,
      });
      setPayoutMessage({ type: "success", text: t("payoutSuccess") });
      setPayoutAmount("");
      setCardNumber("");
      void loadReferralData();
    } catch (err: unknown) {
      let errMsg = t("payoutError");
      if (err instanceof ApiError) {
        switch (err.code) {
          case "payout_insufficient_balance":
            errMsg = t("payoutInsufficient");
            break;
          case "payout_invalid_card":
            errMsg = t("payoutInvalidCard");
            break;
          case "payout_invalid_network":
            errMsg = t("payoutInvalidNetwork");
            break;
          case "payout_invalid_amount":
            errMsg = t("payoutInvalidAmount");
            break;
          default:
            break;
        }
      }
      setPayoutMessage({ type: "error", text: errMsg });
    } finally {
      setPayoutLoading(false);
    }
  };

  return (
    <Card className="border-accent/20 bg-card p-5 sm:p-6">
      <CardHeader className="mb-4 flex flex-row items-center justify-between p-0">
        <div className="flex items-center gap-2">
          <Gift aria-hidden="true" className="h-5 w-5 text-accent" />
          <CardTitle className="text-base font-bold">{t("title")}</CardTitle>
        </div>
        <span className="inline-flex items-center gap-1 rounded-full bg-accent/10 px-2.5 py-0.5 text-xs font-bold text-accent">
          <Percent aria-hidden="true" className="h-3 w-3" />
          {t("commissionBadge", { percent: data?.commission_percent ?? 20 })}
        </span>
      </CardHeader>

      <p className="mb-4 text-xs text-muted-foreground">{t("subtitle")}</p>

      {loading && (
        <div role="status" className="text-sm text-muted-foreground">
          {t("loading")}
        </div>
      )}

      {error && (
        <div role="alert" className="text-sm text-destructive">
          {t("loadError")}
        </div>
      )}

      {!loading && !error && data && (
        <div className="space-y-6">
          <div className="rounded-xl border border-border bg-card/60 p-4">
            <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
              <div>
                <span className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
                  {t("yourCode")}
                </span>
                <p className="font-mono text-xl font-extrabold tracking-widest text-foreground">
                  {data.referral_code}
                </p>
              </div>
              <Button type="button" variant="game" size="sm" onClick={handleCopyLink} className="flex items-center gap-1.5">
                {copied ? (
                  <>
                    <Check aria-hidden="true" className="h-4 w-4" /> {t("linkCopied")}
                  </>
                ) : (
                  <>
                    <Copy aria-hidden="true" className="h-4 w-4" /> {t("copyLink")}
                  </>
                )}
              </Button>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div className="rounded-lg border border-border bg-card p-3 text-center">
              <Users aria-hidden="true" className="mx-auto mb-1 h-4 w-4 text-muted-foreground" />
              <div className="text-lg font-bold text-foreground">{data.total_invited}</div>
              <div className="text-[11px] leading-tight text-muted-foreground">{t("statInvited")}</div>
            </div>
            <div className="rounded-lg border border-border bg-card p-3 text-center">
              <Gift aria-hidden="true" className="mx-auto mb-1 h-4 w-4 text-accent" />
              <div className="text-lg font-bold text-accent">{data.total_rewarded}</div>
              <div className="text-[11px] leading-tight text-muted-foreground">{t("statRewarded")}</div>
            </div>
            <div className="rounded-lg border border-border bg-card p-3 text-center">
              <Wallet aria-hidden="true" className="mx-auto mb-1 h-4 w-4 text-gold" />
              <div className="text-lg font-bold text-gold">{formatSom(data.earned_uzs)}</div>
              <div className="text-[11px] leading-tight text-muted-foreground">{t("statEarned")}</div>
            </div>
            <div className="rounded-lg border border-border bg-card p-3 text-center">
              <Wallet aria-hidden="true" className="mx-auto mb-1 h-4 w-4 text-foreground" />
              <div className="text-lg font-bold text-foreground">{formatSom(data.available_balance_uzs)}</div>
              <div className="text-[11px] leading-tight text-muted-foreground">{t("statBalance")}</div>
            </div>
          </div>

          <p className="rounded-lg border border-accent/10 bg-accent/5 p-3 text-xs italic text-muted-foreground/90">
            {t("note")}
          </p>

          <div className="border-t border-border pt-4">
            <h4 className="mb-2 text-xs font-bold text-foreground">{t("payoutTitle")}</h4>
            <form onSubmit={handlePayout} className="space-y-2">
              <div className="grid gap-2 sm:grid-cols-2">
                <input
                  type="text"
                  inputMode="numeric"
                  value={payoutAmount}
                  onChange={(e) => setPayoutAmount(e.target.value)}
                  placeholder={t("payoutAmountPlaceholder")}
                  disabled={payoutLoading || data.available_balance_uzs <= 0}
                  className="w-full rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-accent focus:outline-none"
                />
                <input
                  type="text"
                  inputMode="numeric"
                  value={cardNumber}
                  onChange={(e) => setCardNumber(e.target.value.replace(/[^\d\s]/g, ""))}
                  placeholder={t("payoutCardPlaceholder")}
                  maxLength={19}
                  disabled={payoutLoading || data.available_balance_uzs <= 0}
                  className="w-full rounded-md border border-border bg-card px-3 py-2 font-mono text-sm text-foreground placeholder:text-muted-foreground focus:border-accent focus:outline-none"
                />
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <label className="flex items-center gap-1.5 text-xs">
                  <input
                    type="radio"
                    name="card_network"
                    checked={cardNetwork === "uzcard"}
                    onChange={() => setCardNetwork("uzcard")}
                  />
                  Uzcard
                </label>
                <label className="flex items-center gap-1.5 text-xs">
                  <input
                    type="radio"
                    name="card_network"
                    checked={cardNetwork === "humo"}
                    onChange={() => setCardNetwork("humo")}
                  />
                  Humo
                </label>
                <Button
                  type="submit"
                  variant="outline"
                  size="sm"
                  className="ml-auto"
                  disabled={payoutLoading || data.available_balance_uzs <= 0 || !cardNumber.trim()}
                >
                  {payoutLoading ? t("payoutSubmitting") : t("payoutButton")}
                </Button>
              </div>
            </form>
            {payoutMessage && (
              <p
                className={`mt-2 text-xs font-medium ${
                  payoutMessage.type === "success" ? "text-success" : "text-destructive"
                }`}
              >
                {payoutMessage.text}
              </p>
            )}
          </div>

          <div className="border-t border-border pt-4">
            <h4 className="mb-2 text-xs font-bold text-foreground">{t("applyTitle")}</h4>
            <form onSubmit={handleApplyCode} className="flex gap-2">
              <input
                type="text"
                value={inputCode}
                onChange={(e) => setInputCode(e.target.value.toUpperCase())}
                placeholder={t("applyPlaceholder")}
                maxLength={16}
                disabled={applying}
                className="w-full rounded-md border border-border bg-card px-3 py-2 font-mono text-sm uppercase tracking-wider text-foreground placeholder:text-muted-foreground focus:border-accent focus:outline-none"
              />
              <Button type="submit" variant="outline" size="sm" disabled={applying || !inputCode.trim()}>
                {applying ? t("applying") : t("applyButton")}
              </Button>
            </form>
            {applyMessage && (
              <p
                className={`mt-2 text-xs font-medium ${
                  applyMessage.type === "success" ? "text-success" : "text-destructive"
                }`}
              >
                {applyMessage.text}
              </p>
            )}
          </div>
        </div>
      )}
    </Card>
  );
}
