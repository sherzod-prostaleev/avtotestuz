import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { setAuthCookies, clearAuthCookies, readCookie, AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { refreshOnce } from "@/lib/refresh-lock";
import { callBackendRefresh } from "@/lib/backend-refresh";

async function forward(
  request: Request,
  path: string[],
  accessToken: string,
  body: string | undefined
): Promise<Response> {
  const url = new URL(request.url);
  const targetPath = `/${path.join("/")}${url.search}`;
  const init: RequestInit = {
    method: request.method,
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": request.headers.get("content-type") ?? "application/json",
    },
  };
  if (body !== undefined) {
    init.body = body;
  }
  return backendFetch(targetPath, init);
}

async function handle(request: Request, context: { params: { path: string[] } }) {
  const { path } = context.params;
  const accessToken = readCookie(request, AUTH_COOKIE);

  if (!accessToken) {
    return NextResponse.json({ error: { code: "unauthorized", message: "no access token" } }, { status: 401 });
  }

  // Read the body ONCE — a Request's body stream can only be consumed a
  // single time, and this same body must be reused verbatim on the retry
  // after a refresh (re-reading request.text() a second time would throw).
  const body = request.method !== "GET" && request.method !== "HEAD" ? await request.text() : undefined;

  let backendRes = await forward(request, path, accessToken, body);

  if (backendRes.status === 401) {
    const refreshToken = readCookie(request, REFRESH_COOKIE);
    const tokens = refreshToken ? await refreshOnce(refreshToken, callBackendRefresh) : null;

    if (!tokens) {
      const response = NextResponse.json(
        { error: { code: "unauthorized", message: "session expired" } },
        { status: 401 }
      );
      clearAuthCookies(response);
      return response;
    }

    backendRes = await forward(request, path, tokens.accessToken, body);
    const data = await backendRes.json();
    const response = NextResponse.json(data, { status: backendRes.status });
    setAuthCookies(response, tokens);
    return response;
  }

  const data = await backendRes.json();
  return NextResponse.json(data, { status: backendRes.status });
}

export const GET = handle;
export const POST = handle;
export const PATCH = handle;
export const DELETE = handle;
