"use client";

import { useEffect } from "react";
import { initSentry } from "@/lib/sentry";

/** Client-only Sentry bootstrap; no-ops when NEXT_PUBLIC_SENTRY_DSN is empty. */
export function InitSentry() {
  useEffect(() => {
    void initSentry();
  }, []);
  return null;
}
