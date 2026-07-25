"use client";

import { useEffect } from "react";

/** Registers the push-capable SW early so installability criteria can be met. */
export function RegisterServiceWorker() {
  useEffect(() => {
    if (typeof window === "undefined" || !("serviceWorker" in navigator)) return;
    void navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => {
      // Installability is best-effort in unsupported browsers.
    });
  }, []);
  return null;
}
