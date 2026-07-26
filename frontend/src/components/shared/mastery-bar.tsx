import { cn } from "@/lib/utils";

export interface MasteryBarProps {
  categoryName: string;
  masteryPercent: number;
  /** Distinct questions studied in this category (honest coverage). */
  studied?: number;
  /** Valid questions in the category bank. */
  total?: number;
}

function colorForMastery(percent: number): string {
  if (percent >= 80) return "bg-success";
  if (percent >= 40) return "bg-gold";
  return "bg-danger";
}

export function MasteryBar({ categoryName, masteryPercent, studied, total }: MasteryBarProps) {
  const clamped = Math.min(100, Math.max(0, masteryPercent));
  const coverage =
    typeof studied === "number" && typeof total === "number" && total > 0
      ? `${studied}/${total}`
      : null;
  return (
    // min-w-0: CSS grid/flex children default to min-width:auto and overflow the card.
    <div className="min-w-0 w-full overflow-hidden">
      <div className="mb-1 flex items-center justify-between gap-2 text-xs sm:mb-1.5 sm:text-sm">
        <span className="min-w-0 flex-1 truncate font-semibold">{categoryName}</span>
        <span className="shrink-0 tabular-nums text-muted-foreground">
          {coverage ? (
            <>
              <span className="font-bold text-foreground">{clamped}%</span>
              <span className="mx-0.5 text-border sm:mx-1">·</span>
              <span>{coverage}</span>
            </>
          ) : (
            `${clamped}%`
          )}
        </span>
      </div>
      <div
        className="h-1.5 w-full max-w-full overflow-hidden rounded-full bg-border sm:h-2"
        data-testid="mastery-bar-track"
      >
        <div
          data-testid="mastery-bar-fill"
          className={cn("h-full max-w-full rounded-full transition-all", colorForMastery(clamped))}
          style={{ width: `${clamped}%` }}
        />
      </div>
    </div>
  );
}
