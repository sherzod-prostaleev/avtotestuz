import { Check, X } from "lucide-react";
import { cn } from "@/lib/utils";

export type AnswerState = "neutral" | "selected" | "correct" | "incorrect" | "hidden";

export interface AnswerOptionProps {
  shortcutLabel: string;
  text: string;
  state: AnswerState;
  onSelect?: () => void;
}

const stateClasses: Record<AnswerState, string> = {
  neutral: "border-border bg-card hover:border-accent",
  selected: "border-accent bg-accent/10",
  correct: "border-success bg-success/15",
  incorrect: "border-danger bg-danger/15",
  hidden: "border-border bg-card",
};

// NOTE: there is deliberately no `isCorrect` prop. The caller can only make
// this component reveal correctness by explicitly passing state="correct" /
// "incorrect" — in exam mode the caller only ever has "neutral"/"selected"/
// "hidden" available because the backend withholds correctness entirely
// until the session ends (see backend/internal/session — anti-cheat).
export function AnswerOption({ shortcutLabel, text, state, onSelect }: AnswerOptionProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "flex w-full items-center gap-4 rounded-lg border-2 px-4 py-3 text-left transition-colors",
        stateClasses[state]
      )}
    >
      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-border font-display text-sm font-bold">
        {shortcutLabel}
      </span>
      <span className="flex-1 font-medium">{text}</span>
      {state === "correct" && <Check data-testid="answer-correct-icon" aria-hidden className="h-5 w-5 text-success" />}
      {state === "incorrect" && <X data-testid="answer-incorrect-icon" aria-hidden className="h-5 w-5 text-danger" />}
    </button>
  );
}
