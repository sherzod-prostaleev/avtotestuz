/* Driver Go — push + offline shell service worker (M4-08 / M6 U-39).
 * Shell cache only: static assets + network-first navigations with offline.html
 * fallback. No full offline exam / question catalog sync.
 */
const SHELL_CACHE = "dg-shell-v1";
const RUNTIME_CACHE = "dg-runtime-v1";
const OFFLINE_URL = "/offline.html";
const PRECACHE_URLS = [
  OFFLINE_URL,
  "/manifest.webmanifest",
  "/icon.svg",
  "/logo.svg",
  "/logo-512.png",
  "/apple-touch-icon.png",
  "/favicon.ico",
];

/** Public / shell paths worth keeping after a successful visit (locale-aware). */
const SHELL_PATH_RE =
  /^\/(?:(uz-Latn|uz-Cyrl|ru)(?:\/(?:login|oferta|privacy|narxlar|jarimalar)?)?)?\/?$/;

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(SHELL_CACHE)
      .then((cache) => cache.addAll(PRECACHE_URLS))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter((key) => key !== SHELL_CACHE && key !== RUNTIME_CACHE)
            .map((key) => caches.delete(key))
        )
      )
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith("/api/") || url.pathname.startsWith("/bff/")) return;

  if (req.mode === "navigate" || req.destination === "document") {
    event.respondWith(networkFirstNavigation(req));
    return;
  }

  if (isStaticAsset(url.pathname)) {
    event.respondWith(cacheFirst(req));
  }
});

function isStaticAsset(pathname) {
  return (
    pathname.startsWith("/_next/static/") ||
    pathname === "/manifest.webmanifest" ||
    pathname === OFFLINE_URL ||
    /\.(?:js|css|svg|png|jpe?g|webp|ico|woff2?)$/i.test(pathname)
  );
}

async function networkFirstNavigation(req) {
  try {
    const fresh = await fetch(req);
    if (fresh && fresh.ok) {
      const url = new URL(req.url);
      if (SHELL_PATH_RE.test(url.pathname)) {
        const cache = await caches.open(RUNTIME_CACHE);
        void cache.put(req, fresh.clone());
      }
    }
    return fresh;
  } catch {
    const cached = await caches.match(req);
    if (cached) return cached;
    const offline = await caches.match(OFFLINE_URL);
    if (offline) return offline;
    return new Response("Offline", {
      status: 503,
      statusText: "Service Unavailable",
      headers: { "Content-Type": "text/plain; charset=utf-8" },
    });
  }
}

async function cacheFirst(req) {
  const cached = await caches.match(req);
  if (cached) return cached;
  try {
    const fresh = await fetch(req);
    if (fresh && fresh.ok) {
      const cache = await caches.open(RUNTIME_CACHE);
      void cache.put(req, fresh.clone());
    }
    return fresh;
  } catch {
    throw new Error("offline");
  }
}

self.addEventListener("push", (event) => {
  let data = { title: "Driver Go", body: "", url: "/dashboard" };
  try {
    if (event.data) {
      data = { ...data, ...event.data.json() };
    }
  } catch {
    try {
      data.body = event.data ? event.data.text() : "";
    } catch {
      /* ignore */
    }
  }
  const title = data.title || "Driver Go";
  const options = {
    body: data.body || "",
    data: { url: data.url || "/dashboard", ...(data.data || {}) },
    icon: "/icon.svg",
    badge: "/icon.svg",
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const raw = (event.notification.data && event.notification.data.url) || "/dashboard";
  const target = raw.startsWith("http") ? raw : new URL(raw, self.location.origin).href;
  event.waitUntil(
    clients.matchAll({ type: "window", includeUncontrolled: true }).then((list) => {
      for (const client of list) {
        if ("focus" in client) {
          client.navigate(target);
          return client.focus();
        }
      }
      if (clients.openWindow) {
        return clients.openWindow(target);
      }
      return undefined;
    })
  );
});
