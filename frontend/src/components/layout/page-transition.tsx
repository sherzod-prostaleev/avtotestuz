"use client";

import { usePathname } from "next/navigation";
import React from "react";

/**
 * The locale prefix is not a route change. Stripping it keeps the key stable
 * while switching language, so the tree is never torn down and the text swaps
 * in place.
 */
function getRouteKey(pathname: string | null): string {
  if (!pathname) return "";
  return pathname.replace(/^\/(uz-Latn|uz-Cyrl|ru)(\/|$)/, "/") || "/";
}

/**
 * Clean, comfortable enter-only page fade via CSS.
 *
 * Zero double-rendering, zero eye fatigue, instant snappy navigation
 * with a soft, natural 220ms fade-in.
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
