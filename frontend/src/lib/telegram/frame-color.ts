import { useEffect, useSyncExternalStore } from "react";

/**
 * The colour a full-screen screen wants Telegram's header and background
 * painted with, when it is not the theme's --background (the official exam
 * view is always dark navy, whatever the theme). A stack, not a slot: during
 * a route transition the outgoing and incoming screen can both be mounted,
 * and the first to unmount must not clear the other's colour. The chrome
 * prefers the newest report and falls back to --background when none is left.
 */
const stack: { color: string }[] = [];
const listeners = new Set<() => void>();

function emit() {
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

const snapshot = () => stack[stack.length - 1]?.color ?? null;

/** Reports `color` ("#rrggbb") for as long as the calling screen is mounted. */
export function useReportTelegramFrameColor(color: string | null): void {
  useEffect(() => {
    if (!color) return;
    const entry = { color };
    stack.push(entry);
    emit();
    return () => {
      const index = stack.indexOf(entry);
      if (index !== -1) stack.splice(index, 1);
      emit();
    };
  }, [color]);
}

export function useTelegramFrameColor(): string | null {
  return useSyncExternalStore(subscribe, snapshot, () => null);
}
