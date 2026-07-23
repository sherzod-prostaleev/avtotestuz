"use client";

import { useTranslations } from "next-intl";
import { BookOpen, ZoomIn } from "lucide-react";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";

/**
 * Beyond this much combined question + answer text the default density stops fitting a short
 * laptop viewport, so the stage drops one step rather than letting the page scroll.
 */
const COMPACT_TEXT_THRESHOLD = 420;
const COMPACT_ANSWER_COUNT = 5;

interface QuestionStageProps {
  question: SessionQuestionItem;
  questionNumber: number;
  totalQuestions: number;
  answered: boolean;
  disabled: boolean;
  onSelectAnswer: (answerId: string) => void;
  onZoomImage: () => void;
  onOpenExplanation: () => void;
  answerStateFor?: (answerId: string) => AnswerState;
}

function isCompact(question: SessionQuestionItem): boolean {
  if (question.answers.length >= COMPACT_ANSWER_COUNT) return true;
  const textLength =
    question.question.length +
    question.answers.reduce((total, answer) => total + answer.text.length, 0);
  return textLength > COMPACT_TEXT_THRESHOLD;
}

/**
 * Owns the height budget of the answering surface. The page around it is a fixed-height flex
 * column, so this component must never grow past the space it is handed: the image absorbs the
 * slack via object-contain, and only the answer list may scroll internally.
 */
export function QuestionStage({
  question,
  questionNumber,
  totalQuestions,
  answered,
  disabled,
  onSelectAnswer,
  onZoomImage,
  onOpenExplanation,
  answerStateFor,
}: QuestionStageProps) {
  const t = useTranslations("Session");
  const hasImage = Boolean(question.image_url);
  const compact = isCompact(question);
  const hasExplanation = Boolean(question.explanation && question.explanation.blocks.length > 0);

  const questionColumn = (
    <div className={`flex h-full min-h-0 flex-col ${compact ? "gap-2.5" : "gap-4"}`}>
      <div className="shrink-0">
        <span className="inline-flex items-center rounded-full border border-accent/30 bg-accent/10 px-3 py-1 text-xs font-extrabold text-accent">
          {t("questionPosition", { number: questionNumber, total: totalQuestions })}
        </span>
        <h1
          className={`mt-2.5 font-display font-bold leading-snug tracking-tight text-foreground ${
            compact ? "text-base sm:text-lg" : hasImage ? "text-lg sm:text-xl" : "text-xl sm:text-2xl"
          }`}
        >
          {question.question}
        </h1>
      </div>

      <div className={`min-h-0 flex-1 overflow-y-auto ${compact ? "space-y-2 pr-1" : "space-y-3 pr-1"}`}>
        {question.answers.map((answer, index) => (
          <AnswerOption
            key={answer.id}
            id={answer.id}
            index={index}
            text={answer.text}
            state={answerStateFor ? answerStateFor(answer.id) : "neutral"}
            disabled={disabled}
            onSelect={onSelectAnswer}
          />
        ))}
      </div>

      {answered && hasExplanation && (
        <div className="shrink-0">
          <button
            type="button"
            onClick={onOpenExplanation}
            className="inline-flex min-h-11 items-center gap-2 rounded-2xl border border-gold/40 bg-gold/10 px-4 text-sm font-bold text-gold transition-colors hover:bg-gold/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <BookOpen className="h-4 w-4" aria-hidden="true" />
            {t("expertAnalysis")}
          </button>
        </div>
      )}
    </div>
  );

  if (!hasImage) {
    return (
      <div
        data-testid="question-stage"
        data-layout="single-column"
        data-density={compact ? "compact" : "default"}
        className="mx-auto flex h-full min-h-0 w-full max-w-3xl flex-col"
      >
        {questionColumn}
      </div>
    );
  }

  return (
    <div
      data-testid="question-stage"
      data-layout="two-column"
      data-density={compact ? "compact" : "default"}
      className="grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)] gap-3 lg:grid-cols-[minmax(380px,0.85fr)_minmax(0,1.15fr)] lg:grid-rows-1 lg:gap-4"
    >
      {questionColumn}

      <button
        type="button"
        onClick={onZoomImage}
        aria-label={t("zoomImage")}
        className="group relative order-first min-h-0 overflow-hidden rounded-2xl border border-border bg-black/5 transition-colors hover:border-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring lg:order-none"
      >
        {/* Dynamic media URLs are served by the backend and intentionally stay unoptimized. */}
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={question.image_url ?? ""}
          alt={t("questionImageAlt", { number: questionNumber })}
          className="h-full max-h-[35dvh] w-full object-contain lg:max-h-full"
        />
        <span className="absolute bottom-2 right-2 inline-flex items-center gap-1.5 rounded-full bg-slate-950/85 px-3 py-1.5 text-xs font-bold text-white shadow-md backdrop-blur-sm">
          <ZoomIn className="h-4 w-4" aria-hidden="true" />
          {t("zoomImage")}
        </span>
      </button>
    </div>
  );
}
