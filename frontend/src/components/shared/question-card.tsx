"use client";

import { useTranslations } from "next-intl";
import {
  AlertTriangle,
  CheckCircle2,
  CircleHelp,
  Info,
  Lightbulb,
  ListChecks,
  ZoomIn,
} from "lucide-react";
import type { QuestionExplanation, QuestionExplanationBlock } from "@/hooks/use-session-engine";

interface QuestionCardProps {
  questionNumber: number;
  totalQuestions: number;
  questionText?: string;
  text?: string;
  imageUrl?: string | null;
  /** Kept for the retired mockup surface; no fake image is rendered without a real URL. */
  hasImage?: boolean;
  onImageClick?: () => void;
  explanation?: QuestionExplanation | null;
}

type ExplanationStyle = {
  labelKey:
    | "explanationIntro"
    | "explanationImportant"
    | "explanationWarning"
    | "explanationTip"
    | "explanationAnswers"
    | "explanationSummary"
    | "explanationNote"
    | "explanationRule";
  className: string;
  Icon: typeof Info;
};

function explanationStyle(type: QuestionExplanationBlock["type"] | string): ExplanationStyle {
  switch (type) {
    case "intro":
      return { labelKey: "explanationIntro", className: "border-accent/30 bg-accent/5", Icon: Info };
    case "important":
    case "muhim":
      return {
        labelKey: "explanationImportant",
        className: "border-gold/40 bg-gold/10",
        Icon: AlertTriangle,
      };
    case "warning":
    case "ogohlantirish":
      return {
        labelKey: "explanationWarning",
        className: "border-danger/40 bg-danger/10",
        Icon: AlertTriangle,
      };
    case "tip":
    case "maslahat":
      return {
        labelKey: "explanationTip",
        className: "border-success/40 bg-success/10",
        Icon: Lightbulb,
      };
    case "option_analysis":
    case "answer_analysis":
      return {
        labelKey: "explanationAnswers",
        className: "border-accent/30 bg-background/60",
        Icon: ListChecks,
      };
    case "summary":
    case "xulosa":
      return {
        labelKey: "explanationSummary",
        className: "border-success/40 bg-success/10",
        Icon: CheckCircle2,
      };
    case "eslatma":
      return {
        labelKey: "explanationNote",
        className: "border-accent/30 bg-background/60",
        Icon: CircleHelp,
      };
    default:
      return {
        labelKey: "explanationRule",
        className: "border-border bg-background/60",
        Icon: Info,
      };
  }
}

function ExplanationBlock({ block, collapsed }: { block: QuestionExplanationBlock; collapsed: boolean }) {
  const t = useTranslations("Session");
  const style = explanationStyle(block.type);
  const Icon = style.Icon;
  const body = <p className="whitespace-pre-line text-sm leading-6 text-foreground/90">{block.content}</p>;

  if (collapsed) {
    return (
      <details className={`rounded-xl border p-4 ${style.className}`}>
        <summary className="flex min-h-7 cursor-pointer list-none items-center gap-2 text-xs font-extrabold uppercase tracking-wider">
          <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
          {t(style.labelKey)}
        </summary>
        <div className="mt-3 border-t border-current/10 pt-3">{body}</div>
      </details>
    );
  }

  return (
    <section className={`rounded-xl border p-4 ${style.className}`}>
      <h3 className="mb-2 flex items-center gap-2 text-xs font-extrabold uppercase tracking-wider">
        <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
        {t(style.labelKey)}
      </h3>
      {body}
    </section>
  );
}

export function QuestionCard({
  questionNumber,
  totalQuestions,
  questionText,
  text,
  imageUrl,
  onImageClick,
  explanation,
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

      {explanation && explanation.blocks.length > 0 && (
        <section className="space-y-3 border-t border-border pt-5" aria-label={t("explanationTitle")}>
          <div className="flex items-center gap-2">
            <Lightbulb className="h-5 w-5 text-gold" aria-hidden="true" />
            <h2 className="font-display text-lg font-bold">{t("explanationTitle")}</h2>
          </div>
          {explanation.blocks.map((block, index) => (
            <ExplanationBlock key={`${block.type}-${index}`} block={block} collapsed={index > 0} />
          ))}
        </section>
      )}
    </div>
  );
}
