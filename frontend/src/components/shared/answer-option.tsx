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
  /** Tighter chrome for mobile session / exam surfaces. */
  dense?: boolean;
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
  dense = false,
}: AnswerOptionProps) {
  const handleClick = () => {
    if (onClick) onClick();
    if (onSelect) onSelect(id);
  };

  const keyLabel = shortcutKey ?? shortcutLabel ?? (typeof index === "number" ? `F${index + 1}` : "");
  const normalizedState = state === "incorrect" ? "wrong" : state === "hidden" ? "selected" : state;

  const stateStyles: Record<string, string> = {
    neutral:
      "border-border bg-card text-foreground shadow-raised-sm hover:border-accent hover:-translate-y-0.5",
    selected: "border-accent bg-accent/15 text-foreground font-bold shadow-3d ring-2 ring-accent/35",
    correct: "border-success bg-success/15 text-foreground font-bold shadow-3d-success ring-2 ring-success/35",
    wrong: "answer-wrong-pulse border-danger bg-danger/15 text-foreground font-bold shadow-3d-danger ring-2 ring-danger/35",
  };

  const keyBadgeStyles: Record<string, string> = {
    neutral: "border-border bg-background text-muted-foreground shadow-raised-sm",
    selected: "border-accent/40 bg-accent text-accent-foreground font-bold shadow-3d",
    correct: "border-success/40 bg-success text-success-foreground font-bold",
    wrong: "border-danger/40 bg-danger text-danger-foreground font-bold",
  };

  // Avoid opacity/scale blink while grading: disabled dim + active:scale fought
  // the old shake transform and read as a full-surface flicker.
  const pressable = normalizedState === "neutral" && !disabled;

  return (
    <button
      type="button"
      onClick={handleClick}
      disabled={disabled}
      data-answer-option=""
      className={`group relative flex h-auto w-full shrink-0 items-start justify-between overflow-visible text-left transition-[border-color,background-color,box-shadow,transform] duration-150 disabled:cursor-not-allowed ${
        dense
          ? "min-h-10 gap-2 rounded-xl border px-2.5 py-2 sm:min-h-12 sm:gap-3 sm:rounded-2xl sm:px-3.5 sm:py-2.5"
          : "min-h-14 gap-3 rounded-2xl border px-3.5 py-3 sm:min-h-[3.5rem] sm:gap-4 sm:px-4"
      } ${pressable ? "active:translate-y-0.5 active:bg-accent/5 active:shadow-none" : ""} ${stateStyles[normalizedState]}`}
    >
      <div className={`flex min-w-0 flex-1 items-start ${dense ? "gap-2 sm:gap-3" : "gap-3"}`}>
        {keyLabel && (
          <span
            className={`mt-0.5 flex shrink-0 items-center justify-center border font-bold transition-colors ${
              dense
                ? "h-8 w-8 rounded-lg text-xs sm:h-9 sm:w-9 sm:rounded-xl sm:text-sm"
                : "h-10 w-10 rounded-xl text-sm sm:h-9 sm:w-9"
            } ${keyBadgeStyles[normalizedState]}`}
          >
            {keyLabel}
          </span>
        )}
        <span
          className={`min-w-0 flex-1 break-words whitespace-normal font-semibold leading-snug [overflow-wrap:anywhere] ${
            dense ? "text-sm sm:text-base sm:leading-relaxed" : "text-base sm:text-lg sm:leading-relaxed"
          }`}
        >
          {text}
        </span>
      </div>

      {normalizedState === "correct" && (
        <CheckCircle2
          data-testid="answer-correct-icon"
          className={`mt-0.5 shrink-0 text-success ${dense ? "h-4 w-4" : "h-5 w-5"}`}
          aria-hidden="true"
        />
      )}
      {normalizedState === "wrong" && (
        <XCircle
          data-testid="answer-incorrect-icon"
          className={`mt-0.5 shrink-0 text-danger ${dense ? "h-4 w-4" : "h-5 w-5"}`}
          aria-hidden="true"
        />
      )}
    </button>
  );
}
