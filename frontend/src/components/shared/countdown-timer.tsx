import { cn } from "@/lib/utils";

export interface CountdownTimerProps {
  remainingSeconds?: number;
  seconds?: number;
  initialSeconds?: number;
  onExpire?: () => void;
}

function formatTime(totalSeconds: number): string {
  const clamped = Math.max(0, totalSeconds);
  const minutes = Math.floor(clamped / 60);
  const seconds = clamped % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export function CountdownTimer({ remainingSeconds, seconds, initialSeconds }: CountdownTimerProps) {
  const total = remainingSeconds ?? seconds ?? initialSeconds ?? 0;
  const isLowTime = total <= 60;
  return (
    <span
      data-testid="countdown-timer"
      className={cn(
        "font-display text-2xl font-bold tabular-nums",
        isLowTime ? "animate-pulse text-danger" : "text-gold"
      )}
    >
      {formatTime(total)}
    </span>
  );
}
