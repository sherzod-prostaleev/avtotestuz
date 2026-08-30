"use client";

import { createContext, useCallback, useContext, useEffect, useRef } from "react";
import { usePathname, useRouter } from "next/navigation";

/**
 * Cross-fades one page into the next.
 *
 * Next cannot drive this on React 19 stable: its experimental.viewTransition
 * flag and Link's transitionTypes both go through React.addTransitionType,
 * which ships only on React's experimental channel. So this listens for link
 * clicks and drives document.startViewTransition itself.
 *
 * The subtlety is that a view transition freezes the screen. The browser holds
 * a picture of the outgoing page until the callback settles, and here that
 * means until the new route commits — a server round trip. Hold it for the
 * whole fetch and the page does not animate, it locks up. Two things keep that
 * from happening:
 *
 *  - links are prefetched as the pointer reaches them, so by the time the click
 *    lands the route is usually already in the router cache and commits in a
 *    frame or two;
 *  - and if it has not committed within HOLD_LIMIT_MS the transition is
 *    skipped, which drops the snapshot at once. The outgoing page goes back to
 *    being live and interactive and the navigation finishes without animation —
 *    never worse than having no transition at all.
 *
 * Scope is deliberately narrow: plain left-clicks on same-origin anchors.
 * router.push calls elsewhere and browser back/forward navigate as before.
 */

/**
 * How long the screen may stay under a snapshot. Only routes already in the
 * router cache get this far, so the time is React rendering the new page — it
 * would be spent either way, and spending it on the outgoing page beats
 * spending it on a half-drawn one. Measured on production: a cached route
 * commits in ~190ms.
 */
const HOLD_LIMIT_MS = 400;

/**
 * How long a prefetch needs before its route counts as ready. Below this the
 * payload is probably still in flight, and a transition would sit frozen
 * waiting for the network — which is what made the app feel stuck.
 */
const PREFETCH_LEAD_MS = 250;

/** Set while a cross-fade owns the navigation, so the CSS enter-fade stands down. */
const VT_ATTR = "data-view-transition";

/**
 * The pending cross-fade's resolver, deliberately outside React.
 *
 * A language switch changes the [locale] segment, which re-mounts this
 * component — and a resolver held in a ref would go with it, leaving the
 * transition to wait out its guard and be skipped every time. Module scope
 * survives that, so the freshly mounted tree can settle a transition the
 * previous one started.
 */
let pendingSettle: (() => void) | null = null;

/** Backstop so a navigation that never lands cannot strand the callback. */
const SETTLE_TIMEOUT_MS = 2000;

function isPlainLeftClick(event: MouseEvent): boolean {
  return (
    !event.defaultPrevented &&
    event.button === 0 &&
    !event.metaKey &&
    !event.ctrlKey &&
    !event.shiftKey &&
    !event.altKey
  );
}

/** The in-app destination of an anchor, or null if it is not one. */
function internalTarget(node: EventTarget | null): string | null {
  const anchor = node instanceof Element ? node.closest("a") : null;
  if (!anchor || anchor.target === "_blank" || anchor.hasAttribute("download")) return null;

  const href = anchor.getAttribute("href");
  if (!href || href.startsWith("#")) return null;

  let url: URL;
  try {
    url = new URL(anchor.href, window.location.href);
  } catch {
    return null;
  }
  if (url.origin !== window.location.origin) return null;
  // Same page: nothing would change, and the callback would never settle.
  if (url.pathname === window.location.pathname && url.search === window.location.search) return null;

  return url.pathname + url.search + url.hash;
}

/**
 * Wraps a navigation in a cross-fade. `warm` says the destination is already
 * loaded — without it the screen would be held waiting on the network, so a
 * cold call navigates plainly instead.
 */
export type TransitionNavigate = (
  /** `driven` is true when a cross-fade is holding the screen for this call. */
  navigate: (driven: boolean) => void,
  opts?: { warm?: boolean },
) => void;

