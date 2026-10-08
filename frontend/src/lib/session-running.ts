import { useEffect, useSyncExternalStore } from "react";

/**
 * A count of mounted reporters, not a flag: during a route transition the
 * outgoing and incoming runner can both be mounted, and the first one to
 * unmount must not clear the second's state.
 */
function createCounter() {
  let count = 0;
  const listeners = new Set<() => void>();
  const emit = () => listeners.forEach((listener) => listener());
  const subscribe = (listener: () => void) => {
    listeners.add(listener);
    return () => listeners.delete(listener);
  };
  const snapshot = () => count > 0;
  const serverSnapshot = () => false;

  function useReport(active: boolean): void {
    useEffect(() => {
      if (!active) return;
      count += 1;
      emit();
      return () => {
        count -= 1;
        emit();
      };
    }, [active]);
  }

  function useActive(): boolean {
    return useSyncExternalStore(subscribe, snapshot, serverSnapshot);
  }

  return { useReport, useActive };
}

const running = createCounter();
const settled = createCounter();

/**
 * Called by a test runner with whether its attempt is actually in progress
 * (not loading, not on the result screen). The route alone cannot say that:
 * a finished session keeps the same /session/<id> URL.
 */
export const useReportSessionRunning = running.useReport;

/** True while some mounted runner reports an attempt in progress. */
export const useSessionRunning = running.useActive;

/**
 * Called by a runner with whether it has settled on a screen with nothing
 * left to lose: the result, or an error it cannot recover from. Until then
 * (loading) the Telegram chrome keeps Back hidden, so it does not flash on
 * for the load and vanish when the attempt starts.
 */
export const useReportSessionSettled = settled.useReport;

/** True while some mounted runner reports it has settled. */
export const useSessionSettled = settled.useActive;
