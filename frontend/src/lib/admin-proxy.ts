import { NextResponse } from "next/server";
import { backendAdminFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import {
  ADMIN_AUTH_COOKIE,
  ADMIN_REFRESH_COOKIE,
  setAdminAuthCookies,
  clearAdminAuthCookies,
} from "@/lib/admin-auth-cookies";
import { readCookie } from "@/lib/auth-cookies";

type AdminRefreshPayload = {
  data?: {
    access_token?: string;
  };
};

function unavailable() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 },
  );
}

async function refreshAdminAccess(request: Request): Promise<string | null> {
  const refresh = readCookie(request, ADMIN_REFRESH_COOKIE);
  if (!refresh) return null;
  const backendRes = await backendAdminFetch("/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refresh }),
  });
  if (!backendRes.ok) return null;
  const data = await readBackendJson<AdminRefreshPayload>(backendRes);
  const access = data.data?.access_token;
  return typeof access === "string" ? access : null;
}

/**
 * Proxies a request to `/admin/v1` using admin httpOnly cookies,
 * refreshing access once on 401.
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
  try {
    let backendRes = await backendAdminFetch(path, { ...init, method, headers });
    if (backendRes.status === 401) {
      const rotated = await refreshAdminAccess(request);
      if (!rotated) {
        const res = NextResponse.json(
          { error: { code: "unauthorized", message: "admin session expired" } },
          { status: 401 },
        );
        clearAdminAuthCookies(res);
        return res;
      }
      access = rotated;
      backendRes = await backendAdminFetch(path, {
        ...init,
        method,
        headers: { ...headers, Authorization: `Bearer ${access}` },
      });
      const data = await readBackendJson(backendRes);
      const res = NextResponse.json(data, { status: backendRes.status });
      const refresh = readCookie(request, ADMIN_REFRESH_COOKIE);
      if (backendRes.ok && refresh) {
        setAdminAuthCookies(res, { accessToken: access, refreshToken: refresh });
      }
      return res;
    }
    const contentType = backendRes.headers.get("content-type") ?? "";
    if (
      contentType.includes("text/csv") ||
      contentType.includes("application/octet-stream")
    ) {
      const buf = await backendRes.arrayBuffer();
      const res = new NextResponse(buf, { status: backendRes.status });
      res.headers.set("Content-Type", contentType);
      const cd = backendRes.headers.get("content-disposition");
      if (cd) res.headers.set("Content-Disposition", cd);
      return res;
    }
    const data = await readBackendJson(backendRes);
    return NextResponse.json(data, { status: backendRes.status });
  } catch {
    return unavailable();
  }
}

/**
 * Proxies an SSE stream from `/admin/v1` using admin httpOnly cookies.
 * Pipes the backend body through without buffering JSON.
 */
export async function adminStreamProxy(
  request: Request,
  path: string,
): Promise<Response> {
  let access = readCookie(request, ADMIN_AUTH_COOKIE) ?? "";
  const headers: HeadersInit = {
    Authorization: access ? `Bearer ${access}` : "",
    Accept: "text/event-stream",
  };
  try {
    let backendRes = await backendAdminFetch(path, { method: "GET", headers });
    if (backendRes.status === 401) {
      const rotated = await refreshAdminAccess(request);
      if (!rotated) {
        const res = NextResponse.json(
          { error: { code: "unauthorized", message: "admin session expired" } },
          { status: 401 },
        );
        clearAdminAuthCookies(res);
        return res;
      }
      access = rotated;
      backendRes = await backendAdminFetch(path, {
        method: "GET",
        headers: { Authorization: `Bearer ${access}`, Accept: "text/event-stream" },
      });
    }
    if (!backendRes.ok || !backendRes.body) {
      const data = await readBackendJson(backendRes);
      return NextResponse.json(data, { status: backendRes.status });
    }
    const res = new Response(backendRes.body, {
      status: backendRes.status,
      headers: {
        "Content-Type": backendRes.headers.get("content-type") ?? "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      },
    });
    return res;
  } catch {
    return unavailable();
  }
}
