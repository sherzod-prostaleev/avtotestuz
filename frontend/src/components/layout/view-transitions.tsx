"use client";

import { useEffect, useRef } from "react";
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

export function ViewTransitions() {
  const router = useRouter();
  const pathname = usePathname();
  const settleRef = useRef<(() => void) | null>(null);

  // The new route has rendered — let the browser finish the cross-fade.
  useEffect(() => {
    settleRef.current?.();
    settleRef.current = null;
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

  useEffect(() => {
    const start = document.startViewTransition?.bind(document);
    if (!start) return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

    const onClick = (event: MouseEvent) => {
      if (!isPlainLeftClick(event)) return;
      const to = internalTarget(event.target);
      if (!to) return;

      const root = document.documentElement;

      // Without a warm route the callback would wait on the network, and the
      // browser would hold the screen frozen for all of it. Navigate plainly
      // instead: the incoming page still fades in on its own.
      const warmedAt = warmedRef.current.get(to);
      if (warmedAt === undefined || performance.now() - warmedAt < PREFETCH_LEAD_MS) {
        event.preventDefault();
        root.removeAttribute(VT_ATTR);
        router.push(to);
        return;
      }

      event.preventDefault();

      // Decide before the incoming page mounts, and leave it decided. Toggling
      // this while that page is on screen re-arms its CSS animation, and the
      // browser restarts it from opacity 0 — a dark blink right after the
      // cross-fade had already finished. Measured: stamp cleared at 400ms, the
      // wrapper back at opacity 0 by 415ms.
      root.setAttribute(VT_ATTR, "");

      const transition = start(
        () =>
          new Promise<void>((resolve) => {
            const done = () => {
              if (settleRef.current !== resolve) return;
              settleRef.current = null;
              resolve();
            };
            settleRef.current = resolve;
            window.setTimeout(done, SETTLE_TIMEOUT_MS);
            router.push(to);
          }),
      );

      // Rather than freeze while a slow route loads, drop the snapshot and let
      // the outgoing page stay live until the new one is ready. The route has
      // not committed at this point, so the incoming wrapper does not exist yet
      // and un-stamping cannot restart anything — it just hands that page its
      // own fade back.
      const holdTimer = window.setTimeout(() => {
        root.removeAttribute(VT_ATTR);
        transition.skipTransition();
      }, HOLD_LIMIT_MS);
      transition.finished.finally(() => window.clearTimeout(holdTimer));
    };

    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, [router]);

  return null;
}
