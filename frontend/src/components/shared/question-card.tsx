"use client";

import { useTranslations } from "next-intl";
import { ZoomIn } from "lucide-react";

interface QuestionCardProps {
  questionNumber: number;
  totalQuestions: number;
  questionText?: string;
  text?: string;
  imageUrl?: string | null;
  /** Kept for the retired mockup surface; no fake image is rendered without a real URL. */
  hasImage?: boolean;
  onImageClick?: () => void;
}

/**
 * Presentational question header used by the exam mockup. The live session screen composes its own
 * layout through QuestionStage, which owns the height budget; explanations live in
 * ExplanationDialog so nothing in the answering flow can grow unbounded.
 */
export function QuestionCard({
  questionNumber,
  totalQuestions,
  questionText,
  text,
  imageUrl,
  onImageClick,
}: QuestionCardProps) {
  const t = useTranslations("Session");
  const displayText = questionText || text || "";

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <span className="inline-flex items-center gap-1.5 rounded-full border border-accent/30 bg-accent/10 px-3.5 py-1 text-xs font-extrabold text-accent">
          {t("questionPosition", { number: questionNumber, total: totalQuestions })}
        </span>
      </div>

      <h1 className="font-display text-xl font-bold leading-snug tracking-tight text-foreground sm:text-2xl">
        {displayText}
      </h1>

      {imageUrl && (
        <button
          type="button"
          onClick={onImageClick}
          disabled={!onImageClick}
          aria-label={t("zoomImage")}
          className="group relative block w-full overflow-hidden rounded-2xl border border-border bg-black/5 transition-all hover:border-accent/60 hover:shadow-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-default"
        >
          {/* Dynamic media URLs are served by the backend and intentionally stay unoptimized. */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={imageUrl}
            alt={t("questionImageAlt", { number: questionNumber })}
            className="mx-auto max-h-[28rem] w-full object-contain transition-transform duration-300 group-hover:scale-[1.01]"
          />
          {onImageClick && (
            <span className="absolute bottom-3 right-3 inline-flex items-center gap-1.5 rounded-full bg-slate-950/85 px-3.5 py-2 text-xs font-bold text-white shadow-md backdrop-blur-sm">
              <ZoomIn className="h-4 w-4" aria-hidden="true" />
              {t("zoomImage")}
            </span>
          )}
        </button>
      )}
    </div>
  );
}
