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

export function ViewTransitions({ children }: { children?: React.ReactNode }) {
  const run = useCallback<TransitionNavigate>((navigate) => {
    navigate(false);
  }, []);

  return <TransitionContext.Provider value={run}>{children}</TransitionContext.Provider>;
}