const TransitionContext = createContext<TransitionNavigate | null>(null);

/**
 * Cross-fade a navigation this component drives itself — a language switch,
 * say, which is a router.replace and not a link click. Falls back to running
 * the navigation directly wherever the provider is absent.
 */
export function useTransitionNavigate(): TransitionNavigate {
  const fromContext = useContext(TransitionContext);
  return fromContext ?? ((navigate) => navigate(false));
}

export function ViewTransitions({ children }: { children?: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  // The new route has rendered — let the browser finish the cross-fade. This
  // also runs on mount, which is what settles a switch that re-mounted us.
  useEffect(() => {
    pendingSettle?.();
  }, [pathname]);

  // Warm a route as the pointer arrives, so the click has nothing to wait for.
  // The timestamp is what later decides whether a cross-fade is affordable.
  const warmedRef = useRef(new Map<string, number>());

  useEffect(() => {
    const warmed = warmedRef.current;
    const warm = (event: Event) => {
      const to = internalTarget(event.target);
      if (!to || warmed.has(to)) return;
      warmed.set(to, performance.now());
      router.prefetch(to);
    };

    // pointerover bubbles where pointerenter does not; focusin covers keyboards.
    document.addEventListener("pointerover", warm, true);
    document.addEventListener("focusin", warm, true);
    return () => {
      document.removeEventListener("pointerover", warm, true);
      document.removeEventListener("focusin", warm, true);
    };
  }, [router]);

  const run = useCallback<TransitionNavigate>((navigate, opts) => {
    const root = document.documentElement;
    const start = document.startViewTransition?.bind(document);
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    // Without a warm route the callback would wait on the network, and the
    // browser would hold the screen frozen for all of it. Navigate plainly
    // instead: the incoming page still fades in on its own.
    if (!start || reduced || !opts?.warm) {
      root.removeAttribute(VT_ATTR);
      navigate(false);
      return;
    }

    // Decide before the incoming page mounts, and leave it decided. Toggling
    // this while that page is on screen re-arms its CSS animation, and the
    // browser restarts it from opacity 0 — a dark blink right after the
    // cross-fade had already finished. Measured: stamp cleared at 400ms, the
    // wrapper back at opacity 0 by 415ms.
    root.setAttribute(VT_ATTR, "");

    let holdTimer = 0;
    let committed = false;

    const transition = start(
      () =>
        new Promise<void>((resolve) => {
          // Called when the route commits. Cancelling the hold here rather than
          // on transition.finished matters: the two land within a frame of each
          // other on a real connection, and a skip that wins that race
          // un-stamps a page already on screen — which re-arms its fade and
          // blinks it through transparent.
          const settle = () => {
            if (pendingSettle !== settle) return;
            pendingSettle = null;
            committed = true;
            window.clearTimeout(holdTimer);
            resolve();
          };
          pendingSettle = settle;
          window.setTimeout(settle, SETTLE_TIMEOUT_MS);
          navigate(true);
        }),
    );

    // Rather than freeze while a slow route loads, drop the snapshot and let
    // the outgoing page stay live until the new one is ready. Only reachable
    // before the route commits, so there is no incoming wrapper to disturb —
    // un-stamping simply hands that page its own fade back.
    holdTimer = window.setTimeout(() => {
      if (committed) return;
      root.removeAttribute(VT_ATTR);
      transition.skipTransition();
    }, HOLD_LIMIT_MS);
  }, []);

  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      if (!isPlainLeftClick(event)) return;
      const to = internalTarget(event.target);
      if (!to) return;

      event.preventDefault();
      const warmedAt = warmedRef.current.get(to);
      const warm = warmedAt !== undefined && performance.now() - warmedAt >= PREFETCH_LEAD_MS;
      run(() => router.push(to), { warm });
    };

    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, [router, run]);

  return <TransitionContext.Provider value={run}>{children}</TransitionContext.Provider>;
}
