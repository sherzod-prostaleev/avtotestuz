import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { setAuthCookies, clearAuthCookies, readCookie, AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { refreshOnce } from "@/lib/refresh-lock";
import { callBackendRefresh } from "@/lib/backend-refresh";

const publicPaths = new Set(["signs", "categories", "demo"]);

function isPublicPath(path: string[]): boolean {
  if (path.length === 0) return false;
  return publicPaths.has(path[0]);
}

async function forward(
  request: Request,
  path: string[],
  accessToken: string | null | undefined,
  body: string | undefined
): Promise<Response> {
  const url = new URL(request.url);
  const targetPath = `/${path.join("/")}${url.search}`;
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

async function handle(request: Request, context: { params: { path: string[] } }) {
  const { path } = context.params;
  let accessToken = readCookie(request, AUTH_COOKIE);
  const refreshToken = readCookie(request, REFRESH_COOKIE);

  // If endpoint is protected and no tokens exist, return 401 immediately
  if (!accessToken && !refreshToken && !isPublicPath(path)) {
    return NextResponse.json({ error: { code: "unauthorized", message: "no access token" } }, { status: 401 });
  }

  let newTokens: { accessToken: string; refreshToken: string } | null = null;

  // If access token is missing but refresh token exists, try refreshing first
  if (!accessToken && refreshToken) {
    newTokens = await refreshOnce(refreshToken, callBackendRefresh);
    if (newTokens) {
      accessToken = newTokens.accessToken;
    }
  }

  // Read body ONCE
  const body = request.method !== "GET" && request.method !== "HEAD" ? await request.text() : undefined;

  let backendRes: Response | undefined;
  
  if (accessToken || isPublicPath(path)) {
    backendRes = await forward(request, path, accessToken, body);
  }

  // If 401 unauthorized and we haven't refreshed yet, try refresh once
  if ((!backendRes || backendRes.status === 401) && refreshToken && !newTokens) {
    newTokens = await refreshOnce(refreshToken, callBackendRefresh);
    if (newTokens) {
      accessToken = newTokens.accessToken;
      backendRes = await forward(request, path, accessToken, body);
    }
  }

  if (!backendRes || backendRes.status === 401) {
    const data = backendRes
      ? await backendRes.json()
      : { error: { code: "unauthorized", message: "session expired" } };
    const response = NextResponse.json(data, { status: 401 });
    if (refreshToken || accessToken) {
      clearAuthCookies(response);
    }
    return response;
  }

  const data = await backendRes.json();
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
