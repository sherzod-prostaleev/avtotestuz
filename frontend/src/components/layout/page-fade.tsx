"use client";

import React from "react";

export { PageTransition } from "./page-transition";

/**
 * Pass-through wrapper so existing route template re-exports do not double-wrap
 * or conflict with persistent layout transitions.
 */
export function PageFade({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
