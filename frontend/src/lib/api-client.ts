export class ApiError extends Error {
  code: string;
  status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
  }
}

function deviceHeaders(): Record<string, string> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  return headers;
}

async function handleResponse<T>(res: Response): Promise<T> {
  // Parsed defensively, not because the API misbehaves but because not every
  // response comes from the API. A classroom PC reaches it through the local
  // station agent, whose reverse proxy answers a plain-text "Bad Gateway" when
  // the upstream is unreachable, and nginx and Cloudflare answer with HTML.
  // Parsing unconditionally threw a SyntaxError on all three and destroyed the
  // status code with it, so the kiosk could not tell "still starting up" from
  // "this PC was revoked" and showed the same endless spinner for both.
  let data: unknown = null;
  try {
    data = await res.json();
  } catch {
    data = null;
  }
  const envelope = data as { error?: { code?: string; message?: string }; data?: unknown } | null;
  if (!res.ok) {
    const errCode = envelope?.error?.code ?? fallbackCode(res.status);
    const errMessage = envelope?.error?.message ?? `HTTP ${res.status}`;
    throw new ApiError(errMessage, errCode, res.status);
  }
  if (envelope === null) {
    throw new ApiError("Unreadable response body", "bad_response", res.status);
  }
  return (envelope.data !== undefined ? envelope.data : envelope) as T;
}

// fallbackCode names the failures that arrive with no JSON envelope, so
// callers can still branch on something more specific than "unknown_error".
//
// 502/504 report `network_error` — the same code the BFF returns when it cannot
// reach Go — because they are the same thing to the person looking at the
// screen: the chain between them and the server is broken, their work is saved,
// retrying is the answer. They used to report `upstream_unreachable`, a second
// code for that one condition, and nothing anywhere consumed it: every screen
// branches on `network_error`, so a 502 from nginx or Cloudflare fell through to
// the generic "something went wrong" instead of the message that tells the
// learner their session survived. One condition, one code, so the two cannot
// drift apart again. Anything that needs to tell a bad gateway from a BFF-level
// failure reads ApiError.status, which is unchanged.
//
// 503 stays its own code: that is the station agent failing closed while it has
// no token — see backend/station/internal/proxy — and the kiosk screen shows a
// genuinely different thing for it.
function fallbackCode(status: number): string {
  if (status === 503) return "station_offline";
  if (status === 502 || status === 504) return "network_error";
  return "unknown_error";
}

export async function apiGet<T>(path: string, init?: { signal?: AbortSignal }): Promise<T> {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  const res = await fetch(`/api/proxy/${cleanPath}`, {
    method: "GET",
    headers: deviceHeaders(),
    signal: init?.signal,
  });
  return handleResponse<T>(res);
}

export async function apiPost<T, B = unknown>(path: string, body?: B): Promise<T> {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  const res = await fetch(`/api/proxy/${cleanPath}`, {
    method: "POST",
    headers: deviceHeaders(),
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  return handleResponse<T>(res);
}

/** Multipart upload (no JSON Content-Type — browser sets the boundary). */
export async function apiPostForm<T>(path: string, form: FormData): Promise<T> {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  const res = await fetch(`/api/proxy/${cleanPath}`, {
    method: "POST",
    body: form,
  });
  return handleResponse<T>(res);
}

export async function apiPatch<T, B = unknown>(path: string, body?: B): Promise<T> {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  const res = await fetch(`/api/proxy/${cleanPath}`, {
    method: "PATCH",
    headers: deviceHeaders(),
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  return handleResponse<T>(res);
}

export async function apiDelete<T, B = unknown>(path: string, body?: B): Promise<T> {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  const res = await fetch(`/api/proxy/${cleanPath}`, {
    method: "DELETE",
    headers: deviceHeaders(),
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  return handleResponse<T>(res);
}
