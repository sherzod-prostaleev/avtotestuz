"use client";

import { createContext, useCallback, useContext } from "react";

export type TransitionNavigate = (
  navigate: (driven: boolean) => void,
  opts?: { warm?: boolean },
) => void;

const TransitionContext = createContext<TransitionNavigate | null>(null);

export function useTransitionNavigate(): TransitionNavigate {
  const fromContext = useContext(TransitionContext);
  return fromContext ?? ((navigate) => navigate(false));
}

/**
 * Lightweight pass-through wrapper. No View Transitions API interception.
 *
 * The native `document.startViewTransition()` API takes a full-page screenshot
 * and cross-fades it with the new DOM. This conflicts with Next.js App Router
 * because RSC payloads stream in progressively — the DOM callback resolves
 * before content is fully rendered, causing a visible "jitter" (screenshot
 * snaps, layout shifts mid-animation, content pops in after the fade).
 *
 * OsonPrava (React Router SPA) has no page-level transition animation at all —
 * content swaps instantly. The smooth feel comes from instant navigation +
 * subtle enter-only opacity fade on the incoming content (handled by
 * PageTransition + .page-enter CSS), not from cross-fading two snapshots.
 */
export function ViewTransitions({ children }: { children?: React.ReactNode }) {
  const run = useCallback<TransitionNavigate>((navigate) => {
    navigate(false);
  }, []);

  return <TransitionContext.Provider value={run}>{children}</TransitionContext.Provider>;
}
