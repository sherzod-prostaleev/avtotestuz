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
    neutral: "border-border bg-card/80 text-foreground hover:border-accent/60 hover:bg-card hover:-translate-y-0.5 shadow-sm",
    selected: "border-accent bg-accent/15 text-accent font-bold ring-2 ring-accent/40 shadow-md",
    correct: "border-success bg-success/15 text-success font-bold ring-2 ring-success/40 shadow-md",
    wrong: "border-danger bg-danger/15 text-danger font-bold ring-2 ring-danger/40 shadow-md",
  };

  const keyBadgeStyles: Record<string, string> = {
    neutral: "border-border bg-background/80 text-muted-foreground",
    selected: "border-accent/40 bg-accent text-white font-bold",
    correct: "border-success/40 bg-success text-white font-bold",
    wrong: "border-danger/40 bg-danger text-white font-bold",
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      disabled={disabled}
      className={`group relative flex w-full items-center justify-between gap-4 rounded-2xl border p-4 text-left transition-all duration-200 active:scale-[0.99] disabled:cursor-not-allowed disabled:opacity-80 ${stateStyles[normalizedState]}`}
    >
      <div className="flex items-center gap-3.5">
        {/* Shortcut Key Badge */}
        {keyLabel && (
          <span
            className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-xl border text-xs font-bold transition-colors ${keyBadgeStyles[normalizedState]}`}
          >
            {keyLabel}
          </span>
        )}

        {/* Option Text */}
        <span className="text-sm font-medium leading-relaxed">{text}</span>
      </div>

      {/* Result Icons */}
      {normalizedState === "correct" && (
        <CheckCircle2 data-testid="answer-correct-icon" className="h-5 w-5 shrink-0 text-success animate-bounce" />
      )}
      {normalizedState === "wrong" && (
        <XCircle data-testid="answer-incorrect-icon" className="h-5 w-5 shrink-0 text-danger animate-pulse" />
      )}
    </button>
  );
}
