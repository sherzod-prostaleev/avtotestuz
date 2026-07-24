"use client";

import { useEffect, useState } from "react";
import { useLocale } from "next-intl";
import { useRouter } from "next/navigation";
import { X, ZoomIn } from "lucide-react";
import type { SessionQuestionItem, SessionState } from "@/hooks/use-session-engine";
import type { AnswerState } from "@/components/shared/answer-option";
import { CountdownTimer } from "@/components/shared/countdown-timer";

interface OfficialAvtotestExamViewProps {
  session: SessionState;
  currentIndex: number;
  onSelectIndex: (index: number) => void;
  onSelectAnswer: (questionId: string, answerId: string) => void;
  onFinish: () => void;
  submitting: boolean;
  finishing: boolean;
  answerStateFor: (question: SessionQuestionItem, answerId: string) => AnswerState;
}

/**
 * Exam-specific answer visual:
 *  - User chose this AND it was correct → "correct" (green)
 *  - User chose this AND it was wrong  → "wrong"   (red)
 *  - User chose this, no feedback yet  → "selected" (blue)
 *  - Any other answer                  → "neutral"  (dark, never reveals correct)
 */
function examVisual(
  question: SessionQuestionItem,
  answerId: string
): "correct" | "wrong" | "selected" | "neutral" {
  const isUserChoice = question.user_answer_id === answerId;
  const isAnswered = question.answered || Boolean(question.user_answer_id);

  if (!isAnswered || !isUserChoice) return "neutral";

  // User selected this answer
  if (question.correct === true) return "correct";
  if (question.correct === false) return "wrong";
  return "selected"; // feedback not yet received
}

