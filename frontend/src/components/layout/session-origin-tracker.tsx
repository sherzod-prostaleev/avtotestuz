"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { rememberSessionOrigin } from "@/lib/session-origin";

/**
 * Records the last hub the learner stood on, so a session's exit button can
 * put them back there. Mounted once per shell — the learner app and the kiosk
 * — which together cover every screen that links into a session. Renders
 * nothing; see @/lib/session-origin for why this is a shell-level tracker
 * rather than a `from=` param on each link.
 */
export function SessionOriginTracker() {
  const pathname = usePathname();

  useEffect(() => {
    if (pathname) rememberSessionOrigin(pathname);
  }, [pathname]);

  return null;
}
