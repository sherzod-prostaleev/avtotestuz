import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import { cookieModeFor, modeForBody, setAuthCookies } from "@/lib/auth-cookies";
import { buildClientIPAssertionHeaders } from "@/lib/client-ip-assertion";
import { rejectCrossSite } from "@/lib/same-origin";

export const runtime = "nodejs";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
}

export async function POST(request: Request) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;

  const body = await request.text();
  // tg_init_data means the Mini App is signing in: the backend links the
  // Telegram account and the cookies must work inside Telegram's iframe. A jar
  // that is already Telegram-mode stays so, or its partitioned cookies would
  // linger beside new lax ones. Without either, the website path is untouched.
  const linkRequested = modeForBody(body) === "telegram";
  const mode = linkRequested ? "telegram" : cookieModeFor(request);
  let backendRes: Response;
  let data: unknown;

  try {
    backendRes = await backendFetch("/auth/login", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...buildClientIPAssertionHeaders(request, "/auth/login"),
      },
      body,
    });
    data = await readBackendJson(backendRes);
  } catch {
    return unavailableResponse();
  }

  if (!backendRes.ok) {
    return NextResponse.json(data, { status: backendRes.status });
  }

  let tokens: { accessToken: string; refreshToken: string };
  try {
    tokens = extractTokenPair(data);
  } catch {
    return unavailableResponse();
  }

  const payload = (data as { data?: { must_change_password?: unknown; telegram_linked?: unknown } }).data;
  const mustChangePassword = payload?.must_change_password === true;

  // The backend omits telegram_linked unless it linked; the key is only added
  // for Mini App sign-ins so the website response stays byte-for-byte as it was.
  const response = NextResponse.json(
    {
      data: linkRequested
        ? { ok: true, must_change_password: mustChangePassword, telegram_linked: payload?.telegram_linked === true }
        : { ok: true, must_change_password: mustChangePassword },
    },
    { status: 200 },
  );
  setAuthCookies(response, tokens, mode);
  return response;
}
