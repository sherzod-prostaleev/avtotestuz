/** Convert a URL-safe base64 VAPID public key to a Uint8Array for PushManager. */
export function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const out = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i += 1) {
    out[i] = raw.charCodeAt(i);
  }
  return out;
}

export function pushSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

export async function ensurePushServiceWorker(): Promise<ServiceWorkerRegistration> {
  const reg = await navigator.serviceWorker.register("/sw.js", { scope: "/" });
  await navigator.serviceWorker.ready;
  return reg;
}

export function subscriptionToJSON(sub: PushSubscription): {
  endpoint: string;
  keys: { p256dh: string; auth: string };
} {
  const json = sub.toJSON();
  const p256dh = json.keys?.p256dh ?? "";
  const auth = json.keys?.auth ?? "";
  return {
    endpoint: sub.endpoint,
    keys: { p256dh, auth },
  };
}
