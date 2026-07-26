"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { Sparkles, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { fireBiletPraiseBurst } from "@/lib/celebration-confetti";

interface BiletPraiseBannerProps {
  open: boolean;
  onClose: () => void;
  score: number;
  total: number;
}

/**
 * Lighter praise when a practice bilet finishes at exam-pass strength (≥18/20).
 * Intentionally smaller than ExamPassCelebration — no full salute theater.
 */
export function BiletPraiseBanner({ open, onClose, score, total }: BiletPraiseBannerProps) {
  const t = useTranslations("Session");

  useEffect(() => {
    if (!open) return;
    fireBiletPraiseBurst();
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      aria-labelledby="bilet-praise-title"
      className="bilet-praise-banner fixed inset-x-0 top-0 z-[55] flex justify-center px-3 pt-[max(0.75rem,env(safe-area-inset-top))]"
    >
      <div className="relative flex w-full max-w-xl items-start gap-3 rounded-2xl border border-gold/40 bg-card/95 px-4 py-3 shadow-[0_12px_40px_-16px_rgba(0,0,0,0.55)] backdrop-blur-md sm:px-5 sm:py-4">
        <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-gold/40 bg-gold/15">
          <Sparkles className="h-5 w-5 text-gold" aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1 pt-0.5">
          <h2 id="bilet-praise-title" className="font-display text-lg font-extrabold tracking-tight">
            {t("biletPraiseTitle")}
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("biletPraiseBody", { score, total })}
          </p>
          <Button type="button" variant="outline" size="sm" className="mt-3" onClick={onClose}>
            {t("biletPraiseClose")}
          </Button>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label={t("biletPraiseClose")}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-border text-muted-foreground transition-colors hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
    </div>
  );
}
