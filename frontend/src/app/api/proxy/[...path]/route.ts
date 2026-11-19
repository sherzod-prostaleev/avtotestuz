import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import {
  setAuthCookies,
  clearAuthCookies,
  cookieModeFor,
  readCookie,
  AUTH_COOKIE,
  REFRESH_COOKIE,
  type CookieMode,
} from "@/lib/auth-cookies";
import { rejectCrossSite } from "@/lib/same-origin";
import { refreshOnce } from "@/lib/refresh-lock";
import { callBackendRefresh } from "@/lib/backend-refresh";

type TokenPair = { accessToken: string; refreshToken: string };

// Content list/detail under variants|signs|categories is grading-neutral on the API.
const publicPaths = new Set(["signs", "categories", "variants", "demo", "tariffs"]);

function isPublicPath(path: string[]): boolean {
  if (path.length === 0) return false;
  if (publicPaths.has(path[0])) return true;
  // GET /billing/providers — kill-switch status for checkout UI
  if (path[0] === "billing" && path[1] === "providers") return true;
  // GET /grand-mock/certificates/{code} — public certificate verification page
  if (path[0] === "grand-mock" && path[1] === "certificates") return true;
  // GET /site/contacts|banner|home|legal — public CMS chrome (no secrets)
  if (
    path[0] === "site" &&
    (path[1] === "contacts" || path[1] === "banner" || path[1] === "home" || path[1] === "legal")
  ) {
    return true;
  }
  // GET /flags — public feature-flag snapshot
  return path[0] === "flags";
}

function safePath(path: string[]): string | null {
  if (path.length === 0) return null;

  const unsafe = path.some(
    (segment) => !segment || segment === "." || segment === ".." || segment.includes("/") || segment.includes("\\")
  );
  return unsafe ? null : path.map(encodeURIComponent).join("/");
}

function unavailableResponse(mode: CookieMode, tokens?: TokenPair | null) {
  const response = NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
  if (tokens) {
    // Refresh rotation may have succeeded before the downstream request
    // failed. Preserve the rotated pair so the next retry remains usable.
    setAuthCookies(response, tokens, mode);
  }
  return response;
}

/**
 * POST /me/password/set ends every other session of the profile and keeps the
 * caller's — which the backend can only tell apart by its refresh token. That
 * lives in our HttpOnly cookie, so it is named here, server to server, and
 * for this one endpoint only.
 */
const REFRESH_TOKEN_HEADER = "X-Avtotest-Refresh-Token";

function namesOwnSession(request: Request, path: string[]): boolean {
  return request.method === "POST" && path.length === 3 && path[0] === "me" && path[1] === "password" && path[2] === "set";
}

function isBinaryRequestContentType(contentType: string): boolean {
  const ct = contentType.toLowerCase();
  return ct.startsWith("multipart/form-data") || ct.startsWith("application/octet-stream");
}

function isBinaryResponse(res: Response): boolean {
  const ct = (res.headers.get("content-type") ?? "").toLowerCase();
  const hasDisposition = Boolean(res.headers.get("content-disposition"));
  return (
    ct.includes("text/csv") ||
    ct.includes("application/octet-stream") ||
    ct.startsWith("image/") ||
    ct.startsWith("application/pdf") ||
    ct.startsWith("application/zip") ||
    ct.startsWith("application/msword") ||
    ct.includes("officedocument") ||
    (hasDisposition && ct.startsWith("text/plain"))
  );
}

function extractErrorCode(data: unknown): string {
  if (
    data &&
    typeof data === "object" &&
    "error" in data &&
    data.error &&
    typeof data.error === "object" &&
    "code" in data.error
  ) {
    return String((data.error as { code?: unknown }).code ?? "");
  }
  return "";
}

async function peekErrorCode(res: Response | undefined): Promise<string> {
  if (!res) return "";
  try {
    return extractErrorCode(await readBackendJson(res.clone()));
  } catch {
    return "";
  }
}

