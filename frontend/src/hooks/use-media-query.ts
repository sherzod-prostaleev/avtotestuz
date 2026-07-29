"use client";

import { useCallback, useSyncExternalStore } from "react";

/**
 * SSR-safe media query subscription.
 *
 * The server snapshot is always `false`, so the first client paint matches the
 * server HTML and React never reports a hydration mismatch. The real value
 * arrives on the effect tick right after hydration.
 */
export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onChange: () => void) => {
      const mql = window.matchMedia(query);
      mql.addEventListener("change", onChange);
      return () => mql.removeEventListener("change", onChange);
    },
    [query],
  );

  const getSnapshot = useCallback(() => window.matchMedia(query).matches, [query]);
  const getServerSnapshot = useCallback(() => false, []);

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

/** Phone-width layout: below Tailwind's `md` breakpoint. */
export function useIsCompact(): boolean {
  return useMediaQuery("(max-width: 767px)");
}
