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
    <div>
      <div className="mb-1 flex items-center justify-between gap-2 text-sm">
        <span className="min-w-0 truncate font-medium">{categoryName}</span>
        <span className="shrink-0 text-muted-foreground">
          {coverage ? (
            <>
              <span className="font-semibold text-foreground">{clamped}%</span>
              <span className="mx-1 text-border">·</span>
              <span className="tabular-nums">{coverage}</span>
            </>
          ) : (
            `${clamped}%`
          )}
        </span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-border" data-testid="mastery-bar-track">
        <div
          data-testid="mastery-bar-fill"
          className={cn("h-full rounded-full transition-all", colorForMastery(clamped))}
          style={{ width: `${clamped}%` }}
        />
      </div>
    </div>
  );
}
