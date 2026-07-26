"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { formatDateWithTime } from "@/lib/date-format";
import {
  Users,
  Copy,
  Check,
  Gift,
  Wallet,
  Percent,
  Clock,
  CheckCircle2,
  XCircle,
  Receipt,
  type LucideIcon,
} from "lucide-react";

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

export interface ReferralActivityResponse {
  payout_summary: {
    pending_count: number;
    pending_uzs: number;
    paid_count: number;
    paid_uzs: number;
    rejected_count: number;
    rejected_uzs: number;
  };
  payouts: Array<{
    id: string;
    amount_uzs: number;
    card_masked: string;
    card_network: string;
    status: string;
    admin_note?: string;
    created_at: string;
    processed_at?: string | null;
  }>;
  earnings: Array<{
    ledger_id: string;
    commission_uzs: number;
    payment_amount_uzs: number;
    tariff_code: string;
    tariff_days: number;
    percent_snapshot: number;
    referee_label: string;
    rewarded_at: string;
  }>;
}

function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

const PAYOUT_STATUS: Record<string, { icon: LucideIcon; labelKey: string; className: string }> = {
  pending: {
    icon: Clock,
    labelKey: "statusPending",
    className: "bg-gold/10 text-gold border-gold/25",
  },
  paid: {
    icon: CheckCircle2,
    labelKey: "statusPaid",
    className: "bg-success/10 text-success border-success/20",
  },
  rejected: {
    icon: XCircle,
    labelKey: "statusRejected",
    className: "bg-destructive/10 text-destructive border-destructive/20",
  },
};

