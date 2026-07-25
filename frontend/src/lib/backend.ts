const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8090";

export function backendFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(`${BACKEND_URL}/api/v1${path}`, init);
}

/** Root probes outside `/api/v1` (e.g. `/healthz`, `/readyz`). */
export function backendRootFetch(path: string, init?: RequestInit): Promise<Response> {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return fetch(`${BACKEND_URL}${normalized}`, init);
}

/** M3 Super Admin API (`/admin/v1/**`). */
export function backendAdminFetch(path: string, init?: RequestInit): Promise<Response> {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return fetch(`${BACKEND_URL}/admin/v1${normalized}`, init);
}
