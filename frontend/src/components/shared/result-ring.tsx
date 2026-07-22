import { cn } from "@/lib/utils";

export interface ResultRingProps {
  percent: number;
  label?: string;
}

const RADIUS = 54;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

export function ResultRing({ percent, label }: ResultRingProps) {
  const clamped = Math.min(100, Math.max(0, percent));
  const offset = CIRCUMFERENCE * (1 - clamped / 100);
  const ringColorClass = clamped >= 80 ? "text-gold" : clamped >= 50 ? "text-accent" : "text-danger";

  return (
    <div className="relative inline-flex h-32 w-32 items-center justify-center">
      <svg viewBox="0 0 120 120" className="h-full w-full -rotate-90">
        <circle cx="60" cy="60" r={RADIUS} strokeWidth="10" className="fill-none stroke-border" />
        <circle
          data-testid="result-ring-progress"
          cx="60"
          cy="60"
          r={RADIUS}
          strokeWidth="10"
          strokeLinecap="round"
          strokeDasharray={CIRCUMFERENCE}
          strokeDashoffset={offset}
          className={cn("fill-none stroke-current transition-all", ringColorClass)}
        />
      </svg>
      <div className="absolute flex flex-col items-center">
        <span className="font-display text-3xl font-extrabold">{clamped}%</span>
        {label && <span className="text-xs text-muted-foreground">{label}</span>}
      </div>
    </div>
  );
}
