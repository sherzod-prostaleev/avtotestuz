"use client";

import { useTranslations } from "next-intl";
import { BookOpen, ZoomIn } from "lucide-react";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";
import { resolveQuestionImageUrl } from "@/lib/question-image";

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
 * slack via object-contain, and only the answer list may scroll internally as a last resort.
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
  const imageUrl = resolveQuestionImageUrl(question.image_url);
  const compact = isCompact(question);
  const hasExplanation = Boolean(question.explanation && question.explanation.blocks.length > 0);
  const progressPct = totalQuestions > 0 ? Math.round((questionNumber / totalQuestions) * 100) : 0;

  const questionColumn = (
    <div className="flex h-full min-h-0 flex-col gap-1.5 sm:gap-3">
      <div className="shrink-0 space-y-1 sm:space-y-2">
        <div className="flex items-center justify-between gap-2">
          <span className="inline-flex items-center rounded-md border border-accent/30 bg-accent/10 px-2 py-0.5 text-[10px] font-extrabold tabular-nums text-accent sm:px-3 sm:py-1 sm:text-xs">
            {t("questionPosition", { number: questionNumber, total: totalQuestions })}
          </span>
          <span className="text-[10px] font-bold tabular-nums text-muted-foreground sm:text-[11px]">
            {progressPct}%
          </span>
        </div>
        <div
          className="h-1 w-full overflow-hidden rounded-full bg-border sm:h-1.5"
          role="progressbar"
          aria-valuenow={questionNumber}
          aria-valuemin={1}
          aria-valuemax={totalQuestions}
          aria-label={t("questionPosition", { number: questionNumber, total: totalQuestions })}
        >
          <div
            className="h-full rounded-full bg-accent transition-[width] duration-300"
            style={{ width: `${progressPct}%` }}
          />
        </div>
        <h1 className="font-display text-sm font-bold leading-snug tracking-tight text-foreground sm:text-xl">
          {question.question}
        </h1>
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto overscroll-contain sm:gap-2.5">
        {question.answers.map((answer, index) => (
          <AnswerOption
            key={answer.id}
            id={answer.id}
            index={index}
            text={answer.text}
            dense
            state={answerStateFor ? answerStateFor(answer.id) : "neutral"}
            disabled={disabled}
            onSelect={onSelectAnswer}
          />
        ))}
      </div>

      {answered && hasExplanation && (
        <div className="shrink-0 pt-0.5">
          <button
            type="button"
            onClick={onOpenExplanation}
            className="inline-flex min-h-9 w-full items-center justify-center gap-1.5 rounded-xl border border-gold/40 bg-gold/10 px-3 text-xs font-bold text-gold transition-colors hover:bg-gold/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:min-h-11 sm:w-auto sm:rounded-2xl sm:px-4 sm:text-sm"
          >
            <BookOpen className="h-3.5 w-3.5 sm:h-4 sm:w-4" aria-hidden="true" />
            {t("expertAnalysis")}
          </button>
        </div>
      )}
    </div>
  );

  return (
    <div
      data-testid="question-stage"
      data-layout="two-column"
      data-density={compact ? "compact" : "default"}
      className="grid h-full min-h-0 grid-rows-[minmax(0,22dvh)_minmax(0,1fr)] gap-1.5 sm:gap-3 lg:grid-cols-[minmax(320px,0.85fr)_minmax(0,1.15fr)] lg:grid-rows-1 lg:gap-4"
    >
      <button
        type="button"
        onClick={onZoomImage}
        aria-label={t("zoomImage")}
        className="group relative order-first min-h-0 overflow-hidden rounded-xl border border-border bg-muted/40 transition-colors hover:border-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:rounded-2xl lg:order-last"
      >
        {/* Dynamic media URLs are served by the backend and intentionally stay unoptimized. */}
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          key={question.id}
          src={imageUrl}
          alt={t("questionImageAlt", { number: questionNumber })}
          decoding="async"
          className="h-full max-h-[22dvh] w-full object-contain lg:max-h-full"
        />
        <span className="absolute bottom-1.5 right-1.5 inline-flex h-8 w-8 items-center justify-center rounded-lg bg-foreground/90 text-background sm:bottom-2 sm:right-2 sm:min-h-9 sm:w-auto sm:gap-1.5 sm:px-3 sm:py-1.5 sm:text-xs sm:font-bold">
          <ZoomIn className="h-3.5 w-3.5 sm:h-4 sm:w-4" aria-hidden="true" />
          <span className="hidden sm:inline">{t("zoomImage")}</span>
        </span>
      </button>

      {questionColumn}
    </div>
  );
}
