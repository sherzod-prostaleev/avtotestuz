"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Users, Copy, Check, Gift, Award, Sparkles } from "lucide-react";

export interface ReferralResponse {
  referral_code: string;
  invite_url: string;
  total_invited: number;
  total_rewarded: number;
  bonus_days_earned: number;
}

export function ReferralCard() {
  const t = useTranslations("Referral");

  const [data, setData] = useState<ReferralResponse | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<boolean>(false);
  const [copied, setCopied] = useState<boolean>(false);

  const [inputCode, setInputCode] = useState<string>("");
  const [applying, setApplying] = useState<boolean>(false);
  const [applyMessage, setApplyMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

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
      // fallback if clipboard fails
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
          default:
            errMsg = t("applyError");
        }
      }
      setApplyMessage({ type: "error", text: errMsg });
    } finally {
      setApplying(false);
    }
  };

  return (
    <Card className="p-6 border-accent/20 bg-gradient-to-br from-card to-accent/5">
      <CardHeader className="p-0 mb-4 flex flex-row items-center justify-between">
        <div className="flex items-center gap-2">
          <Gift aria-hidden="true" className="h-5 w-5 text-accent" />
          <CardTitle className="text-base font-bold">{t("title")}</CardTitle>
        </div>
        <span className="inline-flex items-center gap-1 rounded-full bg-accent/10 px-2.5 py-0.5 text-xs font-bold text-accent">
          <Sparkles aria-hidden="true" className="h-3 w-3" /> {t("bonusBadge", { days: 7 })}
        </span>
      </CardHeader>

      <p className="text-xs text-muted-foreground mb-4">{t("subtitle")}</p>

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
          {/* Referral Code & Copy Box */}
          <div className="rounded-xl border border-border bg-card/60 p-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div>
                <span className="text-xs font-bold text-muted-foreground uppercase tracking-wider">{t("yourCode")}</span>
                <p className="font-mono text-xl font-extrabold text-foreground tracking-widest">{data.referral_code}</p>
              </div>
              <Button
                type="button"
                variant="game"
                size="sm"
                onClick={handleCopyLink}
                className="flex items-center gap-1.5"
              >
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

          {/* Stats Grid */}
          <div className="grid grid-cols-3 gap-3">
            <div className="rounded-lg border border-border bg-card p-3 text-center">
              <Users aria-hidden="true" className="mx-auto mb-1 h-4 w-4 text-muted-foreground" />
              <div className="text-lg font-bold text-foreground">{data.total_invited}</div>
              <div className="text-[11px] text-muted-foreground leading-tight">{t("statInvited")}</div>
            </div>

            <div className="rounded-lg border border-border bg-card p-3 text-center">
              <Gift aria-hidden="true" className="mx-auto mb-1 h-4 w-4 text-accent" />
              <div className="text-lg font-bold text-accent">{data.total_rewarded}</div>
              <div className="text-[11px] text-muted-foreground leading-tight">{t("statRewarded")}</div>
            </div>

            <div className="rounded-lg border border-border bg-card p-3 text-center">
              <Award aria-hidden="true" className="mx-auto mb-1 h-4 w-4 text-amber-500" />
              <div className="text-lg font-bold text-amber-500">+{data.bonus_days_earned}</div>
              <div className="text-[11px] text-muted-foreground leading-tight">{t("statEarnedDays")}</div>
            </div>
          </div>

          {/* Note */}
          <p className="text-xs text-muted-foreground/90 italic bg-accent/5 rounded-lg p-3 border border-accent/10">
            💡 {t("note")}
          </p>

          {/* Apply Referral Code Section */}
          <div className="border-t border-border pt-4">
            <h4 className="text-xs font-bold text-foreground mb-2">{t("applyTitle")}</h4>
            <form onSubmit={handleApplyCode} className="flex gap-2">
              <input
                type="text"
                value={inputCode}
                onChange={(e) => setInputCode(e.target.value.toUpperCase())}
                placeholder={t("applyPlaceholder")}
                maxLength={10}
                disabled={applying}
                className="w-full rounded-md border border-border bg-card px-3 py-2 text-sm font-mono uppercase tracking-wider text-foreground placeholder:text-muted-foreground focus:border-accent focus:outline-none"
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
