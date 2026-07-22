import { cn } from "@/lib/utils";

export interface MasteryBarProps {
  categoryName: string;
  masteryPercent: number;
}

function colorForMastery(percent: number): string {
  if (percent >= 80) return "bg-success";
  if (percent >= 40) return "bg-gold";
  return "bg-danger";
}

export function MasteryBar({ categoryName, masteryPercent }: MasteryBarProps) {
  const clamped = Math.min(100, Math.max(0, masteryPercent));
  return (
    <div>
      <div className="mb-1 flex items-center justify-between text-sm">
        <span className="font-medium">{categoryName}</span>
        <span className="text-muted-foreground">{clamped}%</span>
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
