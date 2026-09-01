"use client";

import { usePathname } from "next/navigation";
import React from "react";

/**
 * The locale prefix is not a route change. Stripping it keeps the key stable
 * while switching language, so the tree is never torn down and the text swaps
 * in place — the behaviour 72b9aee introduced, kept here.
 */
function getRouteKey(pathname: string | null): string {
  if (!pathname) return "";
  return pathname.replace(/^\/(uz-Latn|uz-Cyrl|ru)(\/|$)/, "/") || "/";
}

/**
 * Enter-only page fade, done in CSS.
 *
 * This deliberately does NOT use AnimatePresence. The Framer version
 * (9a37779) cost the platform its responsiveness in three ways, all measured
 * in production:
 *
 *  - `mode="popLayout"` kept the outgoing page mounted and absolutely
 *    positioned while the next one rendered, and measured the whole subtree to
 *    do it. On /signs that is 285 cards held in the DOM twice.
 *  - A `FrozenRoute` provider pinned `LayoutRouterContext` to its first value
 *    for the life of the mounted route, so the App Router's tree and the DOM
 *    drifted apart. The route subtree — head metadata included — remounted
 *    over and over: one browser refetched /manifest.webmanifest 170 times
 *    against 16 real page loads, and every <Link prefetch> under it
 *    re-registered each time, which is what turned prefetching into a storm.
 *  - `will-change: transform, opacity` sat on the wrapper permanently, holding
 *    the entire page on its own compositor layer long after any animation.
 *
 * A keyed <div> with a CSS animation gives the same thing on screen for no JS,
 * no measurement, and no router interference. Opacity and transform are
 * composited, so no will-change hint is needed.
 */
export function PageTransition({
  children,
  className = "w-full",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const routeKey = getRouteKey(usePathname());

  return (
    <div key={routeKey} className={`page-enter ${className}`}>
      {children}
    </div>
  );
}
