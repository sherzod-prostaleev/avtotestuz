"use client";

import { startTransition, useEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { LoaderCircle, X, ZoomIn } from "lucide-react";
import type { SessionQuestionItem, SessionState } from "@/hooks/use-session-engine";
import { BrandLogo } from "@/components/brand/brand-logo";
import { CountdownTimer } from "@/components/shared/countdown-timer";
import { resolveQuestionImageUrl } from "@/lib/question-image";

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
  const activeChipRef = useRef<HTMLButtonElement | null>(null);
  const allAnswered =
    questions.length > 0 &&
    questions.every((q) => q.answered === true || Boolean(q.user_answer_id));

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

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && zoomImageUrl) setZoomImageUrl(null);
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [zoomImageUrl]);

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
                {isActive && <span className="absolute top-0 left-0 w-full h-[3px] bg-[#22c55e]" />}
                <span className="max-lg:hidden">{lang.label}</span>
                <span className="hidden max-lg:inline">{lang.short}</span>
              </button>
            );
          })}
        </div>

        {/* Right: X close */}
        <button
          type="button"
          onClick={() => router.push(`/${locale}/dashboard`)}
          aria-label="Close"
          className="text-slate-300 hover:text-white transition-colors p-1 max-lg:flex max-lg:h-8 max-lg:w-8 max-lg:items-center max-lg:justify-center"
        >
          <X className="w-6 h-6 max-lg:h-5 max-lg:w-5" />
        </button>
      </header>

      {/* ═══ QUESTION TEXT BANNER / RESULT BANNER ═══ */}
      <div className="relative z-10 w-full shrink-0">
        {isFailed ? (
          <div className="w-full bg-[#dc2626] text-white text-center py-3 text-xl font-extrabold tracking-wide shadow-md max-lg:py-1.5 max-lg:text-sm">
            {t("examBannerFailed")}
          </div>
        ) : isCompleted ? (
          <div className="w-full bg-[#16a34a] text-white text-center py-3 text-xl font-extrabold tracking-wide shadow-md max-lg:py-1.5 max-lg:text-sm">
            {t("examBannerPassed")}
          </div>
        ) : (
          <div className="w-full bg-[#0d2e4d] border-y border-[#204a75] text-white text-center py-3.5 px-8 text-lg font-extrabold leading-relaxed tracking-wide shadow-md max-lg:px-2.5 max-lg:py-1.5 max-lg:text-[13px] max-lg:leading-snug">
            {currentQuestion ? currentQuestion.question : "Yuklanmoqda..."}
          </div>
        )}
      </div>

      {/* ═══ MAIN 2-COLUMN LAYOUT (desktop) / STACKED (mobile only) ═══ */}
      <main className="relative z-10 flex flex-1 min-h-0 w-full px-5 py-4 gap-5 items-stretch max-lg:flex-col max-lg:gap-1.5 max-lg:overflow-hidden max-lg:px-2 max-lg:py-1.5">
        {/* LEFT: Answer options — on mobile rendered after image */}
        <div className="flex w-[38%] flex-col gap-3 justify-start pt-2 max-lg:order-2 max-lg:min-h-0 max-lg:w-full max-lg:flex-1 max-lg:gap-1.5 max-lg:overflow-y-auto max-lg:overscroll-contain max-lg:pt-0">
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
                className={`group flex h-auto w-full shrink-0 items-stretch rounded-sm border transition-all text-left text-base font-semibold max-lg:min-h-9 ${btnStyles[visual]}`}
              >
                {/* F-key badge */}
                <div
                  className={`flex w-12 shrink-0 items-center justify-center font-black text-base border-r transition-colors max-lg:w-9 max-lg:text-xs ${badgeStyles[visual]}`}
                >
                  {shortcutLabel}
                </div>
                {/* Text — wrap inside the button; never crush height (overflow overlap bug). */}
                <div className="flex min-w-0 flex-1 items-center break-words whitespace-normal px-4 py-3 leading-normal text-white font-medium [overflow-wrap:anywhere] max-lg:px-2.5 max-lg:py-2 max-lg:text-[13px] max-lg:leading-snug">
                  {answer.text}
                </div>
              </button>
            );
          })}
        </div>

        {/* RIGHT: Question image — real media when present, Driver Go cars placeholder otherwise */}
        {currentQuestion && questionImageUrl ? (
          <div className="exam-question-image flex flex-1 items-center justify-center min-h-0 max-lg:order-1 max-lg:min-h-0 max-lg:shrink-0 max-lg:flex-none lg:h-auto lg:max-h-none">
            <div
              className="relative flex h-full w-full items-center justify-center border-2 border-slate-300 bg-black overflow-hidden cursor-pointer shadow-xl rounded-sm"
              onClick={() => setZoomImageUrl(questionImageUrl)}
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                key={currentQuestion.id}
                src={questionImageUrl}
                alt={currentQuestion.question}
                decoding="async"
                className="h-full w-full object-contain"
              />
              <div className="absolute top-2 right-2 rounded bg-black/60 p-1.5 text-white/70 hover:text-white transition-opacity max-lg:top-1 max-lg:right-1 max-lg:flex max-lg:h-8 max-lg:w-8 max-lg:items-center max-lg:justify-center max-lg:p-0">
                <ZoomIn className="w-5 h-5 max-lg:h-3.5 max-lg:w-3.5" />
              </div>
            </div>
          </div>
        ) : null}
      </main>

      {/* ═══ BOTTOM BAR ═══ */}
      <footer className="relative z-10 flex h-[60px] shrink-0 items-center justify-between bg-[#081320]/95 px-5 border-t border-[#1c3554] max-lg:h-auto max-lg:flex-row max-lg:items-center max-lg:gap-2 max-lg:px-2 max-lg:py-1.5 max-lg:pb-[max(0.35rem,env(safe-area-inset-bottom))]">
        <div className="flex w-28 shrink-0 items-center max-lg:w-auto">
          {allAnswered && !isCompleted ? (
            <button
              type="button"
              onClick={onFinish}
              disabled={finishing || submitting}
              className="rounded-sm border border-emerald-400/70 bg-emerald-600 px-3 py-1.5 text-sm font-extrabold text-white shadow-md transition-colors hover:bg-emerald-500 disabled:opacity-60 max-lg:h-8 max-lg:px-2.5 max-lg:text-xs"
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
            <div className="w-20 max-lg:hidden" />
          )}
        </div>

        {/* Question number pills */}
        <div className="flex items-center gap-1.5 max-lg:min-w-0 max-lg:flex-1 max-lg:gap-1 max-lg:overflow-x-auto max-lg:scrollbar-none">
          {questions.map((q, idx) => {
            const isCurrent = idx === currentIndex;
            const isAns = q.answered || Boolean(q.user_answer_id);
            const isCorr = q.correct === true;
            const isWrn = q.correct === false;

            // Answered+correct → green; answered+wrong → red; answered without
            // grade must NOT stay blue (that confused learners with "correct").
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
                className={`w-8 h-8 flex items-center justify-center text-base rounded-sm transition-all max-lg:h-8 max-lg:w-8 max-lg:shrink-0 max-lg:text-sm ${
                  isCurrent ? "ring-2 ring-white ring-offset-2 ring-offset-[#081320] scale-110 font-black z-10 shadow-lg max-lg:ring-offset-1 max-lg:scale-105" : ""
                } ${bg}`}
              >
                {idx + 1}
              </button>
            );
          })}
        </div>

        {/* Timer */}
        <div className="flex items-center max-lg:shrink-0">
          {session.remaining_sec !== null && (
            <div className="bg-[#050b12] border border-[#2a4568] px-4 py-1.5 rounded-sm font-mono text-xl font-bold tracking-wider text-[#fbbf24] shadow-inner max-lg:flex max-lg:h-8 max-lg:items-center max-lg:px-2.5 max-lg:py-0 max-lg:text-sm">
              <CountdownTimer seconds={session.remaining_sec} onExpire={onFinish} />
            </div>
          )}
        </div>
      </footer>

      {/* ZOOM MODAL */}
      {zoomImageUrl && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 p-4"
          onClick={() => setZoomImageUrl(null)}
        >
          <div className="relative max-h-[95vh] max-w-[95vw]">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={zoomImageUrl} alt="Zoom" className="max-h-[90vh] max-w-[90vw] object-contain border border-white" />
            <button
              type="button"
              onClick={() => setZoomImageUrl(null)}
              className="absolute -top-10 right-0 text-white hover:text-slate-300 max-lg:top-2 max-lg:right-2 max-lg:flex max-lg:h-11 max-lg:w-11 max-lg:items-center max-lg:justify-center max-lg:rounded-full max-lg:bg-black/70"
            >
              <X className="w-8 h-8 max-lg:w-6 max-lg:h-6" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
