"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { Award, LayoutDashboard, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { fireExamPassSalute } from "@/lib/celebration-confetti";
import { AnimatePresence, motion } from "motion/react";

interface ExamPassCelebrationProps {
  open: boolean;
  onClose: () => void;
  onDashboard: () => void;
  score: number;
  total: number;
}

/**
 * Full-bleed win moment after a passed official exam simulation.
 * Grand Mock keeps its own certificate dialog — this is for mode=exam only.
 */
export function ExamPassCelebration({
  open,
  onClose,
  onDashboard,
  score,
  total,
}: ExamPassCelebrationProps) {
  const t = useTranslations("Session");

  useEffect(() => {
    if (!open) return;
    const stop = fireExamPassSalute();
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => {
      stop();
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [open, onClose]);

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.2 }}
          role="dialog"
          aria-modal="true"
          aria-labelledby="exam-pass-title"
          aria-describedby="exam-pass-body"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) onClose();
          }}
          className="fixed inset-0 z-[60] flex items-center justify-center bg-[#050b12]/92 p-4 backdrop-blur-md"
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.92, y: 16 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.92, y: 16 }}
            transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
            className="exam-pass-panel relative w-full max-w-lg overflow-hidden rounded-3xl border border-gold/45 bg-card px-6 py-8 text-center shadow-[0_24px_80px_-24px_rgba(0,0,0,0.75)] sm:px-10 sm:py-10"
          >
            <div
              aria-hidden
              className="pointer-events-none absolute inset-0 opacity-80"
              style={{
                background:
                  "radial-gradient(ellipse 80% 55% at 50% -10%, hsl(43 94% 52% / 0.22), transparent 60%), radial-gradient(ellipse 60% 40% at 50% 110%, hsl(152 70% 42% / 0.12), transparent 55%)",
              }}
            />

            <button
              type="button"
              onClick={onClose}
              aria-label={t("examPassCelebrationClose")}
              className="absolute right-3 top-3 z-10 flex h-10 w-10 items-center justify-center rounded-xl border border-border text-muted-foreground transition-colors hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <X className="h-5 w-5" aria-hidden="true" />
            </button>

            <div className="exam-pass-medal relative mx-auto flex h-24 w-24 items-center justify-center rounded-full border-2 border-gold/60 bg-gold/15">
              <Award className="h-12 w-12 text-gold" aria-hidden="true" />
            </div>

            <p className="exam-pass-eyebrow relative mt-5 text-xs font-bold uppercase tracking-[0.22em] text-gold">
              {t("examPassCelebrationEyebrow")}
            </p>
            <h2
              id="exam-pass-title"
              className="exam-pass-title relative mt-2 font-display text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl"
            >
              {t("examPassCelebrationTitle")}
            </h2>
            <p
              id="exam-pass-body"
              className="exam-pass-body relative mx-auto mt-3 max-w-md text-sm leading-relaxed text-muted-foreground sm:text-base"
            >
              {t("examPassCelebrationBody", { score, total })}
            </p>

            <p className="relative mt-5 font-display text-2xl font-black tabular-nums text-accent">
              {score} / {total}
            </p>

            <div className="relative mt-8 flex flex-col gap-2 sm:flex-row sm:justify-center">
              <Button variant="game" size="lg" className="w-full sm:w-auto transition-transform active:scale-95" onClick={onClose}>
                {t("examPassCelebrationContinue")}
              </Button>
              <Button
                variant="outline"
                size="lg"
                className="w-full gap-2 sm:w-auto transition-transform active:scale-95"
                onClick={onDashboard}
              >
                <LayoutDashboard className="h-4 w-4" aria-hidden="true" />
                {t("examPassCelebrationDashboard")}
              </Button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