async function forward(
  request: Request,
  path: string[],
  accessToken: string | null | undefined,
  body: string | ArrayBuffer | undefined,
  refreshToken?: string | null
): Promise<Response> {
  const url = new URL(request.url);
  const encodedPath = safePath(path);
  if (!encodedPath) {
    throw new TypeError("invalid proxy path");
  }
  const targetPath = `/${encodedPath}${url.search}`;
  const headers: Record<string, string> = {
    "Content-Type": request.headers.get("content-type") ?? "application/json",
  };
  if (accessToken) {
    headers["Authorization"] = `Bearer ${accessToken}`;
  }
  if (refreshToken && namesOwnSession(request, path)) {
    headers[REFRESH_TOKEN_HEADER] = refreshToken;
  }
  const init: RequestInit = {
    method: request.method,
    headers,
  };
  if (body !== undefined) {
    init.body = body;
  }
  return backendFetch(targetPath, init);
}

async function handle(request: Request, context: { params: Promise<{ path: string[] }> }) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;
  // A Telegram session's cookies are Partitioned; rotating or clearing them
  // with site attributes would leave the old partitioned pair in place.
  const mode = cookieModeFor(request);

  const { path } = await context.params;
  if (!safePath(path)) {
    return NextResponse.json(
      { error: { code: "invalid_path", message: "invalid proxy path" } },
      { status: 400 }
    );
  }

  let accessToken = readCookie(request, AUTH_COOKIE);
  const refreshToken = readCookie(request, REFRESH_COOKIE);

  // Anonymous telemetry: events without a profile session are acknowledged
  // without spamming 401 errors into the browser console.
  if (path[0] === "events" && !accessToken && !refreshToken) {
    return NextResponse.json({ data: { ok: true, count: 0 } }, { status: 200 });
  }

  // If endpoint is protected and no tokens exist, return 401 immediately
  if (!accessToken && !refreshToken && !isPublicPath(path)) {
    return NextResponse.json({ error: { code: "unauthorized", message: "no access token" } }, { status: 401 });
  }

  let newTokens: TokenPair | null = null;

  // If access token is missing but refresh token exists, try refreshing first
  if (!accessToken && refreshToken) {
    try {
      newTokens = await refreshOnce(refreshToken, callBackendRefresh);
    } catch {
      return unavailableResponse(mode);
    }
    if (newTokens) {
      accessToken = newTokens.accessToken;
    }
  }

  // Read body ONCE. Binary multipart uploads (e.g. support chat attachments)
  // must stay raw bytes so non-UTF-8 image/file headers are not corrupted.
  const reqContentType = request.headers.get("content-type") ?? "";
  const body: string | ArrayBuffer | undefined =
    request.method !== "GET" && request.method !== "HEAD"
      ? isBinaryRequestContentType(reqContentType)
        ? await request.arrayBuffer()
        : await request.text()
      : undefined;

  let backendRes: Response | undefined;

  if (accessToken || isPublicPath(path)) {
    try {
      backendRes = await forward(request, path, accessToken, body, newTokens?.refreshToken ?? refreshToken);
    } catch {
      return unavailableResponse(mode, newTokens);
    }
  }

  const initialErrCode = backendRes?.status === 401 ? await peekErrorCode(backendRes) : "";
  const isDomainPasswordError = initialErrCode === "invalid_current_password";

  // If 401 unauthorized (and not a domain-level password mismatch) and we haven't refreshed yet, try refresh once
  if ((!backendRes || (backendRes.status === 401 && !isDomainPasswordError)) && refreshToken && !newTokens) {
    try {
      newTokens = await refreshOnce(refreshToken, callBackendRefresh);
    } catch {
      return unavailableResponse(mode);
    }
    if (newTokens) {
      accessToken = newTokens.accessToken;
      try {
        backendRes = await forward(request, path, accessToken, body, newTokens?.refreshToken ?? refreshToken);
      } catch {
        return unavailableResponse(mode, newTokens);
      }
    }
  }

  if (!backendRes || backendRes.status === 401) {
    let data: unknown = { error: { code: "unauthorized", message: "session expired" } };
    if (backendRes) {
      try {
        data = await readBackendJson(backendRes);
      } catch {
        // The status is authoritative even if an upstream proxy stripped the
        // body. Never expose parser details to the browser.
      }
    }
    const response = NextResponse.json(data, { status: 401 });
    // Critical: after a successful rotation, NEVER clear cookies on a lingering
    // endpoint 401. Dashboard/tickets fire many parallel /api/proxy calls near
    // AT expiry; one sibling may setAuthCookies while another clearAuthCookies
    // — last Set-Cookie wins and intermittently logs the user out.
    // Likewise, a domain-level `invalid_current_password` 401 on POST /me/password
    // means the caller's session is valid and must NOT be cleared.
    if (newTokens) {
      setAuthCookies(response, newTokens, mode);
    } else if (!isDomainPasswordError && extractErrorCode(data) !== "invalid_current_password" && (refreshToken || accessToken)) {
      clearAuthCookies(response, mode);
    }
    return response;
  }

  // Admin block: drop cookies so the learner UI cannot keep calling with a live AT.
  if (backendRes.status === 403) {
    let data: unknown = { error: { code: "forbidden", message: "forbidden" } };
    try {
      data = await readBackendJson(backendRes);
    } catch {
      /* status wins */
    }
    const code =
      data &&
      typeof data === "object" &&
      "error" in data &&
      data.error &&
      typeof data.error === "object" &&
      "code" in data.error
        ? String((data.error as { code?: unknown }).code ?? "")
        : "";
    const response = NextResponse.json(data, { status: 403 });
    if (code === "account_blocked" && (refreshToken || accessToken)) {
      clearAuthCookies(response, mode);
    } else if (newTokens) {
      setAuthCookies(response, newTokens, mode);
    }
    return response;
  }

  let data: unknown;
  const contentType = backendRes.headers.get("content-type") ?? "";
  if (backendRes.status === 204 || backendRes.status === 205) {
    const response = new NextResponse(null, { status: backendRes.status });
    if (newTokens) {
      setAuthCookies(response, newTokens, mode);
    }
    return response;
  }
  if (isBinaryResponse(backendRes)) {
    const buf = await backendRes.arrayBuffer();
    const response = new NextResponse(buf, { status: backendRes.status });
    response.headers.set("Content-Type", contentType);
    const cd = backendRes.headers.get("content-disposition");
    if (cd) response.headers.set("Content-Disposition", cd);
    const cc = backendRes.headers.get("cache-control");
    if (cc) response.headers.set("Cache-Control", cc);
    response.headers.set("X-Content-Type-Options", "nosniff");
    if (newTokens) {
      setAuthCookies(response, newTokens, mode);
    }
    return response;
  }
  try {
    data = await readBackendJson(backendRes);
  } catch {
    return unavailableResponse(mode, newTokens);
  }
  const response = NextResponse.json(data, { status: backendRes.status });
  applyPublicCacheHeaders(request, path, backendRes, response, newTokens);

  if (newTokens) {
    setAuthCookies(response, newTokens, mode);
  }

  return response;
}

/** Honour Go Cache-Control on public GETs so CF/nginx can cache CMS lists. */
function applyPublicCacheHeaders(
  request: Request,
  path: string[],
  backendRes: Response,
  response: NextResponse,
  newTokens: TokenPair | null,
) {
  if (request.method !== "GET" || newTokens || !isPublicPath(path) || backendRes.status !== 200) {
    return;
  }
  const cacheControl = backendRes.headers.get("cache-control");
  if (!cacheControl) return;
  response.headers.set("Cache-Control", cacheControl);
}

export const GET = handle;
export const POST = handle;
export const PATCH = handle;
export const DELETE = handle;
