import { NextResponse } from "next/server";
import { backendAdminFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import {
  ADMIN_AUTH_COOKIE,
  ADMIN_REFRESH_COOKIE,
  clearAdminAuthCookies,
  setAdminAuthCookies,
} from "@/lib/admin-auth-cookies";
import { readCookie } from "@/lib/auth-cookies";
import { refreshOnce, type RefreshedTokens } from "@/lib/refresh-lock";

function unavailable(tokens?: RefreshedTokens | null): NextResponse {
  const response = NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 },
  );
  if (tokens) setAdminAuthCookies(response, tokens);
  return response;
}

async function callAdminRefresh(refreshToken: string): Promise<RefreshedTokens | null> {
  const backendRes = await backendAdminFetch("/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  if (backendRes.status >= 500) throw new Error("admin refresh upstream unavailable");
  if (!backendRes.ok) return null;
  return extractTokenPair(await readBackendJson(backendRes));
}

async function refreshAdminAccess(request: Request): Promise<RefreshedTokens | null> {
  const refresh = readCookie(request, ADMIN_REFRESH_COOKIE);
  if (!refresh) return null;
  return refreshOnce(refresh, callAdminRefresh);
}

async function forwardResponse(
  backendRes: Response,
  rotated?: RefreshedTokens | null,
): Promise<NextResponse> {
  const contentType = backendRes.headers.get("content-type") ?? "";
  let response: NextResponse;

  if (backendRes.status === 204 || backendRes.status === 205) {
    response = new NextResponse(null, { status: backendRes.status });
  } else if (
    contentType.includes("text/csv") ||
    contentType.includes("application/octet-stream")
  ) {
    response = new NextResponse(await backendRes.arrayBuffer(), { status: backendRes.status });
  } else {
    response = NextResponse.json(await readBackendJson(backendRes), { status: backendRes.status });
  }

  if (contentType) response.headers.set("Content-Type", contentType);
  const disposition = backendRes.headers.get("content-disposition");
  if (disposition) response.headers.set("Content-Disposition", disposition);
  if (rotated) setAdminAuthCookies(response, rotated);
  return response;
}

/**
 * Proxies a request to `/admin/v1` using admin httpOnly cookies,
 * refreshing access once on 401. Rotated access and refresh cookies are
 * preserved for JSON, empty, CSV and binary responses alike.
 */
export async function adminProxy(
  request: Request,
  path: string,
  init?: RequestInit,
): Promise<NextResponse> {
  let access = readCookie(request, ADMIN_AUTH_COOKIE) ?? "";
  const method = init?.method ?? request.method;
  const headers: HeadersInit = {
    Authorization: access ? `Bearer ${access}` : "",
    "Content-Type": "application/json",
    ...(init?.headers ?? {}),
  };
  let rotated: RefreshedTokens | null = null;

  try {
    let backendRes = await backendAdminFetch(path, { ...init, method, headers });
    if (backendRes.status === 401) {
      rotated = await refreshAdminAccess(request);
      if (!rotated) {
        const response = NextResponse.json(
          { error: { code: "unauthorized", message: "admin session expired" } },
          { status: 401 },
        );
        clearAdminAuthCookies(response);
        return response;
      }
      access = rotated.accessToken;
      backendRes = await backendAdminFetch(path, {
        ...init,
        method,
        headers: { ...headers, Authorization: `Bearer ${access}` },
      });
    }
    return await forwardResponse(backendRes, rotated);
  } catch {
    return unavailable(rotated);
  }
}

/**
 * Proxies an SSE stream from `/admin/v1` using admin httpOnly cookies.
 * NextResponse is used so a refresh performed before opening the stream can
 * rotate both cookies without buffering the event stream.
 */
export async function adminStreamProxy(request: Request, path: string): Promise<NextResponse> {
  let access = readCookie(request, ADMIN_AUTH_COOKIE) ?? "";
  const headers: HeadersInit = {
    Authorization: access ? `Bearer ${access}` : "",
    Accept: "text/event-stream",
  };
  let rotated: RefreshedTokens | null = null;

  try {
    let backendRes = await backendAdminFetch(path, { method: "GET", headers });
    if (backendRes.status === 401) {
      rotated = await refreshAdminAccess(request);
      if (!rotated) {
        const response = NextResponse.json(
          { error: { code: "unauthorized", message: "admin session expired" } },
          { status: 401 },
        );
        clearAdminAuthCookies(response);
        return response;
      }
      access = rotated.accessToken;
      backendRes = await backendAdminFetch(path, {
        method: "GET",
        headers: { Authorization: `Bearer ${access}`, Accept: "text/event-stream" },
      });
    }
    if (!backendRes.ok || !backendRes.body) {
      return await forwardResponse(backendRes, rotated);
    }
    const response = new NextResponse(backendRes.body, {
      status: backendRes.status,
      headers: {
        "Content-Type": backendRes.headers.get("content-type") ?? "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      },
    });
    if (rotated) setAdminAuthCookies(response, rotated);
    return response;
  } catch {
    return unavailable(rotated);
  }
}
