import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import { cookieModeFor, setAuthCookies } from "@/lib/auth-cookies";
import { buildClientIPAssertionHeaders } from "@/lib/client-ip-assertion";
import { rejectCrossSite } from "@/lib/same-origin";
import { isMaskedPhone } from "@/lib/signed-in-notice";
import {
  clearTelegramLoginCookie,
  isLoginToken,
  readTelegramLoginSecret,
} from "@/lib/telegram-login-cookie";

export const runtime = "nodejs";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
}

/**
 * Trades an approved Telegram login for the ordinary website session: the
 * same lax at/rt cookies a password login sets (a jar that is already
 * Telegram-mode stays so, exactly as /api/auth/login decides). One-time: the
 * backend consumes the request and the browser secret cookie is dropped.
 */
export async function POST(request: Request) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;

  let token: unknown;
  try {
    token = ((await request.json()) as { token?: unknown }).token;
  } catch {
    token = undefined;
  }
  const secret = readTelegramLoginSecret(request);
  if (!isLoginToken(token) || !secret) {
    return NextResponse.json(
      { error: { code: "invalid_login_request", message: "sign-in request is invalid or expired" } },
      { status: 400 }
    );
  }

  let backendRes: Response;
  let data: unknown;
  try {
    backendRes = await backendFetch("/auth/telegram-login/complete", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...buildClientIPAssertionHeaders(request, "/auth/telegram-login/complete"),
      },
      body: JSON.stringify({ token, browser_secret: secret }),
    });
    data = await readBackendJson(backendRes);
  } catch {
    return unavailableResponse();
  }
  if (!backendRes.ok) {
    const response = NextResponse.json(data, { status: backendRes.status });
    // Not yet approved keeps the secret for the next attempt; anything else
    // is final for this request.
    if (backendRes.status !== 409 && backendRes.status !== 429) clearTelegramLoginCookie(response);
    return response;
  }

  let tokens: { accessToken: string; refreshToken: string };
  try {
    tokens = extractTokenPair(data);
  } catch {
    return unavailableResponse();
  }
  const payload = (data as { data?: { must_change_password?: unknown; created?: unknown; phone_masked?: unknown } }).data;
  const response = NextResponse.json(
    {
      data: {
        ok: true,
        must_change_password: payload?.must_change_password === true,
        created: payload?.created === true,
        // Which account this browser now holds, masked by the backend; the
        // page shows it once. Anything not in the masked shape is dropped.
        phone_masked: isMaskedPhone(payload?.phone_masked) ? payload.phone_masked : null,
      },
    },
    { status: 200 }
  );
  clearTelegramLoginCookie(response);
  // Last cookie mutation on this response (see setAuthCookies).
  setAuthCookies(response, tokens, cookieModeFor(request));
  return response;
}
