const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8090";

export function backendFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(`${BACKEND_URL}/api/v1${path}`, init);
}
