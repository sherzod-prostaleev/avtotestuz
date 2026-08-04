import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import { setAuthCookies, clearAuthCookies, readCookie, AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";
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

function unavailableResponse(tokens?: TokenPair | null) {
  const response = NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
  if (tokens) {
    // Refresh rotation may have succeeded before the downstream request
    // failed. Preserve the rotated pair so the next retry remains usable.
    setAuthCookies(response, tokens);
  }
  return response;
}

async function forward(
  request: Request,
  path: string[],
  accessToken: string | null | undefined,
  body: string | undefined
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
  const { path } = await context.params;
  if (!safePath(path)) {
    return NextResponse.json(
      { error: { code: "invalid_path", message: "invalid proxy path" } },
      { status: 400 }
    );
  }

  let accessToken = readCookie(request, AUTH_COOKIE);
  const refreshToken = readCookie(request, REFRESH_COOKIE);

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
      return unavailableResponse();
    }
    if (newTokens) {
      accessToken = newTokens.accessToken;
    }
  }

  // Read body ONCE
  const body = request.method !== "GET" && request.method !== "HEAD" ? await request.text() : undefined;

  let backendRes: Response | undefined;

  if (accessToken || isPublicPath(path)) {
    try {
      backendRes = await forward(request, path, accessToken, body);
    } catch {
      return unavailableResponse(newTokens);
    }
  }

  // If 401 unauthorized and we haven't refreshed yet, try refresh once
  if ((!backendRes || backendRes.status === 401) && refreshToken && !newTokens) {
    try {
      newTokens = await refreshOnce(refreshToken, callBackendRefresh);
    } catch {
      return unavailableResponse();
    }
    if (newTokens) {
      accessToken = newTokens.accessToken;
      try {
        backendRes = await forward(request, path, accessToken, body);
      } catch {
        return unavailableResponse(newTokens);
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
    if (newTokens) {
      setAuthCookies(response, newTokens);
    } else if (refreshToken || accessToken) {
      clearAuthCookies(response);
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
      clearAuthCookies(response);
    } else if (newTokens) {
      setAuthCookies(response, newTokens);
    }
    return response;
  }

  let data: unknown;
  const contentType = backendRes.headers.get("content-type") ?? "";
  if (
    contentType.includes("text/csv") ||
    contentType.includes("application/octet-stream")
  ) {
    const buf = await backendRes.arrayBuffer();
    const response = new NextResponse(buf, { status: backendRes.status });
    response.headers.set("Content-Type", contentType);
    const cd = backendRes.headers.get("content-disposition");
    if (cd) response.headers.set("Content-Disposition", cd);
    if (newTokens) {
      setAuthCookies(response, newTokens);
    }
    return response;
  }
  try {
    data = await readBackendJson(backendRes);
  } catch {
    return unavailableResponse(newTokens);
  }
  const response = NextResponse.json(data, { status: backendRes.status });

  if (newTokens) {
    setAuthCookies(response, newTokens);
  }

  return response;
}

export const GET = handle;
export const POST = handle;
export const PATCH = handle;
export const DELETE = handle;
