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
  is_vip: boolean;
  reason: "mastery_too_low" | "vip_required" | null;
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
    <Card className="overflow-hidden border-gold/40 bg-gradient-to-br from-card to-gold/10 p-6">
      <CardHeader className="mb-3 flex flex-row items-center gap-2 p-0">
        <Trophy aria-hidden="true" className="h-5 w-5 text-gold" />
        <CardTitle className="font-display text-base font-extrabold tracking-wide">{t("title")}</CardTitle>
      </CardHeader>
      <p className="mb-4 text-xs text-muted-foreground">{t("subtitle")}</p>

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
          <Button variant="game" size="lg" className="w-full">
            {t("startButton")}
          </Button>
        </Link>
      )}

      {!loading && !error && data && !data.eligible && (
        <div className="space-y-3">
          <div className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
            <Lock aria-hidden="true" className="h-4 w-4 shrink-0 text-gold" />
            <span>
              {data.reason === "vip_required"
                ? t("lockedVipReason")
                : t("lockedMasteryReason", {
                    min: data.min_required_percent,
                    current: data.mastery_percent,
                  })}
            </span>
          </div>
          <div
            role="progressbar"
            aria-valuenow={data.mastery_percent}
            aria-valuemin={0}
            aria-valuemax={data.min_required_percent}
            className="h-2.5 w-full overflow-hidden rounded-full bg-background/60"
          >
            <div
              className="h-full rounded-full bg-gold transition-all"
              style={{
                width: `${Math.min(100, Math.round((data.mastery_percent / data.min_required_percent) * 100))}%`,
              }}
            />
          </div>
          <p className="text-xs font-bold text-gold">
            {t("progressLabel", { current: data.mastery_percent, min: data.min_required_percent })}
          </p>
        </div>
      )}
    </Card>
  );
}
