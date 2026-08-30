"use client";

import { useEffect, useRef } from "react";
import { usePathname, useRouter } from "next/navigation";

/**
 * Cross-fades one page into the next.
 *
 * The enter-only CSS fade made the old page vanish and the new one assemble
 * from nothing, which does not read as a transition. A real cross-fade needs
 * the browser to hold a picture of the outgoing page while the incoming one
 * appears over it — that is what document.startViewTransition does.
 *
 * Next cannot drive it for us here: its experimental.viewTransition flag and
 * Link's transitionTypes both go through React.addTransitionType, which only
 * exists on React's experimental channel. So this listens for link clicks and
 * drives the API itself.
 *
 * startViewTransition takes a callback that must not settle until the new DOM
 * is committed. router.push is asynchronous, so the promise is resolved by the
 * effect below once the pathname has actually changed — with a timeout so a
 * navigation that never lands cannot leave the page frozen under a snapshot.
 *
 * Navigations made through router.push elsewhere in the app, and browser
 * back/forward, are not intercepted; they simply navigate without the
 * cross-fade, exactly as before.
 */

/** Longer than the CSS cross-fade, short enough not to strand the page. */
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

export function ViewTransitions() {
  const router = useRouter();
  const pathname = usePathname();
  const settleRef = useRef<(() => void) | null>(null);

  // The new route has rendered — let the browser finish the cross-fade.
  useEffect(() => {
    settleRef.current?.();
    settleRef.current = null;
  }, [pathname]);

  useEffect(() => {
    const start = document.startViewTransition?.bind(document);
    if (!start) return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

    const onClick = (event: MouseEvent) => {
      if (!isPlainLeftClick(event)) return;

      const target = event.target;
      const anchor = target instanceof Element ? target.closest("a") : null;
      if (!anchor || anchor.target === "_blank" || anchor.hasAttribute("download")) return;

      const href = anchor.getAttribute("href");
      if (!href || href.startsWith("#")) return;

      let url: URL;
      try {
        url = new URL(anchor.href, window.location.href);
      } catch {
        return;
      }
      if (url.origin !== window.location.origin) return;
      // Same page: nothing would change, and the promise would never settle.
      if (url.pathname === window.location.pathname && url.search === window.location.search) return;

      event.preventDefault();

      start(
        () =>
          new Promise<void>((resolve) => {
            const done = () => {
              if (settleRef.current !== resolve) return;
              settleRef.current = null;
              resolve();
            };
            settleRef.current = resolve;
            window.setTimeout(done, SETTLE_TIMEOUT_MS);
            router.push(url.pathname + url.search + url.hash);
          }),
      );
    };

    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, [router]);

  return null;
}
