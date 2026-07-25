/* Driver Go — push-only service worker (M4-08 / U-11).
 * Intentionally no offline precache (that is M6 / U-38–39).
 */
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
