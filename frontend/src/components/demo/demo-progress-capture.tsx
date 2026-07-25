"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { migrateDemoProgressOnLogin } from "@/lib/demo-progress-storage";

/**
 * Retries guest demo → account migration for signed-in users.
 *
 * Verify already migrates on OTP success; this covers the case where that
 * attempt failed transiently (offline / session not ready) so the next
 * authenticated navigation can finish bookmarking incorrect demo answers.
 */
export function DemoProgressCapture() {
  const pathname = usePathname();

  useEffect(() => {
    void migrateDemoProgressOnLogin();
  }, [pathname]);

  return null;
}
