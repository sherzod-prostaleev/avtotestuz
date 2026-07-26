"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Lock, Trophy } from "lucide-react";
import { apiGet } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

export interface MockEligibilityResponse {
  eligible: boolean;
  mastery_percent: number;
  min_required_percent: number;
  questions_studied: number;
  min_required_questions: number;
  is_vip: boolean;
  reason: "mastery_too_low" | "too_few_studied" | "vip_required" | null;
}

/**
 * The gate the user is currently stuck behind, expressed as a single
 * progress pair so the bar always tracks the requirement being reported
 * rather than mastery alone — a user blocked on study volume would otherwise
 * see a full bar next to a "locked" message.
 */
function blockingProgress(data: MockEligibilityResponse) {
  if (data.reason === "too_few_studied") {
    return { current: data.questions_studied, min: data.min_required_questions, unit: "questions" as const };
  }
  return { current: data.mastery_percent, min: data.min_required_percent, unit: "percent" as const };
}

function progressWidth(current: number, min: number): number {
  if (min <= 0) return 100;
  return Math.min(100, Math.max(0, Math.round((current / min) * 100)));
}

/**
 * Gold-accented dashboard/practice card for Grand Mock — the gated full
 * exam simulation. Reads eligibility from GET /me/mock-eligibility (a
 * read-only UI hint; StartSession re-checks the same server-side
 * conditions, see docs/superpowers/specs/2026-07-24-m2-07-grand-mock-design.md
 * §5). The "start" button navigates to the existing
 * /session/start?mode=grand_mock flow — the same mechanism "exam" mode
 * already uses — rather than starting a session directly here.
 */
export function GrandMockCard() {
  const t = useTranslations("GrandMock");
  const locale = useLocale();

  const [data, setData] = useState<MockEligibilityResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const res = await apiGet<MockEligibilityResponse>("me/mock-eligibility");
      setData(res);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <Card className="min-w-0 overflow-hidden border-gold/40 bg-card p-3 sm:p-5 md:p-6">
      <CardHeader className="mb-2 flex flex-row items-center gap-2 p-0 sm:mb-3">
        <Trophy aria-hidden="true" className="h-4 w-4 text-gold sm:h-5 sm:w-5" />
        <CardTitle className="font-display text-sm font-extrabold tracking-wide sm:text-base">{t("title")}</CardTitle>
      </CardHeader>
      <p className="mb-3 text-[11px] text-muted-foreground sm:mb-4 sm:text-xs">{t("subtitle")}</p>

      {loading && (
        <div role="status" className="text-sm text-muted-foreground">
          {t("loading")}
        </div>
      )}

      {!loading && error && (
        <div role="alert" className="text-sm text-destructive">
          {t("loadError")}
        </div>
      )}

      {!loading && !error && data && data.eligible && (
        <Link href={`/${locale}/session/start?mode=grand_mock`} className="block">
          <Button as="span" variant="game" size="lg" className="w-full">
            {t("startButton")}
          </Button>
        </Link>
      )}

      {!loading && !error && data && !data.eligible && (
        <LockedState data={data} locale={locale} />
      )}
    </Card>
  );
}

function LockedState({ data, locale }: { data: MockEligibilityResponse; locale: string }) {
  const t = useTranslations("GrandMock");
  const { current, min, unit } = blockingProgress(data);

  const reasonText =
    data.reason === "vip_required"
      ? t("lockedVipReason")
      : data.reason === "too_few_studied"
        ? t("lockedStudiedReason", { min: data.min_required_questions, current: data.questions_studied })
        : t("lockedMasteryReason", { min: data.min_required_percent, current: data.mastery_percent });

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
        <Lock aria-hidden="true" className="h-4 w-4 shrink-0 text-gold" />
        <span>{reasonText}</span>
      </div>
      <div
        role="progressbar"
        aria-valuenow={current}
        aria-valuemin={0}
        aria-valuemax={min}
        className="h-2.5 w-full overflow-hidden rounded-full bg-background/60"
      >
        <div
          className="h-full rounded-full bg-gold transition-all"
          style={{ width: `${progressWidth(current, min)}%` }}
        />
      </div>
      <p className="text-xs font-bold text-gold">
        {unit === "questions"
          ? t("progressLabelQuestions", { current, min })
          : t("progressLabel", { current, min })}
      </p>

      {/* VIP is the one lock the user can clear right now, so it gets a real
          call to action instead of a dead-end message. StartSession answers 402
          for this case; the session-start screen routes there too. */}
      {data.reason === "vip_required" && (
        <Link href={`/${locale}/premium`} className="block">
          <Button as="span" variant="game" size="lg" className="w-full">
            {t("upgradeButton")}
          </Button>
        </Link>
      )}
    </div>
  );
}
