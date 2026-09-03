"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Check, ChevronRight, Copy, Wallet } from "lucide-react";
import { ApiError, apiGet, apiPost } from "@/lib/api-client";
import type { ReferralResponse } from "@/components/profile/referral-card";
import { MobileScreen } from "./mobile-screen";

function groupDigits(value: number): string {
  return String(value).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

/**
 * Uzcard cards start 8600, Humo 9860. The payout endpoint requires the network
 * and the artboard drops the radio buttons, so it is read off the number —
 * and an unrecognised prefix is refused here rather than guessed and sent.
 */
function networkForPan(pan: string): "uzcard" | "humo" | null {
  const digits = pan.replace(/\D/g, "");
  if (digits.startsWith("8600")) return "uzcard";
  if (digits.startsWith("9860")) return "humo";
  return null;
}

const PAYOUT_ERRORS: Record<string, string> = {
  payout_insufficient_balance: "payoutInsufficient",
  payout_invalid_card: "payoutInvalidCard",
  payout_invalid_network: "payoutInvalidNetwork",
};

/**
 * The referral programme on a phone, and the payout form behind it.
 *
 * Payout is a step inside this screen rather than a seventh panel: it is only
 * reachable from the balance row, and it needs the balance the screen has
 * already loaded.
 */
export function MobileReferral({ onBack }: { onBack: () => void }) {
  const t = useTranslations("Referral");
  const [data, setData] = useState<ReferralResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [copied, setCopied] = useState(false);
  const [step, setStep] = useState<"main" | "payout">("main");

  const [amount, setAmount] = useState("");
  const [pan, setPan] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const [inputCode, setInputCode] = useState("");
  const [applying, setApplying] = useState(false);
  const [applyMessage, setApplyMessage] = useState<{ type: "success" | "error"; text: string } | null>(
    null,
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      setData(await apiGet<ReferralResponse>("me/referral"));
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function copyLink() {
    if (!data) return;
    try {
      await navigator.clipboard.writeText(data.invite_url);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // A denied clipboard permission is not an error worth a banner — the
      // code is on screen and can be typed.
    }
  }

  async function applyCode() {
    const code = inputCode.trim();
    if (!code) return;
    setApplying(true);
    setApplyMessage(null);
    try {
      await apiPost<{ applied: boolean }>("referral/apply", { code });
      setApplyMessage({ type: "success", text: t("applySuccess") });
      setInputCode("");
      await load();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "";
      const key =
        code === "referral_already_applied"
          ? "errorAlreadyApplied"
          : code === "referral_self"
            ? "errorSelf"
            : code === "referral_not_found"
              ? "errorNotFound"
              : "applyError";
      setApplyMessage({ type: "error", text: t(key) });
    } finally {
      setApplying(false);
    }
  }

  async function submitPayout() {
    const value = Number(amount.replace(/\s/g, ""));
    if (!Number.isFinite(value) || value <= 0) {
      setMessage({ type: "error", text: t("payoutInvalidAmount") });
      return;
    }
    const network = networkForPan(pan);
    if (!network) {
      setMessage({ type: "error", text: t("payoutInvalidCard") });
      return;
    }
    setSubmitting(true);
    setMessage(null);
    try {
      await apiPost("me/referral/payout", {
        amount_uzs: value,
        card_number: pan.replace(/\D/g, ""),
        card_network: network,
      });
      setMessage({ type: "success", text: t("payoutSuccess") });
      setAmount("");
      setPan("");
      await load();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "";
      setMessage({ type: "error", text: t(PAYOUT_ERRORS[code] ?? "payoutError") });
    } finally {
      setSubmitting(false);
    }
  }

  if (step === "payout") {
    return (
      <MobileScreen
        title={t("payoutTitle")}
        onBack={() => setStep("main")}
        gapClassName="gap-2.5"
        footer={
          <button
            type="button"
            onClick={() => void submitPayout()}
            disabled={submitting}
            className="btn-3d-primary inline-flex min-h-[50px] w-full items-center justify-center rounded-xl px-4 font-display text-lg font-extrabold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {submitting ? t("payoutSubmitting") : t("payoutButton")}
          </button>
        }
      >
        <div className="surface-raised flex items-center justify-between gap-3 rounded-2xl border border-border bg-card p-3">
          <span className="text-sm text-muted-foreground">{t("balanceShort")}</span>
          <span className="font-display text-xl font-extrabold text-gold tabular-nums">
            {groupDigits(data?.available_balance_uzs ?? 0)}{" "}
            <span className="text-sm font-bold text-muted-foreground">{t("somSuffix")}</span>
          </span>
        </div>

        <div>
          <label htmlFor="payout-amount" className="mb-1.5 block text-xs font-bold text-muted-foreground">
            {t("payoutAmountPlaceholder")}
          </label>
          <input
            id="payout-amount"
            inputMode="numeric"
            value={amount}
            onChange={(e) => {
              setAmount(e.target.value);
              setMessage(null);
            }}
            className="field-input"
          />
        </div>

        <div>
          <label htmlFor="payout-card" className="mb-1.5 block text-xs font-bold text-muted-foreground">
            {t("payoutCardPlaceholder")}
          </label>
          <input
            id="payout-card"
            inputMode="numeric"
            value={pan}
            onChange={(e) => {
              setPan(e.target.value);
              setMessage(null);
            }}
            className="field-input tabular-nums"
          />
          {/* Named back to the sender, so a mistyped first four digits is
              visible before the request rather than after it. */}
          <p className="mt-1 text-xs text-muted-foreground">
            {networkForPan(pan) === "uzcard"
              ? "Uzcard"
              : networkForPan(pan) === "humo"
                ? "Humo"
                : t("payoutInvalidNetwork")}
          </p>
        </div>

        {message && (
          <p
            role={message.type === "error" ? "alert" : "status"}
            className={`text-sm font-medium ${
              message.type === "error" ? "text-destructive" : "text-success"
            }`}
          >
            {message.text}
          </p>
        )}
      </MobileScreen>
    );
  }

  return (
    <MobileScreen title={t("title")} onBack={onBack} gapClassName="gap-2.5">
      {loading ? (
        <div aria-hidden="true" className="flex flex-col gap-2.5">
          <span className="block h-28 animate-pulse rounded-2xl bg-border/50" />
          <span className="block h-16 animate-pulse rounded-2xl bg-border/50" />
        </div>
      ) : error || !data ? (
        <div role="alert" className="rounded-2xl border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          <p>{t("loadError")}</p>
          <button type="button" onClick={() => void load()} className="mt-2 min-h-11 text-sm font-bold underline">
            {t("applyButton")}
          </button>
        </div>
      ) : (
        <>
          <div className="surface-raised rounded-2xl border border-accent/40 bg-accent/[0.06] p-3">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-extrabold uppercase tracking-[0.08em] text-muted-foreground">
                {t("yourCodeShort")}
              </span>
              <span className="shrink-0 rounded-full bg-accent/15 px-2 py-0.5 text-xs font-extrabold text-accent">
                {t("commissionBadge", { percent: data.commission_percent })}
              </span>
            </div>
            <p className="mt-1 font-display text-3xl font-extrabold tracking-wide">
              {data.referral_code}
            </p>
            <button
              type="button"
              onClick={() => void copyLink()}
              className="mt-2 flex min-h-11 w-full items-center justify-center gap-2 rounded-xl border border-border bg-card text-sm font-bold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {copied ? (
                <Check aria-hidden="true" className="h-4 w-4 text-success" />
              ) : (
                <Copy aria-hidden="true" className="h-4 w-4" />
              )}
              {copied ? t("linkCopied") : t("copyLink")}
            </button>
          </div>

          <button
            type="button"
            onClick={() => setStep("payout")}
            className="flex items-center gap-3 rounded-2xl border border-border bg-card p-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <Wallet aria-hidden="true" className="h-5 w-5 shrink-0 text-gold" />
            <div className="min-w-0 flex-1">
              <p className="text-xs text-muted-foreground">{t("balanceShort")}</p>
              <p className="truncate font-display text-xl font-extrabold tabular-nums">
                {groupDigits(data.available_balance_uzs)}{" "}
                <span className="text-sm font-bold text-muted-foreground">{t("somSuffix")}</span>
              </p>
            </div>
            <span className="flex shrink-0 items-center gap-1 whitespace-nowrap text-sm font-bold text-accent">
              {t("payoutShort")}
              <ChevronRight aria-hidden="true" className="h-4 w-4" />
            </span>
          </button>

          <div className="grid grid-cols-3 gap-1.5">
            {[
              { key: "invited", value: data.total_invited, label: t("statInvitedShort") },
              { key: "rewarded", value: data.total_rewarded, label: t("statRewardedShort") },
              { key: "days", value: data.bonus_days_earned ?? 0, label: t("statDaysShort") },
            ].map((tile) => (
              <div key={tile.key} className="rounded-2xl border border-border bg-card p-2 text-center">
                <p className="font-display text-xl font-extrabold tabular-nums">{tile.value}</p>
                <p className="mt-0.5 text-xs leading-[1.15] text-muted-foreground">{tile.label}</p>
              </div>
            ))}
          </div>

          <p className="text-xs leading-snug text-muted-foreground">
            {t("noteShort")}
          </p>

          <div>
            <p className="mb-1 text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
              {t("applyTitle")}
            </p>
            <div className="flex gap-2">
              <input
                aria-label={t("applyTitle")}
                value={inputCode}
                onChange={(e) => {
                  setInputCode(e.target.value);
                  setApplyMessage(null);
                }}
                placeholder={t("applyPlaceholder")}
                className="field-input min-w-0 flex-1"
              />
              <button
                type="button"
                onClick={() => void applyCode()}
                disabled={applying || !inputCode.trim()}
                className="min-h-11 shrink-0 rounded-xl border border-border bg-card px-3 text-sm font-bold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {applying ? t("applying") : t("applyButton")}
              </button>
            </div>
            {applyMessage && (
              <p
                role={applyMessage.type === "error" ? "alert" : "status"}
                className={`mt-1 text-xs font-medium ${
                  applyMessage.type === "error" ? "text-destructive" : "text-success"
                }`}
              >
                {applyMessage.text}
              </p>
            )}
          </div>
        </>
      )}
    </MobileScreen>
  );
}
