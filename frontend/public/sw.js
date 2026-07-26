/* Driver Go — push + offline shell service worker (M4-08 / M6 U-38/U-39).
 * Shell cache + thin metadata/CMS API cache (variants / categories / signs / site chrome).
 * Plus network-first cache for recently opened ticket (variant) question payloads.
 * No full offline exam / session create / answer grading — that gap remains large (U-39).
 */
const SHELL_CACHE = "dg-shell-v3";
const RUNTIME_CACHE = "dg-runtime-v3";
const META_CACHE = "dg-meta-v3";
const VARIANT_CACHE = "dg-variant-v2";
const VARIANT_CACHE_MAX = 20;
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
  /^\/(?:(uz-Latn|uz-Cyrl|ru)(?:\/(?:login|oferta|privacy|narxlar|jarimalar|support)?)?)?\/?$/;

/** Metadata / public CMS list endpoints safe to cache (not question bodies / exam payloads). */
const META_LIST_RE =
  /^\/api\/proxy\/(?:variants|me\/variants|categories|signs|site\/(?:contacts|banner|home))(?:\?|$)/;

/** Ticket/bilet detail: GET /api/proxy/variants/{n}?locale=… — grading-neutral question text. */
const VARIANT_DETAIL_RE = /^\/api\/proxy\/variants\/\d+(?:\?|$)/;

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(SHELL_CACHE)
      .then((cache) => cache.addAll(PRECACHE_URLS))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  const keep = new Set([SHELL_CACHE, RUNTIME_CACHE, META_CACHE, VARIANT_CACHE]);
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(keys.filter((key) => !keep.has(key)).map((key) => caches.delete(key)))
      )
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;

  const pathAndQuery = url.pathname + (url.search || "");

  if (VARIANT_DETAIL_RE.test(pathAndQuery)) {
    event.respondWith(networkFirstVariantDetail(req));
    return;
  }

  if (META_LIST_RE.test(pathAndQuery)) {
    event.respondWith(networkFirstMetaList(req));
    return;
  }

  if (url.pathname.startsWith("/api/") || url.pathname.startsWith("/bff/")) return;

  if (req.mode === "navigate" || req.destination === "document") {
    event.respondWith(networkFirstNavigation(req));
    return;
  }

  if (isStaticAsset(url.pathname)) {
    // Prefer network for Next hashed bundles so a new deploy never hydrates
    // against a stale cached chunk; fall back to cache when offline.
    event.respondWith(networkFirstStatic(req));
  }
});

async function networkFirstStatic(req) {
  const cache = await caches.open(RUNTIME_CACHE);
  try {
    const fresh = await fetch(req);
    if (fresh && fresh.ok) {
      void cache.put(req, fresh.clone());
    }
    return fresh;
  } catch {
    const cached = await cache.match(req);
    if (cached) return cached;
    throw new Error("offline");
  }
}

function isStaticAsset(pathname) {
  return (
    pathname.startsWith("/_next/static/") ||
    pathname === "/manifest.webmanifest" ||
    pathname === OFFLINE_URL ||
    /\.(?:js|css|svg|png|jpe?g|webp|ico|woff2?)$/i.test(pathname)
  );
}

async function networkFirstMetaList(req) {
  const cache = await caches.open(META_CACHE);
  try {
    const fresh = await fetch(req);
    if (fresh && fresh.ok) {
      void cache.put(req, fresh.clone());
    }
    return fresh;
  } catch {
    const cached = await cache.match(req);
    if (cached) return cached;
    return new Response(JSON.stringify({ error: { code: "offline", message: "content list unavailable offline" } }), {
      status: 503,
      headers: { "Content-Type": "application/json; charset=utf-8" },
    });
  }
}

async function networkFirstVariantDetail(req) {
  const cache = await caches.open(VARIANT_CACHE);
  try {
    const fresh = await fetch(req);
    if (fresh && fresh.ok) {
      await cache.put(req, fresh.clone());
      void trimVariantCache(cache);
    }
    return fresh;
  } catch {
    const cached = await cache.match(req);
    if (cached) return cached;
    return new Response(
      JSON.stringify({
        error: {
          code: "offline",
          message: "variant detail unavailable offline (open this ticket online once first)",
        },
      }),
      {
        status: 503,
        headers: { "Content-Type": "application/json; charset=utf-8" },
      }
    );
  }
}

/** Drop oldest entries when over VARIANT_CACHE_MAX (insertion order ≈ Cache.keys). */
async function trimVariantCache(cache) {
  const keys = await cache.keys();
  if (keys.length <= VARIANT_CACHE_MAX) return;
  const excess = keys.length - VARIANT_CACHE_MAX;
  for (let i = 0; i < excess; i++) {
    await cache.delete(keys[i]);
  }
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
