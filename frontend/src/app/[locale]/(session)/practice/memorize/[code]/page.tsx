"use client";

import { useEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useParams, useRouter } from "next/navigation";
import { ChevronLeft, ChevronRight, LoaderCircle, X } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useMemorize } from "@/hooks/use-memorize";
import { ExplanationDialog } from "@/components/shared/explanation-dialog";
import { QuestionStage } from "@/components/shared/question-stage";
import { resolveQuestionImageUrl } from "@/lib/question-image";
import { readSessionOrigin } from "@/lib/session-origin";

export interface MemorizePageProps {
  // Reused as-is under the login-free kiosk
  // (frontend/src/app/[locale]/(kiosk)/station/practice/memorize/[code]/page.tsx):
  // a licensed classroom station is treated as VIP by the same server check
  // a personal subscription uses (billing.StationVIPChecker), so this screen
  // behaves identically there — only the exit and vip_required destinations
  // differ (never a premium checkout link on a kiosk).
  kiosk?: boolean;
}

export default function MemorizePage({ kiosk = false }: MemorizePageProps = {}) {
  const params = useParams();
  const router = useRouter();
  const locale = useLocale();
  const t = useTranslations("Memorize");
  const sessionT = useTranslations("Session");
  const startT = useTranslations("SessionStart");
  const practiceT = useTranslations("Practice");
  const code = typeof params.code === "string" ? params.code : "";

  const { questions, loading, error } = useMemorize(code, locale);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [zoomImageUrl, setZoomImageUrl] = useState<string | null>(null);
  const [explanationOpen, setExplanationOpen] = useState(false);
  const activeChipRef = useRef<HTMLButtonElement | null>(null);

  const practiceHref = `/${locale}/${kiosk ? "station/practice" : "practice"}`;
  // Yodlash is opened from a topic card, but a learner can also reach the topic
  // list from several places — go back where they actually came from, and fall
  // back to the topic list when the tab has no record of it.
  const [backHref, setBackHref] = useState(practiceHref);
  useEffect(() => {
    const origin = readSessionOrigin();
    if (origin) setBackHref(origin);
  }, []);

  // Keep the active chip in view while advancing through a long topic on
  // mobile — same behaviour the live session runner has.
  useEffect(() => {
    activeChipRef.current?.scrollIntoView({
      behavior: "smooth",
      inline: "center",
      block: "nearest",
    });
  }, [currentIndex]);

  const goTo = (index: number) => {
    setExplanationOpen(false);
    setCurrentIndex(index);
  };
  const goPrev = () => goTo(Math.max(0, currentIndex - 1));
  const goNext = () => goTo(Math.min(questions.length, currentIndex + 1));

  if (error) {
    let destination = practiceHref;
    let actionLabel = startT("backToPractice");
    let message = error.code === "network_error" ? sessionT("networkError") : sessionT("genericError");

    if (error.code === "vip_required") {
      if (kiosk) {
        destination = `/${locale}/station`;
        actionLabel = startT("backToStation");
      } else {
        destination = `/${locale}/premium`;
        actionLabel = startT("goToPremium");
      }
      message = startT("vipRequired");
    }

    return (
      <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
        <Card className="w-full max-w-md border-destructive/40 bg-destructive/5 p-6 text-center">
          <p className="font-display text-lg font-bold text-destructive">{startT("errorTitle")}</p>
          <p className="mt-2 text-sm text-muted-foreground">{message}</p>
          <div className="sticky-cta-bar mt-5">
            <Button variant="game" className="w-full" onClick={() => router.push(destination)}>
              {actionLabel}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  if (loading) {
    return (
      <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
        <Card className="flex items-center justify-center gap-2 p-8 text-muted-foreground">
          <LoaderCircle className="h-5 w-5 animate-spin text-accent" aria-hidden="true" />
          <span className="text-sm">{t("loading")}</span>
        </Card>
      </main>
    );
  }

  if (questions.length === 0) {
    return (
      <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
        <Card className="w-full max-w-md p-6 text-center">
          <p className="font-display text-lg font-bold">{t("emptyTitle")}</p>
          <p className="mt-2 text-sm text-muted-foreground">{t("emptyBody")}</p>
          <div className="sticky-cta-bar mt-5">
            <Button variant="game" className="w-full" onClick={() => router.push(practiceHref)}>
              {startT("backToPractice")}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  const isFinished = currentIndex >= questions.length;

  if (isFinished) {
    return (
      <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
        <Card className="w-full max-w-md p-6 text-center">
          <p className="font-display text-lg font-bold">{t("finishedTitle")}</p>
          <p className="mt-2 text-sm text-muted-foreground">{t("finishedBody")}</p>
          <div className="sticky-cta-bar mt-5">
            <Button variant="game" className="w-full" onClick={() => router.push(practiceHref)}>
              {startT("backToPractice")}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  const currentQuestion = questions[currentIndex];

  return (
    <main className="page-enter-fade session-shell flex flex-col gap-1 overflow-hidden bg-background px-2 pb-[max(0.35rem,env(safe-area-inset-bottom))] pt-[max(0.35rem,env(safe-area-inset-top))] sm:gap-3 sm:px-4 sm:py-3">
      <header className="session-header flex shrink-0 items-center justify-between gap-1.5 rounded-xl border border-border bg-card px-2 py-1.5 sm:gap-3 sm:rounded-2xl sm:p-3">
        <Button
          variant="outline"
          size="sm"
          className="h-9 min-h-9 gap-1 rounded-lg border-border px-2.5 text-xs font-extrabold transition-transform active:scale-95 sm:h-11 sm:min-h-11 sm:rounded-xl sm:px-4 sm:text-sm"
          aria-label={sessionT("exit")}
          onClick={() => router.push(backHref)}
        >
          <ChevronLeft className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span>{sessionT("exit")}</span>
        </Button>
        <span className="truncate rounded-lg border border-accent/30 bg-accent/10 px-2 py-1 text-[11px] font-bold text-accent sm:px-3 sm:py-1.5 sm:text-xs">
          {practiceT("memorizeButton")}
        </span>
      </header>

      <Card className="session-content-card flex min-h-0 flex-1 flex-col gap-1 overflow-hidden p-1.5 sm:gap-3 sm:p-5">
        <div className="min-h-0 flex-1 overflow-hidden">
          <QuestionStage
            question={currentQuestion}
            questionNumber={currentIndex + 1}
            totalQuestions={questions.length}
            answered={true}
            disabled={true}
            onSelectAnswer={() => {}}
            answerStateFor={(answerId) =>
              currentQuestion.correct_answer_id === answerId ? "correct" : "neutral"
            }
            onZoomImage={() => setZoomImageUrl(resolveQuestionImageUrl(currentQuestion.image_url))}
            onOpenExplanation={() => setExplanationOpen(true)}
          />
        </div>
      </Card>

      <footer className="session-actions flex shrink-0 flex-col gap-2 rounded-xl border border-border bg-card p-2 sm:rounded-2xl sm:p-2.5 shadow-raised-sm">
        <nav
          className="session-navigator flex flex-wrap items-center justify-center gap-1 sm:gap-1.5 max-h-24 sm:max-h-36 overflow-y-auto px-1 py-0.5"
          aria-label={sessionT("questionNavigator")}
        >
          {questions.map((question, index) => {
            const isCurrent = index === currentIndex;
            return (
              <button
                key={question.id}
                ref={isCurrent ? activeChipRef : undefined}
                type="button"
                onClick={() => goTo(index)}
                aria-current={isCurrent ? "step" : undefined}
                aria-label={sessionT("questionNavLabel", {
                  number: index + 1,
                  status: isCurrent ? sessionT("statusCurrent") : sessionT("statusCorrect"),
                })}
                className={`relative flex h-7 w-7 sm:h-8 sm:w-8 md:h-9 md:w-9 shrink-0 items-center justify-center rounded-lg border text-[11px] sm:text-xs md:text-sm tabular-nums transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:scale-95 ${
                  isCurrent
                    ? "border-accent bg-accent text-accent-foreground ring-2 ring-accent/30 font-black scale-105 shadow-md"
                    : "border-border bg-background text-muted-foreground hover:border-accent/50 hover:text-foreground font-bold"
                }`}
              >
                {index + 1}
              </button>
            );
          })}
        </nav>

        <div className="flex items-center justify-between gap-2 border-t border-border/60 pt-1.5">
          <Button
            variant="outline"
            className="h-9 min-h-9 px-3 sm:h-11 sm:min-h-11 sm:px-5"
            disabled={currentIndex === 0}
            onClick={goPrev}
          >
            <ChevronLeft className="mr-1 h-4 w-4" aria-hidden="true" />
            <span className="hidden xs:inline sm:inline">{sessionT("previous")}</span>
          </Button>

          <div className="flex items-center gap-2 text-xs font-bold text-muted-foreground sm:text-sm">
            <span className="tabular-nums font-extrabold text-foreground">
              {currentIndex + 1} / {questions.length}
            </span>
          </div>

          <Button
            variant="game"
            className="h-9 min-h-9 px-4 sm:h-11 sm:min-h-11 sm:px-6"
            onClick={goNext}
          >
            <span>{sessionT("next")}</span>
            <ChevronRight className="ml-1 h-4 w-4" aria-hidden="true" />
          </Button>
        </div>
      </footer>

      <ExplanationDialog
        open={explanationOpen}
        onClose={() => setExplanationOpen(false)}
        questionNumber={currentIndex + 1}
        questionText={currentQuestion.question}
        imageUrl={resolveQuestionImageUrl(currentQuestion.image_url)}
        explanation={currentQuestion.explanation ?? null}
      />

      <AnimatePresence>
        {zoomImageUrl && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            role="dialog"
            aria-modal="true"
            aria-label={sessionT("zoomDialog")}
            onMouseDown={(event) => {
              if (event.target === event.currentTarget) setZoomImageUrl(null);
            }}
            className="fixed inset-0 z-50 flex items-end justify-center bg-black/85 p-0 backdrop-blur-sm sm:items-center sm:p-4"
          >
            <motion.div
              initial={{ opacity: 0, scale: 0.92 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.92 }}
              transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
              className="session-zoom-panel relative w-full max-w-5xl rounded-t-3xl bg-card p-3 sm:rounded-2xl sm:bg-transparent sm:p-0"
            >
              {/* Dynamic media URL is served by the backend. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={zoomImageUrl}
                alt={sessionT("zoomedImageAlt")}
                className="session-zoom-image w-full rounded-2xl object-contain"
              />
              <button
                type="button"
                onClick={() => setZoomImageUrl(null)}
                aria-label={sessionT("closeZoom")}
                className="absolute right-3 top-3 flex h-11 w-11 items-center justify-center rounded-full border border-border bg-card text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:right-2 sm:top-2 sm:border-0 sm:bg-foreground/90 sm:text-background"
              >
                <X className="h-5 w-5" aria-hidden="true" />
              </button>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </main>
  );
}
