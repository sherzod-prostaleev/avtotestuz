"use client";

import { startTransition, useEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { LoaderCircle, X, ZoomIn } from "lucide-react";
import type { SessionQuestionItem, SessionState } from "@/hooks/use-session-engine";
import { BrandLogo } from "@/components/brand/brand-logo";
import { CountdownTimer } from "@/components/shared/countdown-timer";
import { useFitScale } from "@/hooks/use-fit-scale";
import { resolveQuestionImageUrl } from "@/lib/question-image";
import { AnimatePresence, motion } from "motion/react";


interface OfficialAvtotestExamViewProps {
  session: SessionState;
  currentIndex: number;
  onSelectIndex: (index: number) => void;
  onSelectAnswer: (questionId: string, answerId: string) => void;
  onFinish: () => void;
  submitting: boolean;
  finishing: boolean;
  /** Optimistic selection while the grade request is in flight. */
  pendingAnswer?: { questionId: string; answerId: string } | null;
  submitError?: string | null;
  onRetryAnswer?: () => void;
  /**
   * Where the close (X) button exits to. Callers on the kiosk pass
   * `/${locale}/station` instead of the learner `/${locale}/dashboard` —
   * see session/[id]/page.tsx's `exitHref`.
   */
  exitHref: string;
}

type ExamAnswerVisual = "correct" | "wrong" | "selected" | "answered" | "neutral";

/**
 * Exam-specific answer visual:
 *  - User chose this AND it was correct → "correct" (green)
 *  - User chose this AND it was wrong  → "wrong"   (red)
 *  - Optimistic choice while submit flies → "selected" (blue, brief)
 *  - Recorded but grade missing          → "answered" (muted, never blue)
 *  - Any other answer                    → "neutral"  (never reveals the key)
 */
export function examVisual(
  question: SessionQuestionItem,
  answerId: string,
  pending?: { questionId: string; answerId: string } | null
): ExamAnswerVisual {
  const pendingHere =
    Boolean(pending) && pending!.questionId === question.id && pending!.answerId === answerId;
  const isUserChoice = question.user_answer_id === answerId || pendingHere;
  const isAnswered = question.answered === true || Boolean(question.user_answer_id);

  if (pendingHere && !isAnswered) return "selected";
  if (!isAnswered || !isUserChoice) return "neutral";
  if (question.correct === true) return "correct";
  if (question.correct === false) return "wrong";
  return "answered";
}

/**
 * Official Avtotest exam simulation. Desktop look/layout is locked to the
 * authentic exam vibe. Mobile-only overrides use `max-lg:` so lg+ rendering
 * stays identical to the original two-column navy exam UI.
 */
export function OfficialAvtotestExamView({
  session,
  currentIndex,
  onSelectIndex,
  onSelectAnswer,
  onFinish,
  submitting,
  finishing,
  pendingAnswer = null,
  submitError = null,
  onRetryAnswer,
  exitHref,
}: OfficialAvtotestExamViewProps) {
  const locale = useLocale();
  const router = useRouter();
  const t = useTranslations("Session");

  const questions = session.questions ?? [];
  const currentQuestion = questions[currentIndex];
  const questionImageUrl = currentQuestion
    ? resolveQuestionImageUrl(currentQuestion.image_url)
    : null;
  const [zoomImageUrl, setZoomImageUrl] = useState<string | null>(null);
  const [exitConfirm, setExitConfirm] = useState(false);
  const activeChipRef = useRef<HTMLButtonElement | null>(null);
  const allAnswered =
    questions.length > 0 &&
    questions.every((q) => q.answered === true || Boolean(q.user_answer_id));

  const { viewportRef, contentRef, scale, contentStyle } = useFitScale([
    currentQuestion?.id,
    currentQuestion?.question,
    currentQuestion?.answers.length,
    currentIndex,
  ]);

  useEffect(() => {
    activeChipRef.current?.scrollIntoView({
      behavior: "smooth",
      inline: "center",
      block: "nearest",
    });
  }, [currentIndex]);

  const isFailed =
    session.status === "completed"
      ? session.passed === false
      : session.stopped_reason === "too_many_errors" || session.stopped_reason === "time_expired";

  const isCompleted = session.status === "completed";
  // The server sends the session's real budget (2 standard exam, 4 restore
  // exam, 1 placement); the mode-name guess is only a fallback for sessions
  // created before it did.
  const errorsAllowed = session.errors_allowed ?? (session.mode === "placement" ? 1 : 2);
  const wrongCount = questions.filter((q) => q.correct === false).length;
  const visualStatus: Record<ExamAnswerVisual, string> = {
    correct: t("answerStateCorrect"),
    wrong: t("answerStateWrong"),
    selected: t("answerStateSelected"),
    answered: t("answerStateAnswered"),
    neutral: t("answerStateNeutral"),
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      if (zoomImageUrl) {
        setZoomImageUrl(null);
        return;
      }
      if (exitConfirm) setExitConfirm(false);
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [zoomImageUrl, exitConfirm]);

  const switchLocale = (newLoc: string) => {
    if (newLoc === locale) return;
    const currentPath = window.location.pathname;
    const parts = currentPath.split("/");
    parts[1] = newLoc;
    // Soft replace — keep session painted; theme FOUC script prevents light flash.
    startTransition(() => {
      router.replace(parts.join("/"));
    });
  };

  /* ── Style Maps (ultra-crisp high contrast) ──────── */

  // Answer button container border+bg
  const btnStyles: Record<ExamAnswerVisual, string> = {
    correct:  "border-green-500 bg-[#163820] shadow-md",
    wrong:    "border-red-500 bg-[#421414] shadow-md",
    selected: "border-blue-400 bg-[#162e4a] shadow-md",
    answered: "border-[#5279a6] bg-[#1a3048] shadow-md",
    neutral:  "border-[#354f6e] bg-[#162738] hover:border-[#5279a6] hover:bg-[#1f364d] shadow-sm",
  };

  // F-key badge
  const badgeStyles: Record<ExamAnswerVisual, string> = {
    correct:  "bg-green-600 text-white font-black border-green-500",
    wrong:    "bg-red-600 text-white font-black border-red-500",
    selected: "bg-blue-600 text-white font-black border-blue-400",
    answered: "bg-[#2a4a6e] text-white font-black border-[#5279a6]",
    neutral:  "bg-[#1d334a] text-white font-black border-[#354f6e]",
  };

  return (
    <div className="exam-shell relative flex h-screen w-screen flex-col overflow-hidden bg-[#091726] text-white font-sans select-none subpixel-antialiased">
      {/* 3D cube mesh background (matching video wallpaper) */}
      <div
        className="absolute inset-0 pointer-events-none"
        style={{
          opacity: 0.12,
          backgroundImage: `
            linear-gradient(135deg, #1a3a6a 25%, transparent 25%),
            linear-gradient(225deg, #1a3a6a 25%, transparent 25%),
            linear-gradient(315deg, #1a3a6a 25%, transparent 25%),
            linear-gradient(45deg,  #1a3a6a 25%, transparent 25%)
          `,
          backgroundSize: `20px 20px`,
          backgroundPosition: `0 0, 0 10px, 10px -10px, -10px 0px`,
        }}
      />

      {/* ═══ TOP HEADER BAR ═══ */}
      <header className="relative z-10 flex h-[52px] shrink-0 items-center justify-between bg-[#081320]/95 px-5 border-b border-[#1c3554] max-lg:h-10 max-lg:gap-1.5 max-lg:px-2 max-lg:pt-[max(0.2rem,env(safe-area-inset-top))]">
        {/* Left: Driver Go logo */}
        <div className="flex items-center gap-3 max-lg:gap-1.5">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-black border border-emerald-500/50 shadow-[0_0_15px_rgba(34,197,94,0.4)] overflow-hidden max-lg:h-7 max-lg:w-7">
            <BrandLogo alt="Driver Go Logo" size={40} className="h-full w-full object-cover scale-110" />
          </div>
          <span className="font-black tracking-wider text-2xl text-white max-lg:hidden" style={{ fontFamily: "'Arial Black', 'Impact', sans-serif" }}>
            Driver <span className="text-[#22c55e]">Go</span>
          </span>
        </div>

        {/* Center: Language tabs — scroll on narrow screens; desktop unchanged */}
        <div className="flex items-center gap-2 max-lg:max-w-[62vw] max-lg:gap-1 max-lg:overflow-x-auto max-lg:scrollbar-none">
          {[
            { id: "uz-Latn", label: "Uzb (lotin.)", short: "Lotin" },
            { id: "uz-Cyrl", label: "Uzb (кирил.)", short: "Кирил" },
            { id: "ru", label: "Rus (кирил.)", short: "Рус" },
          ].map((lang) => {
            const isActive =
              (lang.id === "uz-Latn" && locale === "uz-Latn") ||
              (lang.id === "uz-Cyrl" && locale === "uz-Cyrl") ||
              (lang.id === "ru" && locale === "ru");
            return (
              <button
                key={lang.id}
                type="button"
                onClick={() => switchLocale(lang.id)}
                className={`relative shrink-0 px-3.5 py-1 text-xs font-bold rounded-sm border transition-all max-lg:h-8 max-lg:px-2 max-lg:py-0 max-lg:text-[11px] ${
                  isActive
                    ? "bg-[#183654] text-white border-[#5a8aaa] shadow-sm"
                    : "bg-[#0f2236] text-slate-200 border-[#284260] hover:bg-[#183654] hover:text-white"
                }`}
              >
                {isActive && (
                  <span
                    aria-hidden="true"
                    className="nav-pill-in absolute top-0 left-0 w-full h-[3px] bg-[#22c55e]"
                  />
                )}
                <span className="relative z-10 max-lg:hidden">{lang.label}</span>
                <span className="relative z-10 hidden max-lg:inline">{lang.short}</span>
              </button>
            );
          })}
        </div>


        {/* Right: X close */}
        <button
          type="button"
          onClick={() => (isCompleted ? router.push(exitHref) : setExitConfirm(true))}
          aria-label={t("exit")}
          className="text-slate-300 hover:text-white transition-colors p-1 max-lg:flex max-lg:h-8 max-lg:w-8 max-lg:items-center max-lg:justify-center"
        >
          <X className="w-6 h-6 max-lg:h-5 max-lg:w-5" />
        </button>
      </header>

      {/* ═══ QUESTION TEXT BANNER / RESULT BANNER ═══ */}
      <div className="relative z-10 w-full shrink-0">
        {isFailed ? (
          <motion.div
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
            className="w-full bg-[#dc2626] text-white text-center py-3 text-xl font-extrabold tracking-wide shadow-md max-lg:py-1.5 max-lg:text-sm"
          >
            {t("examBannerFailed")}
          </motion.div>
        ) : isCompleted ? (
          <motion.div
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
            className="w-full bg-[#16a34a] text-white text-center py-3 text-xl font-extrabold tracking-wide shadow-md max-lg:py-1.5 max-lg:text-sm"
          >
            {t("examBannerPassed")}
          </motion.div>
        ) : (
          <div className="exam-question-banner w-full border-y border-[#204a75] bg-[#0d2e4d] px-8 py-3.5 text-center text-lg font-extrabold leading-relaxed tracking-wide text-white shadow-md max-lg:px-2.5 max-lg:py-1.5 max-lg:text-[13px] max-lg:leading-snug">
            {currentQuestion ? currentQuestion.question : "Yuklanmoqda..."}
          </div>
        )}
      </div>


      {/* ═══ MAIN 2-COLUMN LAYOUT (desktop) / STACKED fit-to-viewport (mobile) ═══ */}
      <main className="relative z-10 flex min-h-0 w-full flex-1 flex-col overflow-hidden px-5 py-4 max-lg:px-2 max-lg:py-1.5">
        <div
          ref={viewportRef}
          data-fit-scale={scale.toFixed(3)}
          className="min-h-0 w-full flex-1 overflow-hidden"
        >
          <div
            ref={contentRef}
            style={contentStyle}
            className="flex h-full min-h-0 w-full items-stretch gap-5 max-lg:h-auto max-lg:flex-col max-lg:gap-1.5"
          >
            {/* LEFT: Answer options — on mobile rendered after image; never scroll/crush */}
            <div className="flex w-[38%] flex-col gap-3 justify-start pt-2 max-lg:order-2 max-lg:w-full max-lg:gap-1 max-lg:pt-0">
              {currentQuestion?.answers.map((answer, index) => {
                const shortcutLabel = `F${index + 1}`;
                const visual = examVisual(currentQuestion, answer.id, pendingAnswer);
                const isAnswered = currentQuestion.answered || Boolean(currentQuestion.user_answer_id);

                return (
                  <button
                    key={answer.id}
                    type="button"
                    disabled={isAnswered || submitting || finishing || isCompleted}
                    onClick={() => onSelectAnswer(currentQuestion.id, answer.id)}
                    aria-label={`${shortcutLabel}. ${answer.text}. ${visualStatus[visual]}`}
                    className={`group flex h-auto w-full shrink-0 items-stretch rounded-sm border transition-all text-left text-base font-semibold max-lg:min-h-8 ${btnStyles[visual]}`}
                  >
                    {/* F-key badge */}
                    <div
                      className={`flex w-12 shrink-0 items-center justify-center font-black text-base border-r transition-colors max-lg:w-8 max-lg:text-[11px] ${badgeStyles[visual]}`}
                    >
                      {shortcutLabel}
                    </div>
                    {/* Text — wrap inside the button; never crush height (overflow overlap bug). */}
                    <div className="exam-answer-text flex min-w-0 flex-1 items-center break-words whitespace-normal px-4 py-3 leading-normal text-white font-medium [overflow-wrap:anywhere] max-lg:px-2 max-lg:py-1.5 max-lg:text-[13px] max-lg:leading-snug">
                      {answer.text}
                    </div>
                  </button>
                );
              })}
            </div>

            {/* RIGHT: Question image — real media when present, Driver Go cars placeholder otherwise */}
            {currentQuestion && questionImageUrl ? (
              <div className="exam-question-image flex min-h-0 flex-1 items-center justify-center max-lg:order-1 max-lg:w-full max-lg:shrink-0 max-lg:flex-none lg:h-auto lg:max-h-none">
                <div
                  className="relative flex h-full w-full cursor-pointer items-center justify-center overflow-hidden rounded-sm border-2 border-slate-300 bg-black shadow-xl"
                  onClick={() => setZoomImageUrl(questionImageUrl)}
                >
                  {/* Sizing: globals .exam-question-image img — max-height:inherit + object-contain (never crop). */}
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    key={currentQuestion.id}
                    src={questionImageUrl}
                    alt={currentQuestion.question}
                    decoding="async"
                    className="object-contain lg:h-full lg:w-full lg:max-h-full"
                  />
                  <div className="absolute top-2 right-2 rounded bg-black/60 p-1.5 text-white/70 transition-opacity hover:text-white max-lg:top-1 max-lg:right-1 max-lg:flex max-lg:h-8 max-lg:w-8 max-lg:items-center max-lg:justify-center max-lg:p-0">
                    <ZoomIn className="h-5 w-5 max-lg:h-3.5 max-lg:w-3.5" />
                  </div>
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </main>

      {/* ═══ BOTTOM BAR ═══ */}
      <footer className="relative z-10 flex shrink-0 flex-col gap-1.5 border-t border-[#1c3554] bg-[#081320]/95 px-2 py-1.5 pb-[max(0.35rem,env(safe-area-inset-bottom))] lg:min-h-[64px] lg:flex-row lg:items-center lg:justify-between lg:px-5 lg:py-1">
        {/* Mobile top sub-row / Desktop Left + Right HUD */}
        <div className="flex w-full items-center justify-between gap-2 lg:contents">
          {/* Left: Finish button */}
          <div className="flex shrink-0 items-center lg:w-28">
            {allAnswered && !isCompleted ? (
              <button
                type="button"
                onClick={onFinish}
                disabled={finishing || submitting}
                className="rounded-sm border border-emerald-400/70 bg-emerald-600 px-2.5 py-1 text-xs font-extrabold text-white shadow-md transition-colors hover:bg-emerald-500 disabled:opacity-60 lg:h-9 lg:px-3 lg:text-sm"
              >
                {finishing ? (
                  <span className="inline-flex items-center gap-1.5">
                    <LoaderCircle className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                    {t("finishing")}
                  </span>
                ) : (
                  t("finish")
                )}
              </button>
            ) : (
              <div className="hidden lg:block lg:w-20" />
            )}
          </div>

          {/* Right: Timer + live error budget */}
          <div className="flex shrink-0 items-center gap-1.5 lg:order-last lg:gap-2">
            {!isCompleted && (
              <div
                role="status"
                aria-live="polite"
                className="flex h-7 items-center border border-[#2a4568] bg-[#050b12] px-2 py-0.5 rounded-sm font-mono text-[11px] font-bold tabular-nums text-white lg:h-8 lg:text-xs"
              >
                {t("examErrorsHud", { wrong: wrongCount, max: errorsAllowed })}
              </div>
            )}
            {session.remaining_sec !== null && (
              <div className="flex h-7 items-center bg-[#050b12] border border-[#2a4568] px-2.5 py-0.5 rounded-sm font-mono text-xs font-bold tracking-wider text-[#fbbf24] shadow-inner lg:h-8 lg:px-4 lg:text-base">
                <CountdownTimer seconds={session.remaining_sec} onExpire={onFinish} />
              </div>
            )}
          </div>
        </div>

        {/* Center: Question number pills
            Mobile: 10-column solid grid (20 questions = 2 rows of 10, 50 questions = 5 rows of 10) -> 0 scroll, 0 dangling items.
            Desktop: 20 in 1 row, 50 in 2 rows of 25 (1..25 top, 26..50 bottom). */}
        <div className="flex min-w-0 flex-1 items-center justify-center px-1 py-0.5 lg:px-3">
          {(() => {
            const renderChip = (q: SessionQuestionItem, idx: number) => {
              const isCurrent = idx === currentIndex;
              const isAns = q.answered || Boolean(q.user_answer_id);
              const isCorr = q.correct === true;
              const isWrn = q.correct === false;

              let bg: string;
              if (isWrn) bg = "bg-[#dc2626] text-white border border-red-400 shadow-md font-extrabold";
              else if (isCorr) bg = "bg-[#16a34a] text-white border border-green-400 shadow-md font-extrabold";
              else if (isAns) bg = "bg-[#334155] text-white border border-slate-400 shadow-md font-extrabold";
              else bg = "bg-[#1c334d] text-white border border-[#304d70] hover:bg-[#28486e] hover:border-white font-extrabold";

              return (
                <button
                  key={q.id}
                  ref={isCurrent ? activeChipRef : undefined}
                  type="button"
                  onClick={() => onSelectIndex(idx)}
                  aria-label={t("questionNavLabel", {
                    number: idx + 1,
                    status: isWrn
                      ? t("statusWrong")
                      : isCorr
                        ? t("statusCorrect")
                        : isAns
                          ? t("statusAnswered")
                          : isCurrent
                            ? t("statusCurrent")
                            : t("statusUnanswered"),
                  })}
                  className={`flex h-6 w-full max-w-[28px] sm:h-7 sm:max-w-[32px] lg:h-8 lg:w-8 shrink-0 items-center justify-center rounded-sm text-[10px] sm:text-xs lg:text-sm font-extrabold transition-all active:scale-95 ${
                    isCurrent ? "ring-2 ring-white ring-offset-1 ring-offset-[#081320] scale-105 font-black z-10 shadow-lg" : ""
                  } ${bg}`}
                >
                  {idx + 1}
                </button>
              );
            };

            return (
              <>
                {/* Mobile view (< lg): 10 columns grid without horizontal scroll */}
                <div className="grid w-full max-w-[340px] grid-cols-10 gap-1 justify-items-center lg:hidden">
                  {questions.map((q, idx) => renderChip(q, idx))}
                </div>

                {/* Desktop view (lg+): 20 in 1 row or 50 in 2 rows of 25 */}
                <div className="hidden lg:flex lg:items-center lg:justify-center">
                  {questions.length > 25 ? (
                    <div className="flex flex-col items-center justify-center gap-1">
                      <div className="flex flex-nowrap items-center justify-center gap-1.5">
                        {questions.slice(0, Math.ceil(questions.length / 2)).map((q, idx) => renderChip(q, idx))}
                      </div>
                      <div className="flex flex-nowrap items-center justify-center gap-1.5">
                        {questions.slice(Math.ceil(questions.length / 2)).map((q, idx) =>
                          renderChip(q, Math.ceil(questions.length / 2) + idx)
                        )}
                      </div>
                    </div>
                  ) : (
                    <div className="flex flex-nowrap items-center justify-center gap-1.5">
                      {questions.map((q, idx) => renderChip(q, idx))}
                    </div>
                  )}
                </div>
              </>
            );
          })()}
        </div>
      </footer>

      {submitError && !isCompleted && (
        <div
          role="alert"
          className="relative z-20 flex shrink-0 items-center justify-between gap-3 border-t border-red-400/40 bg-[#421414] px-4 py-2 text-sm text-white max-lg:px-2"
        >
          <span>{submitError}</span>
          {onRetryAnswer && (
            <button
              type="button"
              onClick={onRetryAnswer}
              className="shrink-0 rounded-sm border border-white/40 px-3 py-1 text-xs font-extrabold"
            >
              {t("retryAnswer")}
            </button>
          )}
        </div>
      )}

      <AnimatePresence>
        {exitConfirm && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
            role="dialog"
            aria-modal="true"
            aria-labelledby="exam-exit-title"
          >
            <motion.div
              initial={{ opacity: 0, scale: 0.94, y: 8 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.94, y: 8 }}
              transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
              className="w-full max-w-sm"
            >
              <div className="w-full rounded-lg border border-[#2a4568] bg-[#0d2e4d] p-5 text-white shadow-xl">
                <p id="exam-exit-title" className="text-sm font-semibold leading-relaxed">
                  {t("examExitConfirm")}
                </p>
                <div className="mt-4 flex gap-2">
                  <button
                    type="button"
                    onClick={() => setExitConfirm(false)}
                    className="flex-1 rounded-sm border border-[#5a8aaa] bg-[#183654] px-3 py-2 text-sm font-extrabold"
                  >
                    {t("examExitStay")}
                  </button>
                  <button
                    type="button"
                    onClick={() => router.push(exitHref)}
                    className="flex-1 rounded-sm border border-red-400/50 bg-[#421414] px-3 py-2 text-sm font-extrabold"
                  >
                    {t("examExitLeave")}
                  </button>
                </div>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* ZOOM MODAL */}
      <AnimatePresence>
        {zoomImageUrl && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 p-4"
            onClick={() => setZoomImageUrl(null)}
          >
            <motion.div
              initial={{ opacity: 0, scale: 0.92 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.92 }}
              transition={{ duration: 0.2 }}
              className="relative max-h-[95vh] max-w-[95vw]"
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={zoomImageUrl} alt="Zoom" className="max-h-[90vh] max-w-[90vw] object-contain border border-white" />
              <button
                type="button"
                onClick={() => setZoomImageUrl(null)}
                className="absolute -top-10 right-0 text-white hover:text-slate-300 max-lg:top-2 max-lg:right-2 max-lg:flex max-lg:h-11 max-lg:w-11 max-lg:items-center max-lg:justify-center max-lg:rounded-full max-lg:bg-black/70"
              >
                <X className="w-8 h-8 max-lg:w-6 max-lg:h-6" />
              </button>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

