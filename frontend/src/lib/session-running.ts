import { useEffect, useSyncExternalStore } from "react";

// How many mounted runners currently hold an attempt in progress. A count, not
// a flag: during a route transition the outgoing and incoming runner can both
// be mounted, and the first one to unmount must not clear the second's state.
let running = 0;
const listeners = new Set<() => void>();

function emit() {
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

const snapshot = () => running > 0;
const serverSnapshot = () => false;

/**
 * Called by a test runner with whether its attempt is actually in progress
 * (not loading, not on the result screen). The route alone cannot say that:
 * a finished session keeps the same /session/<id> URL.
 */
export function useReportSessionRunning(isRunning: boolean): void {
  useEffect(() => {
    if (!isRunning) return;
    running += 1;
    emit();
    return () => {
      running -= 1;
      emit();
    };
  }, [isRunning]);
}

/** True while some mounted runner reports an attempt in progress. */
export function useSessionRunning(): boolean {
  return useSyncExternalStore(subscribe, snapshot, serverSnapshot);
}
