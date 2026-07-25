"use client";

import { CheckCircle2, XCircle } from "lucide-react";

export type AnswerState = "neutral" | "selected" | "correct" | "wrong" | "incorrect" | "hidden";

interface AnswerOptionProps {
  id?: string;
  index?: number;
  text: string;
  state?: AnswerState;
  onClick?: () => void;
  onSelect?: (id: string) => void;
  shortcutKey?: string;
  shortcutLabel?: string;
  disabled?: boolean;
}

export function AnswerOption({
  id = "",
  index,
  text,
  state = "neutral",
  onClick,
  onSelect,
  shortcutKey,
  shortcutLabel,
  disabled = false,
}: AnswerOptionProps) {
  const handleClick = () => {
    if (onClick) onClick();
    if (onSelect) onSelect(id);
  };

  const keyLabel = shortcutKey ?? shortcutLabel ?? (typeof index === "number" ? `F${index + 1}` : "");
  const normalizedState = state === "incorrect" ? "wrong" : state === "hidden" ? "selected" : state;

  const stateStyles: Record<string, string> = {
    neutral: "border-border bg-card text-foreground hover:border-accent",
    selected: "border-accent bg-accent/15 text-foreground font-bold ring-2 ring-accent/35",
    correct: "border-success bg-success/15 text-foreground font-bold ring-2 ring-success/35",
    wrong: "answer-wrong-shake border-danger bg-danger/15 text-foreground font-bold ring-2 ring-danger/35",
  };

  const keyBadgeStyles: Record<string, string> = {
    neutral: "border-border bg-background text-muted-foreground",
    selected: "border-accent/40 bg-accent text-accent-foreground font-bold",
    correct: "border-success/40 bg-success text-success-foreground font-bold",
    wrong: "border-danger/40 bg-danger text-danger-foreground font-bold",
  };

  // Avoid opacity/scale blink while grading: disabled dim + active:scale fight
  // answer-wrong-shake's transform and read as a full-surface flicker.
  const pressable = normalizedState === "neutral" && !disabled;

  return (
    <button
      type="button"
      onClick={handleClick}
      disabled={disabled}
      className={`group relative flex min-h-14 w-full items-center justify-between gap-3 rounded-2xl border px-3.5 py-3 text-left transition-[border-color,background-color,box-shadow] duration-150 disabled:cursor-not-allowed sm:min-h-[3.5rem] sm:gap-4 sm:px-4 ${
        pressable ? "active:bg-accent/5" : ""
      } ${stateStyles[normalizedState]}`}
    >
      <div className="flex min-w-0 items-center gap-3">
        {keyLabel && (
          <span
            className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border text-xs font-bold transition-colors sm:h-8 sm:w-8 ${keyBadgeStyles[normalizedState]}`}
          >
            {keyLabel}
          </span>
        )}
        <span className="text-[15px] font-medium leading-snug sm:text-sm sm:leading-relaxed">{text}</span>
      </div>

      {normalizedState === "correct" && (
        <CheckCircle2 data-testid="answer-correct-icon" className="h-5 w-5 shrink-0 text-success" aria-hidden="true" />
      )}
      {normalizedState === "wrong" && (
        <XCircle data-testid="answer-incorrect-icon" className="h-5 w-5 shrink-0 text-danger" aria-hidden="true" />
      )}
    </button>
  );
}
