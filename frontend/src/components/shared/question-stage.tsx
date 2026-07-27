"use client";

import { useTranslations } from "next-intl";
import { BookOpen, ZoomIn } from "lucide-react";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import { useFitScale } from "@/hooks/use-fit-scale";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";
import { resolveQuestionImageUrl } from "@/lib/question-image";

/**
 * Beyond this much combined question + answer text the default density stops fitting a short
 * laptop viewport, so the stage drops one step rather than overflowing.
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
 * Answering surface for practice sessions.
 *
 * Mobile: fixed-height shell. Image/gaps/type compact via CSS; if content still exceeds
 * the handed height, useFitScale scales the block down (never up). Prefer a fully visible
 * question image + no answer overlap over zero-scroll when those trade off.
 * Answers stay content-sized (shrink-0 / h-auto) so F1/F2 labels never overlap.
 *
 * Desktop (lg+): two-column grid; image absorbs slack via object-contain.
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

  const { viewportRef, contentRef, scale, contentStyle } = useFitScale([
    question.id,
    question.question,
    question.answers.length,
    answered,
    hasExplanation,
  ]);

  const questionColumn = (
    <div className="session-question-copy flex min-h-0 w-full flex-col gap-1 sm:gap-3 lg:h-full">
      <div className="shrink-0 space-y-0.5 sm:space-y-2">
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
        <h1 className="session-question-title font-display font-bold leading-snug tracking-tight text-foreground">
          {question.question}
        </h1>
      </div>

      <div
        data-testid="answer-list"
        className="flex flex-col gap-1 sm:gap-2.5 lg:min-h-0 lg:flex-1 lg:overflow-y-auto lg:overscroll-contain"
      >
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
      ref={viewportRef}
      data-testid="question-stage"
      data-layout="two-column"
      data-density={compact ? "compact" : "default"}
      data-fit-scale={scale.toFixed(3)}
      className="h-full min-h-0 overflow-hidden"
    >
      <div
        ref={contentRef}
        style={contentStyle}
        className="flex flex-col gap-1 sm:gap-3 lg:grid lg:h-full lg:grid-cols-[minmax(320px,0.85fr)_minmax(0,1.15fr)] lg:grid-rows-1 lg:gap-4"
      >
        <button
          type="button"
          onClick={onZoomImage}
          aria-label={t("zoomImage")}
          className="session-question-image group relative order-first w-full shrink-0 overflow-hidden rounded-xl border border-border bg-muted/40 transition-colors hover:border-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:rounded-2xl lg:order-last lg:h-full lg:min-h-0 lg:shrink"
        >
          {/* Dynamic media URLs are served by the backend and intentionally stay unoptimized.
              Sizing comes from .session-question-image > img (max-height:inherit + contain). */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            key={question.id}
            src={imageUrl}
            alt={t("questionImageAlt", { number: questionNumber })}
            decoding="async"
            className="object-contain"
          />
          <span className="absolute bottom-1.5 right-1.5 inline-flex h-8 w-8 items-center justify-center rounded-lg bg-foreground/90 text-background sm:bottom-2 sm:right-2 sm:min-h-9 sm:w-auto sm:gap-1.5 sm:px-3 sm:py-1.5 sm:text-xs sm:font-bold">
            <ZoomIn className="h-3.5 w-3.5 sm:h-4 sm:w-4" aria-hidden="true" />
            <span className="hidden sm:inline">{t("zoomImage")}</span>
          </span>
        </button>

        {questionColumn}
      </div>
    </div>
  );
}
