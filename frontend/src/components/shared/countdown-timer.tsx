import { useEffect, useRef, useState } from "react";
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

export function CountdownTimer({ remainingSeconds, seconds, initialSeconds, onExpire }: CountdownTimerProps) {
  const initial = remainingSeconds ?? seconds ?? initialSeconds ?? 0;

  const [remaining, setRemaining] = useState(initial);
  const prevInitialRef = useRef(initial);
  const onExpireRef = useRef(onExpire);
  const hasExpiredRef = useRef(false);

  // Always call the latest onExpire without re-triggering effects on every render.
  onExpireRef.current = onExpire;

  // Re-sync internal state whenever the caller-provided value actually changes
  // (e.g. resuming a session with a different server-reported remaining time),
  // without restarting on unrelated parent re-renders.
  useEffect(() => {
    if (initial !== prevInitialRef.current) {
      prevInitialRef.current = initial;
      setRemaining(initial);
      hasExpiredRef.current = false;
    }
  }, [initial]);

  // Tick once per second while time remains. The effect only re-runs when
  // crossing the zero boundary, not on every tick, so the interval isn't
  // recreated every second.
  useEffect(() => {
    if (remaining <= 0) return;
    const id = setInterval(() => {
      setRemaining((prev) => Math.max(0, prev - 1));
    }, 1000);
    return () => clearInterval(id);
  }, [remaining <= 0]);

  // Fire onExpire exactly once when reaching zero.
  useEffect(() => {
    if (remaining <= 0) {
      if (!hasExpiredRef.current) {
        hasExpiredRef.current = true;
        onExpireRef.current?.();
      }
    } else {
      hasExpiredRef.current = false;
    }
  }, [remaining]);

  const isLowTime = remaining <= 60;

  return (
    <span
      data-testid="countdown-timer"
      className={cn(
        "font-display text-2xl font-bold tabular-nums",
        isLowTime ? "animate-pulse text-danger" : "text-gold"
      )}
    >
      {formatTime(remaining)}
    </span>
  );
}
