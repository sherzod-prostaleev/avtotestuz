"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import {
  AlertTriangle,
  CheckCircle2,
  CircleHelp,
  Info,
  Lightbulb,
  ListChecks,
  X,
} from "lucide-react";
import type { QuestionExplanation, QuestionExplanationBlock } from "@/hooks/use-session-engine";

interface ExplanationDialogProps {
  open: boolean;
  onClose: () => void;
  questionNumber: number;
  questionText: string;
  imageUrl?: string | null;
  explanation: QuestionExplanation | null;
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
      return { labelKey: "explanationImportant", className: "border-gold/40 bg-gold/10", Icon: AlertTriangle };
    case "warning":
    case "ogohlantirish":
      return { labelKey: "explanationWarning", className: "border-danger/40 bg-danger/10", Icon: AlertTriangle };
    case "tip":
    case "maslahat":
      return { labelKey: "explanationTip", className: "border-success/40 bg-success/10", Icon: Lightbulb };
    case "option_analysis":
    case "answer_analysis":
      return { labelKey: "explanationAnswers", className: "border-accent/30 bg-background/60", Icon: ListChecks };
    case "summary":
    case "xulosa":
      return { labelKey: "explanationSummary", className: "border-success/40 bg-success/10", Icon: CheckCircle2 };
    case "eslatma":
      return { labelKey: "explanationNote", className: "border-accent/30 bg-background/60", Icon: CircleHelp };
    default:
      return { labelKey: "explanationRule", className: "border-border bg-background/60", Icon: Info };
  }
}

/**
 * Full-screen explanation surface. Deliberately separate from the answering flow: keeping it out
 * of the question column is what lets that column hold a fixed height and never scroll the page.
 */
export function ExplanationDialog({
  open,
  onClose,
  questionNumber,
  questionText,
  imageUrl,
  explanation,
}: ExplanationDialogProps) {
  const t = useTranslations("Session");

  useEffect(() => {
    if (!open) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  if (!open || !explanation || explanation.blocks.length === 0) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t("explanationTitle")}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/80 p-0 sm:items-center sm:p-6"
    >
      <div className="session-explanation-panel flex w-full max-w-6xl flex-col overflow-hidden rounded-t-3xl border border-border bg-card sm:h-full sm:rounded-3xl">
        <header className="flex shrink-0 items-start gap-3 border-b border-border p-4 sm:p-5">
          <Lightbulb className="mt-0.5 h-5 w-5 shrink-0 text-gold" aria-hidden="true" />
          <div className="min-w-0 flex-1">
            <p className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground">
              {t("expertAnalysis")}
            </p>
            <h2 className="mt-1 font-display text-base font-bold leading-snug sm:text-lg">{questionText}</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("closeExplanation")}
            className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-border text-muted-foreground transition-colors hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <X className="h-5 w-5" aria-hidden="true" />
          </button>
        </header>

        <div
          className={`grid min-h-0 flex-1 ${
            imageUrl ? "lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.6fr)]" : "grid-cols-1"
          }`}
        >
          {imageUrl && (
            <div className="flex shrink-0 items-start justify-center border-b border-border bg-background/40 p-4 lg:min-h-0 lg:shrink lg:border-b-0 lg:border-r lg:p-5">
              {/* Dynamic media URL is served by the backend. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={imageUrl}
                alt={t("questionImageAlt", { number: questionNumber })}
                className="max-h-40 w-full rounded-2xl object-contain lg:max-h-full"
              />
            </div>
          )}

          <div className="min-h-0 overflow-y-auto p-4 sm:p-5">
            <div className="space-y-3">
              {Array.isArray(explanation.legal_refs) && explanation.legal_refs.length > 0 && (
                <section className="rounded-2xl border border-accent/30 bg-accent/5 p-4">
                  <h3 className="mb-2 text-xs font-extrabold uppercase tracking-wider text-accent">
                    {t("legalRefsTitle")}
                  </h3>
                  <ul className="flex flex-wrap gap-2">
                    {explanation.legal_refs.map((ref) => (
                      <li
                        key={`${ref.code}-${ref.title}`}
                        className="inline-flex max-w-full items-center rounded-xl border border-border bg-card px-3 py-1.5 text-xs font-bold text-foreground shadow-raised-sm"
                        title={ref.title}
                      >
                        <span className="truncate">{ref.code}</span>
                      </li>
                    ))}
                  </ul>
                </section>
              )}
              {explanation.blocks.map((block, index) => {
                const style = explanationStyle(block.type);
                const Icon = style.Icon;
                return (
                  <section key={`${block.type}-${index}`} className={`rounded-2xl border p-4 ${style.className}`}>
                    <h3 className="mb-2 flex items-center gap-2 text-xs font-extrabold uppercase tracking-wider">
                      <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                      {t(style.labelKey)}
                    </h3>
                    <p className="whitespace-pre-line text-sm leading-6 text-foreground/90">{block.content}</p>
                  </section>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
