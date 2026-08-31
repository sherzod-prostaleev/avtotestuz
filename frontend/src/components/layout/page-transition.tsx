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

export function PageTransition({
  children,
  className = "w-full",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const pathname = usePathname();

  return (
    <AnimatePresence mode="popLayout" initial={false}>
      <motion.div
        key={pathname}
        initial={{ opacity: 0.25, scale: 0.985, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 1.01, y: -8 }}
        transition={{
          duration: 0.40,
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
