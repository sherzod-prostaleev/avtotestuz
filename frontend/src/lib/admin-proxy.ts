import { NextResponse } from "next/server";
import { backendAdminFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import {
  ADMIN_AUTH_COOKIE,
  ADMIN_REFRESH_COOKIE,
  ADMIN_TOTP_ENROLL_COOKIE,
  clearAdminAuthCookies,
  clearAdminEnrollCookie,
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
    contentType.includes("application/octet-stream") ||
    contentType.startsWith("image/") ||
    contentType.startsWith("application/pdf") ||
    contentType.startsWith("application/zip") ||
    contentType.startsWith("application/msword") ||
    contentType.includes("officedocument") ||
    contentType.startsWith("text/plain")
  ) {
    response = new NextResponse(await backendRes.arrayBuffer(), { status: backendRes.status });
  } else {
    response = NextResponse.json(await readBackendJson(backendRes), { status: backendRes.status });
  }

  if (contentType) response.headers.set("Content-Type", contentType);
  const disposition = backendRes.headers.get("content-disposition");
  if (disposition) response.headers.set("Content-Disposition", disposition);
  // Nothing behind an admin session may be cached by a browser, a proxy or a
  // CDN. This used to forward only the two headers above, which dropped the
  // backend's own Cache-Control -- and the installer download URL ends in
  // .exe, an extension edge caches treat as static, so a school kept being
  // handed the same build after every fix while the console reported an old
  // version. That response body is also a bearer credential: the school's
  // installer key is appended to the binary.
  response.headers.set("Cache-Control", "no-store, no-cache, must-revalidate, private, max-age=0");
  response.headers.set("Pragma", "no-cache");
  if (rotated) setAdminAuthCookies(response, rotated);
  return response;
}

/**
 * The only two upstream paths the scoped enrollment token authorizes — the
 * same pair the backend puts behind `TOTPEnrollAuth`. Checking it here, rather
 * than trusting the caller's flag alone, means no future route can opt itself
 * into a credential that bypasses the admin session boundary.
 */
const ENROLLMENT_PATHS = new Set(["/security/totp/enroll", "/security/totp/confirm"]);

export type AdminProxyOptions = {
  /**
   * Lets the request fall back to the scoped TOTP-enrollment cookie when the
   * caller has no admin session at all. Only honoured for ENROLLMENT_PATHS.
   */
  allowEnrollmentToken?: boolean;
};

/**
 * Picks the enrollment token, but only when there is nothing else to use.
 *
 * The `art` check is the load-bearing one: `aat` expires after 15 minutes
 * while `art` lives for 30 days, so "no `aat`" on its own does not mean "no
 * session" — it usually means "refresh me". Preferring the enrollment token
 * there would push a perfectly healthy session down the branch below, whose
 * 401 handling would then be wrong for it.
 */
function enrollmentToken(request: Request, path: string, options?: AdminProxyOptions): string {
  if (!options?.allowEnrollmentToken || !ENROLLMENT_PATHS.has(path)) return "";
  if (readCookie(request, ADMIN_AUTH_COOKIE) || readCookie(request, ADMIN_REFRESH_COOKIE)) return "";
  return readCookie(request, ADMIN_TOTP_ENROLL_COOKIE) ?? "";
}

/**
 * Proxies a request to `/admin/v1` using admin httpOnly cookies,
 * refreshing access once on 401. Rotated access and refresh cookies are
 * preserved for JSON, empty, CSV and binary responses alike.
 *
 * With `allowEnrollmentToken`, an admin who has no session at all — which is
 * precisely the state ADMIN_TOTP_ENFORCE leaves an un-enrolled admin in — can
 * still reach the two TOTP enrollment endpoints using the scoped token login
 * handed them. That branch never refreshes and never touches `aat`/`art`, so a
 * failed enrollment can neither mint nor destroy an admin session.
 */
export async function adminProxy(
  request: Request,
  path: string,
  init?: RequestInit,
  options?: AdminProxyOptions,
): Promise<NextResponse> {
  let access = readCookie(request, ADMIN_AUTH_COOKIE) ?? "";
  const method = init?.method ?? request.method;
  const enrollment = enrollmentToken(request, path, options);
  const headers: HeadersInit = {
    Authorization: enrollment ? `Bearer ${enrollment}` : access ? `Bearer ${access}` : "",
    "Content-Type": "application/json",
    ...(init?.headers ?? {}),
  };
  let rotated: RefreshedTokens | null = null;

  if (enrollment) {
    try {
      const backendRes = await backendAdminFetch(path, { ...init, method, headers });
      const response = await forwardResponse(backendRes);
      // A rejected enrollment token is spent: drop it so the login page
      // restarts the flow instead of retrying a dead credential forever.
      if (backendRes.status === 401) clearAdminEnrollCookie(response);
      return response;
    } catch {
      return unavailable();
    }
  }

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
