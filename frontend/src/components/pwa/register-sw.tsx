"use client";

import { useEffect } from "react";

/**
 * isClassroomKiosk reports whether this page is being served by the local
 * station agent on a driving school's classroom PC rather than by drivergo.uz.
 *
 * 127.0.0.1 counts as a secure context, so the service worker registers there
 * happily and then owns every navigation on that origin. When the agent stops
 * — someone ended the process, an update restarted it, it never started — the
 * SW catches the failed fetch and renders its own offline.html, which says
 * "Internet aloqasi yo'q" ("no internet connection") next to a retry button
 * that shows the same page forever. On a classroom PC that sentence is simply
 * false: the internet is usually fine and the local agent is what is missing,
 * so the school is sent to check the wrong thing entirely. The kiosk gets no
 * service worker at all — it has no offline story to protect (the agent must
 * be reachable for anything to work) and everything it needs is one hop away
 * on localhost.
 */
function isClassroomKiosk(): boolean {
  const { hostname, pathname } = window.location;
  return (
    (hostname === "127.0.0.1" || hostname === "localhost") && pathname.split("/").includes("station")
  );
}

/**
 * Registers the push + offline-shell SW early so installability criteria can be met.
 * Never register in development: Next.dev chunk URLs reuse paths while contents change,
 * and cache-first `/_next/static` would serve stale JS → React hydration mismatches.
 */
export function RegisterServiceWorker() {
  useEffect(() => {
    if (typeof window === "undefined" || !("serviceWorker" in navigator)) return;

    if (process.env.NODE_ENV !== "production" || isClassroomKiosk()) {
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
