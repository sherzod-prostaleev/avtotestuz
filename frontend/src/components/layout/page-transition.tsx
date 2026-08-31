"use client";

import { AnimatePresence, motion } from "motion/react";
import { usePathname } from "next/navigation";
import { LayoutRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";
import React, { useContext, useRef } from "react";

// Apple iOS fluid deceleration curve
const iosEasing = [0.16, 1, 0.3, 1] as const;

/**
 * Freezes the outgoing route's React component tree during the exit animation
 * so that AnimatePresence mode="popLayout" can smoothly animate the old page fading out
 * while revealing the incoming page.
 */
function FrozenRoute({ children }: { children: React.ReactNode }) {
  const context = useContext(LayoutRouterContext ?? {});
  const frozen = useRef(context).current;

  if (!LayoutRouterContext) {
    return <>{children}</>;
  }

  return (
    <LayoutRouterContext.Provider value={frozen}>
      {children}
    </LayoutRouterContext.Provider>
  );
}

function getRouteKey(pathname: string | null): string {
  if (!pathname) return "";
  return pathname.replace(/^\/(uz-Latn|uz-Cyrl|ru)(\/|$)/, "/") || "/";
}

export function PageTransition({
  children,
  className = "w-full",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const pathname = usePathname();
  const routeKey = getRouteKey(pathname);

  return (
    <AnimatePresence mode="popLayout" initial={false}>
      <motion.div
        key={routeKey}
        initial={{ opacity: 0.35, scale: 0.99, y: 6 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 1.005, y: -6 }}
        transition={{
          duration: 0.22,
          ease: iosEasing,
        }}
        className={className}
        style={{ willChange: "transform, opacity" }}
      >
        <FrozenRoute>{children}</FrozenRoute>
      </motion.div>
    </AnimatePresence>
  );
}

export { PageTransition as PageFade };

