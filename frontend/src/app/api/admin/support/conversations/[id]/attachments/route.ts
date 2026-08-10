import { NextResponse } from "next/server";
import { backendAdminFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import {
  ADMIN_AUTH_COOKIE,
  ADMIN_REFRESH_COOKIE,
  clearAdminAuthCookies,
  setAdminAuthCookies,
} from "@/lib/admin-auth-cookies";
import { readCookie } from "@/lib/auth-cookies";
import { refreshOnce, type RefreshedTokens } from "@/lib/refresh-lock";
import { extractTokenPair } from "@/lib/backend-response";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

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

export async function POST(request: Request, context: Ctx) {
  const { id } = await context.params;
  let access = readCookie(request, ADMIN_AUTH_COOKIE) ?? "";
  const refresh = readCookie(request, ADMIN_REFRESH_COOKIE);
  const buf = await request.arrayBuffer();
  const contentType = request.headers.get("content-type") ?? "multipart/form-data";

  async function forward(token: string) {
    return backendAdminFetch(`/support/conversations/${id}/attachments`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": contentType,
      },
      body: buf,
    });
  }

  try {
    let backendRes = access ? await forward(access) : null;
    let rotated: RefreshedTokens | null = null;
    if (!backendRes || backendRes.status === 401) {
      if (!refresh) {
        const response = NextResponse.json(
          { error: { code: "unauthorized", message: "admin session expired" } },
          { status: 401 },
        );
        clearAdminAuthCookies(response);
        return response;
      }
      rotated = await refreshOnce(refresh, callAdminRefresh);
      if (!rotated) {
        const response = NextResponse.json(
          { error: { code: "unauthorized", message: "admin session expired" } },
          { status: 401 },
        );
        clearAdminAuthCookies(response);
        return response;
      }
      access = rotated.accessToken;
      backendRes = await forward(access);
    }
    const response = NextResponse.json(await readBackendJson(backendRes), {
      status: backendRes.status,
    });
    response.headers.set("Cache-Control", "no-store");
    if (rotated) setAdminAuthCookies(response, rotated);
    return response;
  } catch {
    return NextResponse.json(
      { error: { code: "network_error", message: "service temporarily unavailable" } },
      { status: 502 },
    );
  }
}
