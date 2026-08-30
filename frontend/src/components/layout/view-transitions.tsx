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
 * How long the screen may stay frozen under a snapshot. Past this the
 * transition is worth less than the responsiveness it costs.
 */
const HOLD_LIMIT_MS = 120;

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
  useEffect(() => {
    const warmed = new Set<string>();
    const warm = (event: Event) => {
      const to = internalTarget(event.target);
      if (!to || warmed.has(to)) return;
      warmed.add(to);
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

      event.preventDefault();

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
      // the outgoing page stay live until the new one is ready.
      const holdTimer = window.setTimeout(() => transition.skipTransition(), HOLD_LIMIT_MS);

      // Mark the document only once the cross-fade is genuinely running, so the
      // CSS enter-fade stands down for it. A skipped transition never sets the
      // mark and the incoming page keeps its own fade — the reader always gets
      // one animation, never two and never none.
      const root = document.documentElement;
      transition.ready.then(
        () => root.setAttribute("data-view-transition", ""),
        () => {},
      );
      transition.finished.finally(() => {
        window.clearTimeout(holdTimer);
        root.removeAttribute("data-view-transition");
      });
    };

    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, [router]);

  return null;
}