export function ReferralCard() {
  const t = useTranslations("Referral");

  const [data, setData] = useState<ReferralResponse | null>(null);
  const [activity, setActivity] = useState<ReferralActivityResponse | null>(null);
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
      const [stats, act] = await Promise.all([
        apiGet<ReferralResponse>("me/referral"),
        apiGet<ReferralActivityResponse>("me/referral/activity").catch(() => null),
      ]);
      setData(stats);
      setActivity(act);
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

  const statusBadge = (status: string) => {
    const style = PAYOUT_STATUS[status.toLowerCase()] ?? PAYOUT_STATUS.pending;
    const Icon = style.icon;
    return (
      <span
        className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-bold ${style.className}`}
      >
        <Icon aria-hidden="true" className="h-3 w-3" />
        {t(style.labelKey as "statusPending")}
      </span>
    );
  };

  const summary = activity?.payout_summary;
  const hasPayouts = (activity?.payouts?.length ?? 0) > 0;
  const hasEarnings = (activity?.earnings?.length ?? 0) > 0;

  return (
    <Card className="border-accent/20 bg-card p-5 sm:p-6">
      <CardHeader className="mb-4 flex flex-row items-center justify-between p-0">
        <div className="flex items-center gap-2">
          <Gift aria-hidden="true" className="h-5 w-5 text-accent" />
          <CardTitle className="text-base font-bold">{t("title")}</CardTitle>
        </div>
        {data && (
          <span className="inline-flex items-center gap-1 rounded-full bg-accent/10 px-2.5 py-0.5 text-xs font-bold text-accent">
            <Percent aria-hidden="true" className="h-3 w-3" />
            {t("commissionBadge", { percent: data.commission_percent })}
          </span>
        )}
      </CardHeader>

      <p className="mb-4 text-xs text-muted-foreground">
        {data ? t("subtitleWithPercent", { percent: data.commission_percent }) : t("subtitle")}
      </p>

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

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
            <div className="rounded-lg border border-accent/30 bg-accent/5 p-3 text-center sm:col-span-1">
              <Percent aria-hidden="true" className="mx-auto mb-1 h-4 w-4 text-accent" />
              <div className="text-lg font-bold text-accent">{data.commission_percent}%</div>
              <div className="text-[11px] leading-tight text-muted-foreground">{t("statCommission")}</div>
            </div>
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
            {t("noteWithPercent", { percent: data.commission_percent })}
          </p>

          {activity && (
            <div className="space-y-4 border-t border-border pt-4">
              <div>
                <h4 className="mb-1 text-xs font-bold text-foreground">{t("monitorTitle")}</h4>
                <p className="mb-3 text-[11px] text-muted-foreground">{t("monitorSubtitle")}</p>
                {summary && (
                  <div className="grid grid-cols-3 gap-2">
                    <div className="rounded-lg border border-gold/25 bg-gold/5 px-2 py-2 text-center">
                      <div className="text-[10px] font-bold uppercase tracking-wide text-gold">{t("statusPending")}</div>
                      <div className="mt-0.5 font-mono text-sm font-bold text-foreground">
                        {formatSom(summary.pending_uzs)}
                      </div>
                      <div className="text-[10px] text-muted-foreground">
                        {t("countLabel", { count: summary.pending_count })}
                      </div>
                    </div>
                    <div className="rounded-lg border border-success/25 bg-success/5 px-2 py-2 text-center">
                      <div className="text-[10px] font-bold uppercase tracking-wide text-success">{t("statusPaid")}</div>
                      <div className="mt-0.5 font-mono text-sm font-bold text-foreground">
                        {formatSom(summary.paid_uzs)}
                      </div>
                      <div className="text-[10px] text-muted-foreground">
                        {t("countLabel", { count: summary.paid_count })}
                      </div>
                    </div>
                    <div className="rounded-lg border border-destructive/25 bg-destructive/5 px-2 py-2 text-center">
                      <div className="text-[10px] font-bold uppercase tracking-wide text-destructive">
                        {t("statusRejected")}
                      </div>
                      <div className="mt-0.5 font-mono text-sm font-bold text-foreground">
                        {formatSom(summary.rejected_uzs)}
                      </div>
                      <div className="text-[10px] text-muted-foreground">
                        {t("countLabel", { count: summary.rejected_count })}
                      </div>
                    </div>
                  </div>
                )}
              </div>

              <div>
                <h5 className="mb-2 flex items-center gap-1.5 text-xs font-bold text-foreground">
                  <Wallet aria-hidden="true" className="h-3.5 w-3.5 text-muted-foreground" />
                  {t("payoutHistoryTitle")}
                </h5>
                {!hasPayouts ? (
                  <p className="text-xs text-muted-foreground">{t("payoutHistoryEmpty")}</p>
                ) : (
                  <ul className="divide-y divide-border rounded-lg border border-border">
                    {activity.payouts.map((row) => (
                      <li key={row.id} className="flex flex-col gap-1.5 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-mono text-sm font-bold text-foreground">
                              {formatSom(row.amount_uzs)} {t("somSuffix")}
                            </span>
                            {statusBadge(row.status)}
                          </div>
                          <p className="mt-0.5 text-[11px] text-muted-foreground">
                            {row.card_masked} · {row.card_network.toUpperCase()} · {formatDateWithTime(row.created_at)}
                          </p>
                          {row.admin_note ? (
                            <p className="mt-0.5 text-[11px] text-muted-foreground/90">{row.admin_note}</p>
                          ) : null}
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <div>
                <h5 className="mb-2 flex items-center gap-1.5 text-xs font-bold text-foreground">
                  <Receipt aria-hidden="true" className="h-3.5 w-3.5 text-muted-foreground" />
                  {t("earningsTitle")}
                </h5>
                {!hasEarnings ? (
                  <p className="text-xs text-muted-foreground">{t("earningsEmpty")}</p>
                ) : (
                  <ul className="divide-y divide-border rounded-lg border border-border">
                    {activity.earnings.map((row) => (
                      <li key={row.ledger_id} className="px-3 py-2.5">
                        <div className="flex flex-wrap items-baseline justify-between gap-2">
                          <span className="text-sm font-semibold text-foreground">{row.referee_label}</span>
                          <span className="font-mono text-sm font-bold text-gold">
                            +{formatSom(row.commission_uzs)} {t("somSuffix")}
                          </span>
                        </div>
                        <p className="mt-0.5 text-[11px] text-muted-foreground">
                          {t("earningsLine", {
                            tariff: row.tariff_code || "—",
                            days: row.tariff_days,
                            payment: formatSom(row.payment_amount_uzs),
                            percent: row.percent_snapshot,
                          })}
                        </p>
                        <p className="text-[11px] text-muted-foreground/80">{formatDateWithTime(row.rewarded_at)}</p>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          )}

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