export function OfficialAvtotestExamView({
  session,
  currentIndex,
  onSelectIndex,
  onSelectAnswer,
  onFinish,
  submitting,
  finishing,
}: OfficialAvtotestExamViewProps) {
  const locale = useLocale();
  const router = useRouter();

  const questions = session.questions ?? [];
  const currentQuestion = questions[currentIndex];
  const [zoomImageUrl, setZoomImageUrl] = useState<string | null>(null);

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
    const currentPath = window.location.pathname;
    const parts = currentPath.split("/");
    parts[1] = newLoc;
    router.push(parts.join("/"));
  };

  /* ── Style Maps (ultra-crisp high contrast) ──────── */

  // Answer button container border+bg
  const btnStyles: Record<string, string> = {
    correct:  "border-green-500 bg-[#163820] shadow-md",
    wrong:    "border-red-500 bg-[#421414] shadow-md",
    selected: "border-blue-400 bg-[#162e4a] shadow-md",
    neutral:  "border-[#354f6e] bg-[#162738] hover:border-[#5279a6] hover:bg-[#1f364d] shadow-sm",
  };

  // F-key badge
  const badgeStyles: Record<string, string> = {
    correct:  "bg-green-600 text-white font-black border-green-500",
    wrong:    "bg-red-600 text-white font-black border-red-500",
    selected: "bg-blue-600 text-white font-black border-blue-400",
    neutral:  "bg-[#1d334a] text-white font-black border-[#354f6e]",
  };

  return (
    <div className="relative flex h-screen w-screen flex-col overflow-hidden bg-[#091726] text-white font-sans select-none subpixel-antialiased">
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
      <header className="relative z-10 flex h-[52px] shrink-0 items-center justify-between bg-[#081320]/95 px-5 border-b border-[#1c3554]">
        {/* Left: DriveGo logo */}
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-black border border-emerald-500/50 shadow-[0_0_15px_rgba(34,197,94,0.4)] overflow-hidden">
            <img src="/logo.svg" alt="DriveGo Logo" className="h-full w-full object-cover scale-110" />
          </div>
          <span className="font-black tracking-wider text-2xl text-white" style={{ fontFamily: "'Arial Black', 'Impact', sans-serif" }}>
            Drive<span className="text-[#22c55e]">Go</span>
          </span>
        </div>

        {/* Center: Language tabs */}
        <div className="flex items-center gap-2">
          {[
            { id: "uz-Latn", label: "Uzb (lotin.)" },
            { id: "uz-Cyrl", label: "Uzb (кирил.)" },
            { id: "ru", label: "Rus (кирил.)" },
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
                className={`relative px-3.5 py-1 text-xs font-bold rounded-sm border transition-all ${
                  isActive
                    ? "bg-[#183654] text-white border-[#5a8aaa] shadow-sm"
                    : "bg-[#0f2236] text-slate-200 border-[#284260] hover:bg-[#183654] hover:text-white"
                }`}
              >
                {isActive && <span className="absolute top-0 left-0 w-full h-[3px] bg-[#22c55e]" />}
                {lang.label}
              </button>
            );
          })}
        </div>

        {/* Right: X close */}
        <button
          type="button"
          onClick={() => router.push(`/${locale}/dashboard`)}
          aria-label="Close"
          className="text-slate-300 hover:text-white transition-colors p-1"
        >
          <X className="w-6 h-6" />
        </button>
      </header>

      {/* ═══ QUESTION TEXT BANNER / RESULT BANNER ═══ */}
      <div className="relative z-10 w-full shrink-0">
        {isFailed ? (
          <div className="w-full bg-[#dc2626] text-white text-center py-3 text-xl font-extrabold tracking-wide shadow-md">
            Topshirilmadi
          </div>
        ) : isCompleted ? (
          <div className="w-full bg-[#16a34a] text-white text-center py-3 text-xl font-extrabold tracking-wide shadow-md">
            Topshirildi
          </div>
        ) : (
          <div className="w-full bg-[#0d2e4d] border-y border-[#204a75] text-white text-center py-3.5 px-8 text-lg font-extrabold leading-relaxed tracking-wide shadow-md">
            {currentQuestion ? currentQuestion.question : "Yuklanmoqda..."}
          </div>
        )}
      </div>

      {/* ═══ MAIN 2-COLUMN LAYOUT ═══ */}
      <main className="relative z-10 flex flex-1 min-h-0 w-full px-5 py-4 gap-5 items-stretch">
        {/* LEFT: Answer options */}
        <div className="flex w-[38%] flex-col gap-3 justify-start pt-2">
          {currentQuestion?.answers.map((answer, index) => {
            const shortcutLabel = `F${index + 1}`;
            const visual = examVisual(currentQuestion, answer.id);
            const isAnswered = currentQuestion.answered || Boolean(currentQuestion.user_answer_id);

            return (
              <button
                key={answer.id}
                type="button"
                disabled={isAnswered || submitting || finishing || isCompleted}
                onClick={() => onSelectAnswer(currentQuestion.id, answer.id)}
                className={`group flex w-full items-stretch rounded-sm overflow-hidden border transition-all text-left text-base font-semibold ${btnStyles[visual]}`}
              >
                {/* F-key badge */}
                <div
                  className={`flex w-12 shrink-0 items-center justify-center font-black text-base border-r transition-colors ${badgeStyles[visual]}`}
                >
                  {shortcutLabel}
                </div>
                {/* Text */}
                <div className="flex flex-1 items-center px-4 py-3 leading-normal text-white font-medium">
                  {answer.text}
                </div>
              </button>
            );
          })}
        </div>

        {/* RIGHT: Question image */}
        <div className="flex flex-1 items-center justify-center min-h-0">
          <div
            className="relative flex h-full w-full items-center justify-center border-2 border-slate-300 bg-black overflow-hidden cursor-pointer shadow-xl rounded-sm"
            onClick={() => {
              if (currentQuestion?.image_url) setZoomImageUrl(currentQuestion.image_url);
            }}
          >
            {currentQuestion?.image_url ? (
              <>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={currentQuestion.image_url}
                  alt={currentQuestion.question}
                  className="max-h-full max-w-full object-contain"
                />
                <div className="absolute top-2 right-2 rounded bg-black/60 p-1.5 text-white/70 hover:text-white transition-opacity">
                  <ZoomIn className="w-5 h-5" />
                </div>
              </>
            ) : (
              <div className="text-slate-400 text-base italic font-medium">Rasm mavjud emas</div>
            )}
          </div>
        </div>
      </main>

      {/* ═══ BOTTOM BAR ═══ */}
      <footer className="relative z-10 flex h-[60px] shrink-0 items-center justify-between bg-[#081320]/95 px-5 border-t border-[#1c3554]">
        <div className="w-20" />

        {/* Question number pills */}
        <div className="flex items-center gap-1.5">
          {questions.map((q, idx) => {
            const isCurrent = idx === currentIndex;
            const isAns = q.answered || Boolean(q.user_answer_id);
            const isCorr = q.correct === true;
            const isWrn = q.correct === false;

            let bg: string;
            if (isWrn) bg = "bg-[#dc2626] text-white border border-red-400 shadow-md font-extrabold";
            else if (isCorr) bg = "bg-[#16a34a] text-white border border-green-400 shadow-md font-extrabold";
            else if (isAns) bg = "bg-[#2563eb] text-white border border-blue-400 shadow-md font-extrabold";
            else bg = "bg-[#1c334d] text-white border border-[#304d70] hover:bg-[#28486e] hover:border-white font-extrabold";

            return (
              <button
                key={q.id}
                type="button"
                onClick={() => onSelectIndex(idx)}
                className={`w-8 h-8 flex items-center justify-center text-base rounded-sm transition-all ${
                  isCurrent ? "ring-2 ring-white ring-offset-2 ring-offset-[#081320] scale-110 font-black z-10 shadow-lg" : ""
                } ${bg}`}
              >
                {idx + 1}
              </button>
            );
          })}
        </div>

        {/* Timer */}
        <div className="flex items-center">
          {session.remaining_sec !== null && (
            <div className="bg-[#050b12] border border-[#2a4568] px-4 py-1.5 rounded-sm font-mono text-xl font-bold tracking-wider text-[#fbbf24] shadow-inner">
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
              className="absolute -top-10 right-0 text-white hover:text-slate-300"
            >
              <X className="w-8 h-8" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
