"use client";

import { useEffect } from "react";

/**
 * Registers the push + offline-shell SW early so installability criteria can be met.
 * Never register in development: Next.dev chunk URLs reuse paths while contents change,
 * and cache-first `/_next/static` would serve stale JS → React hydration mismatches.
 */
export function RegisterServiceWorker() {
  useEffect(() => {
    if (typeof window === "undefined" || !("serviceWorker" in navigator)) return;

    if (process.env.NODE_ENV !== "production") {
      void navigator.serviceWorker.getRegistrations().then((regs) => {
        for (const reg of regs) void reg.unregister();
      });
      if (typeof caches !== "undefined") {
        void caches.keys().then((keys) => Promise.all(keys.map((key) => caches.delete(key))));
      }
      return;
    }

    void navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => {
      // Installability is best-effort in unsupported browsers.
    });
  }, []);
  return null;
}
