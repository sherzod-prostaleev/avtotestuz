import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import { buildClientIPAssertionHeaders } from "@/lib/client-ip-assertion";
import { rejectCrossSite } from "@/lib/same-origin";
import { isLoginToken, setTelegramLoginCookie } from "@/lib/telegram-login-cookie";

export const runtime = "nodejs";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
}

/**
 * Starts a website «Telegram orqali kirish». The browser secret goes into
 * the HttpOnly cookie and is stripped from the body: page script only gets
 * the deep link (and its token, which the link carries anyway).
 */
export async function POST(request: Request) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;

  let backendRes: Response;
  let data: unknown;
  try {
    backendRes = await backendFetch("/auth/telegram-login/start", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...buildClientIPAssertionHeaders(request, "/auth/telegram-login/start"),
      },
      // The backend sees Node's User-Agent on this hop; the browser's is the
      // one the bot prompt describes ("Chrome · Android").
      body: JSON.stringify({ user_agent: (request.headers.get("user-agent") ?? "").slice(0, 512) }),
    });
    data = await readBackendJson(backendRes);
  } catch {
    return unavailableResponse();
  }
  if (!backendRes.ok) {
    return NextResponse.json(data, { status: backendRes.status });
  }
  const payload = (data as { data?: Record<string, unknown> }).data ?? {};
  const { bot_url: botURL, token, browser_secret: secret, expires_in_sec: expires } = payload;
  if (typeof botURL !== "string" || !isLoginToken(token) || typeof secret !== "string" || !secret) {
    return unavailableResponse();
  }
  const response = NextResponse.json(
    { data: { bot_url: botURL, token, expires_in_sec: typeof expires === "number" ? expires : 300 } },
    { status: 200, headers: { "Cache-Control": "no-store" } }
  );
  setTelegramLoginCookie(response, secret);
  return response;
}
